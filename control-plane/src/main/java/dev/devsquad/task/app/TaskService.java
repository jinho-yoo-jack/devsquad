package dev.devsquad.task.app;

import dev.devsquad.common.NotFoundException;
import dev.devsquad.common.RuntimeUnavailableException;
import dev.devsquad.project.domain.Project;
import dev.devsquad.project.infra.ProjectRepository;
import dev.devsquad.runtime.AgentRuntimeClient;
import dev.devsquad.runtime.AgentRuntimeClient.RunAccepted;
import dev.devsquad.task.domain.*;
import dev.devsquad.task.domain.TaskStateMachine.Event;
import dev.devsquad.task.infra.StageRepository;
import dev.devsquad.task.infra.TaskRepository;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Task 생명주기 — 14-Backend 설계 A.4 TaskService.
 *
 * create 는 두 트랜잭션으로 나눈다: (1) Task(queued) 저장·커밋 → (2) Agent Runtime run 호출 → (3) 응답의 stages 로
 * Stage 행 생성 + RUNNING 전이. (2) 가 실패하면 Task 는 queued 로 남고 RunRecoveryJob 이 재시도한다.
 * 한 트랜잭션에 묶으면 외부 HTTP 호출이 DB 락을 잡고 있게 되고, 커밋 전에 이벤트가 도착할 수 있다.
 */
@Service
public class TaskService {
    private static final Logger log = LoggerFactory.getLogger(TaskService.class);

    private final TaskRepository tasks;
    private final StageRepository stages;
    private final ProjectRepository projects;
    private final AgentRuntimeClient runtime;
    private final TransactionTemplate tx;
    private final ApplicationEventPublisher events;

    public TaskService(TaskRepository tasks, StageRepository stages, ProjectRepository projects,
                       AgentRuntimeClient runtime, TransactionTemplate tx, ApplicationEventPublisher events) {
        this.tasks = tasks; this.stages = stages; this.projects = projects; this.runtime = runtime; this.tx = tx; this.events = events;
    }

    /** 도메인 이벤트: 다른 모듈(discord, ws)이 구독한다. */
    public record TaskCreated(UUID taskId, UUID projectId, String command, String createdBy) {}
    public record TaskStatusChanged(UUID taskId, TaskStatus from, TaskStatus to, String reason) {}

    public Task create(UUID projectId, String command, String createdBy) {
        Project project = projects.findById(projectId).orElseThrow(() -> new NotFoundException("project", projectId));
        Task task = tx.execute(s -> tasks.save(new Task(UUID.randomUUID(), projectId, command.strip(), createdBy)));
        events.publishEvent(new TaskCreated(task.getId(), projectId, task.getCommand(), createdBy));
        startRun(task, project);
        return tasks.findById(task.getId()).orElseThrow();
    }

    /** queued Task 를 Agent Runtime 에 넘긴다. RunRecoveryJob 도 이 메서드를 재사용한다. */
    public void startRun(Task task, Project project) {
        var ref = new AgentRuntimeClient.ProjectRef(project.getGithubOwner(), project.getGithubRepo(), project.getDefaultBranch(),
            null, project.getContextPath(), project.getLocalPath());
        RunAccepted accepted;
        try {
            accepted = runtime.run(new AgentRuntimeClient.RunRequest(task.getId(), task.getCommand(), ref, Map.of()));
        } catch (RuntimeException e) {
            if (AgentRuntimeClient.isUnavailable(e)) {
                log.warn("agent runtime unavailable; task {} stays queued", task.getId());
                throw new RuntimeUnavailableException(e);
            }
            transition(task.getId(), Event.FAIL, "run rejected: " + e.getMessage());
            throw e;
        }
        tx.executeWithoutResult(s -> {
            Task t = tasks.findById(task.getId()).orElseThrow();
            if (accepted.stages() != null) {
                for (var st : accepted.stages()) {
                    if (stages.findByTaskIdAndStageKey(t.getId(), st.id()).isEmpty()) {
                        stages.save(new Stage(UUID.randomUUID(), t.getId(), st.id(), st.agent(),
                            st.dependsOn() == null ? List.of() : st.dependsOn()));
                    }
                }
            }
            TaskStatus from = t.getStatus();
            t.apply(Event.RUN);
            events.publishEvent(new TaskStatusChanged(t.getId(), from, t.getStatus(), "run accepted"));
        });
    }

    @Transactional
    public Task transition(UUID taskId, Event event, String reason) {
        Task t = tasks.findById(taskId).orElseThrow(() -> new NotFoundException("task", taskId));
        TaskStatus from = t.getStatus();
        t.apply(event);
        events.publishEvent(new TaskStatusChanged(taskId, from, t.getStatus(), reason));
        return t;
    }

    /** 이미 그 상태면 조용히 무시 (이벤트 재전송·순서 뒤바뀜에 관대). */
    @Transactional
    public void transitionIfPossible(UUID taskId, Event event, String reason) {
        Task t = tasks.findById(taskId).orElseThrow(() -> new NotFoundException("task", taskId));
        if (TaskStateMachine.canTransition(t.getStatus(), event)) {
            TaskStatus from = t.getStatus();
            t.apply(event);
            events.publishEvent(new TaskStatusChanged(taskId, from, t.getStatus(), reason));
        }
    }

    public Task pause(UUID taskId) { return transition(taskId, Event.PAUSE, "user"); }

    public Task resume(UUID taskId) {
        Task t = get(taskId);
        Event ev = t.getStatus() == TaskStatus.BLOCKED ? Event.MANUAL_RESUME : Event.RESUME;
        Task after = transition(taskId, ev, "user");
        runtime.continueRun(taskId);
        return after;
    }

    public Task cancel(UUID taskId) {
        Task after = transition(taskId, Event.CANCEL, "user");
        try { runtime.cancel(taskId); } catch (RuntimeException e) { log.warn("cancel on runtime failed for {}: {}", taskId, e.toString()); }
        return after;
    }

    @Transactional(readOnly = true)
    public Task get(UUID taskId) { return tasks.findById(taskId).orElseThrow(() -> new NotFoundException("task", taskId)); }

    @Transactional(readOnly = true)
    public List<Stage> stagesOf(UUID taskId) { return stages.findByTaskIdOrderByStartedAtAsc(taskId); }

    @Transactional(readOnly = true)
    public List<Task> list(UUID projectId, List<TaskStatus> statuses) {
        List<TaskStatus> st = (statuses == null || statuses.isEmpty()) ? List.of(TaskStatus.values()) : statuses;
        return projectId == null ? tasks.findByStatusIn(st) : tasks.findByProjectIdAndStatusIn(projectId, st);
    }
}
