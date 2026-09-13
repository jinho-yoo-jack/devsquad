package dev.devsquad.runtime;

import dev.devsquad.approval.app.ApprovalService;
import dev.devsquad.approval.domain.Approval;
import dev.devsquad.approval.domain.Decision;
import dev.devsquad.project.infra.ProjectRepository;
import dev.devsquad.task.app.TaskService;
import dev.devsquad.task.domain.Task;
import dev.devsquad.task.domain.TaskStatus;
import dev.devsquad.task.infra.TaskRepository;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 11-아키텍처 §6.2 프로세스 재시작 복구 + 14-Backend A.4 RunRecoveryJob.
 * 기동 시와 5분마다:
 *  1) queued Task → run 재시도 (Agent Runtime 이 죽어 있었던 경우)
 *  2) running/waiting_approval Task 의 런타임 상태 대조 → stranded 면 /continue
 *  3) resume_pending Approval → resume 재전송
 */
@Component
public class RunRecoveryJob {
    private static final Logger log = LoggerFactory.getLogger(RunRecoveryJob.class);

    private final TaskRepository tasks;
    private final ProjectRepository projects;
    private final TaskService taskService;
    private final ApprovalService approvalService;
    private final AgentRuntimeClient runtime;

    public RunRecoveryJob(TaskRepository tasks, ProjectRepository projects, TaskService taskService,
                          ApprovalService approvalService, AgentRuntimeClient runtime) {
        this.tasks = tasks; this.projects = projects; this.taskService = taskService; this.approvalService = approvalService; this.runtime = runtime;
    }

    @EventListener(ApplicationReadyEvent.class)
    public void onStartup() { run(); }

    @Scheduled(fixedDelayString = "${devsquad.recovery.interval-ms:300000}", initialDelayString = "${devsquad.recovery.interval-ms:300000}")
    public void scheduled() { run(); }

    public void run() {
        if (!runtime.isHealthy()) { log.warn("recovery skipped: agent runtime unhealthy"); return; }

        for (Task t : tasks.findByStatusIn(List.of(TaskStatus.QUEUED))) {
            projects.findById(t.getProjectId()).ifPresent(p -> {
                try { taskService.startRun(t, p); log.info("recovered queued task {}", t.getId()); }
                catch (RuntimeException e) { log.warn("start retry failed for {}: {}", t.getId(), e.toString()); }
            });
        }

        for (Approval a : approvalService.resumePending()) {
            Decision d = ApprovalService.toDecision(
                a.getStatus().name().equals("EDITED") ? "edit" : a.getStatus().name().toLowerCase().replace("approved", "approve").replace("rejected", "reject"),
                a.getDecisionFeedback(), a.getEditedContent());
            approvalService.sendResume(a.getId(), a.getTaskId(), d);
        }

        for (Task t : tasks.findByStatusIn(List.of(TaskStatus.RUNNING, TaskStatus.WAITING_APPROVAL))) {
            try {
                var st = runtime.state(t.getId());
                if (st != null && st.isStranded()) {
                    log.info("task {} stranded at {} — continuing", t.getId(), st.next());
                    runtime.continueRun(t.getId());
                }
            } catch (RuntimeException e) {
                log.warn("state check failed for {}: {}", t.getId(), e.toString());
            }
        }
    }
}
