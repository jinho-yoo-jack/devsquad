package dev.devsquad.project.app;

import dev.devsquad.common.NotFoundException;
import dev.devsquad.project.domain.Project;
import dev.devsquad.project.infra.ProjectRepository;
import java.util.List;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class ProjectService {
    private final ProjectRepository repo;

    public ProjectService(ProjectRepository repo) { this.repo = repo; }

    @Transactional
    public Project create(String name, String owner, String repoName, String defaultBranch, Long installationId, String localPath) {
        var p = new Project(UUID.randomUUID(), name, owner, repoName);
        if (defaultBranch != null && !defaultBranch.isBlank()) p.setDefaultBranch(defaultBranch);
        p.setInstallationId(installationId);
        p.setLocalPath(localPath);
        return repo.save(p);
    }

    @Transactional(readOnly = true)
    public Project get(UUID id) { return repo.findById(id).orElseThrow(() -> new NotFoundException("project", id)); }

    @Transactional(readOnly = true)
    public List<Project> list() { return repo.findAll(); }
}
