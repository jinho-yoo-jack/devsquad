package dev.devsquad.task.infra;

import dev.devsquad.task.domain.Task;
import dev.devsquad.task.domain.TaskStatus;
import java.util.Collection;
import java.util.List;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;

public interface TaskRepository extends JpaRepository<Task, UUID> {
    List<Task> findByProjectIdAndStatusIn(UUID projectId, Collection<TaskStatus> statuses);
    List<Task> findByStatusIn(Collection<TaskStatus> statuses); // RunRecoveryJob 용
}
