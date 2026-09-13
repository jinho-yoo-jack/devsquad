package dev.devsquad.task.infra;

import dev.devsquad.task.domain.Stage;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;

public interface StageRepository extends JpaRepository<Stage, UUID> {
    List<Stage> findByTaskIdOrderByStartedAtAsc(UUID taskId);
    Optional<Stage> findByTaskIdAndStageKey(UUID taskId, String stageKey);
}
