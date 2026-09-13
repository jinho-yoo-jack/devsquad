package dev.devsquad.approval.infra;

import dev.devsquad.approval.domain.Approval;
import dev.devsquad.approval.domain.ApprovalStatus;
import java.util.List;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;

public interface ApprovalRepository extends JpaRepository<Approval, UUID> {
    List<Approval> findByTaskIdAndStatus(UUID taskId, ApprovalStatus status);
    List<Approval> findByStatusOrderByRequestedAtAsc(ApprovalStatus status);
    List<Approval> findByResumePendingTrue(); // RunRecoveryJob 재전송 대상
}
