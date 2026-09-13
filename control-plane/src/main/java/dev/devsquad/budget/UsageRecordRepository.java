package dev.devsquad.budget;

import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface UsageRecordRepository extends JpaRepository<UsageRecord, Long> {
    @Query("select coalesce(sum(u.inputTokens + u.outputTokens), 0) from UsageRecord u where u.taskId = :taskId")
    long totalTokens(UUID taskId);
}
