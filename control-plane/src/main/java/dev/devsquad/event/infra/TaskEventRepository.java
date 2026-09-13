package dev.devsquad.event.infra;

import dev.devsquad.event.domain.TaskEvent;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface TaskEventRepository extends JpaRepository<TaskEvent, Long> {
    boolean existsByEventId(UUID eventId);
    Optional<TaskEvent> findTopByTaskIdOrderBySeqDesc(UUID taskId);

    @Query("select e from TaskEvent e where e.taskId = :taskId and e.seq > :afterSeq order by e.seq asc")
    List<TaskEvent> findAfter(UUID taskId, long afterSeq, Pageable page);
}
