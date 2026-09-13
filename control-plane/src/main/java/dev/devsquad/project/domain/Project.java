package dev.devsquad.project.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "project")
public class Project {
    @Id private UUID id;
    @Column(nullable = false) private String name;
    @Column(name = "github_owner", nullable = false) private String githubOwner;
    @Column(name = "github_repo", nullable = false) private String githubRepo;
    @Column(name = "default_branch", nullable = false) private String defaultBranch = "main";
    @Column(name = "installation_id") private Long installationId;
    @Column(name = "context_path", nullable = false) private String contextPath = ".devsquad";
    @Column(name = "local_path") private String localPath;
    @Column(name = "token_budget", nullable = false) private long tokenBudget = 2_000_000L;
    @Column(name = "created_at", nullable = false, updatable = false) private Instant createdAt = Instant.now();

    protected Project() {}

    public Project(UUID id, String name, String githubOwner, String githubRepo) {
        this.id = id; this.name = name; this.githubOwner = githubOwner; this.githubRepo = githubRepo;
    }

    public UUID getId() { return id; }
    public String getName() { return name; }
    public String getGithubOwner() { return githubOwner; }
    public String getGithubRepo() { return githubRepo; }
    public String getDefaultBranch() { return defaultBranch; }
    public Long getInstallationId() { return installationId; }
    public String getContextPath() { return contextPath; }
    public String getLocalPath() { return localPath; }
    public void setLocalPath(String v) { this.localPath = v; }
    public long getTokenBudget() { return tokenBudget; }
    public Instant getCreatedAt() { return createdAt; }
    public void setDefaultBranch(String v) { this.defaultBranch = v; }
    public void setInstallationId(Long v) { this.installationId = v; }
    public void setTokenBudget(long v) { this.tokenBudget = v; }
}
