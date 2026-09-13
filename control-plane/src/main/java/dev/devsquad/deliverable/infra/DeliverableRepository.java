package dev.devsquad.deliverable.infra;

import dev.devsquad.deliverable.domain.Deliverable;
import java.util.List;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;

public interface DeliverableRepository extends JpaRepository<Deliverable, UUID> {
    List<Deliverable> findByTaskIdOrderByCreatedAtAsc(UUID taskId);
}
