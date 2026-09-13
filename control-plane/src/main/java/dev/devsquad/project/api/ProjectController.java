package dev.devsquad.project.api;

import dev.devsquad.project.app.ProjectService;
import dev.devsquad.project.domain.Project;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import java.util.List;
import java.util.UUID;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1/projects")
public class ProjectController {
    private final ProjectService service;

    public ProjectController(ProjectService service) { this.service = service; }

    public record CreateRequest(@NotBlank String name, @NotBlank String githubOwner, @NotBlank String githubRepo,
                                String defaultBranch, Long installationId, String localPath) {}

    public record ProjectResponse(UUID id, String name, String githubOwner, String githubRepo, String defaultBranch,
                                  String contextPath, String localPath, long tokenBudget) {
        static ProjectResponse of(Project p) {
            return new ProjectResponse(p.getId(), p.getName(), p.getGithubOwner(), p.getGithubRepo(), p.getDefaultBranch(),
                p.getContextPath(), p.getLocalPath(), p.getTokenBudget());
        }
    }

    @GetMapping
    public List<ProjectResponse> list() { return service.list().stream().map(ProjectResponse::of).toList(); }

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public ProjectResponse create(@Valid @RequestBody CreateRequest req) {
        return ProjectResponse.of(service.create(req.name(), req.githubOwner(), req.githubRepo(), req.defaultBranch(),
            req.installationId(), req.localPath()));
    }

    @GetMapping("/{id}")
    public ProjectResponse get(@PathVariable UUID id) { return ProjectResponse.of(service.get(id)); }
}
