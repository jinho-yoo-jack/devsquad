package scm

import "context"

type PullRequest struct {
	Owner, Repo, Head, Base, Title, Body string
}
type Opened struct {
	URL     string
	Number  int
	Created bool
}

// Publisher is the hosting service behind clone, push and pull requests. Without
// one, only local_path projects run and the publisher stage skips pull requests.
type Publisher interface {
	// Open is idempotent: an open pull request for the head branch is updated and reused.
	Open(context.Context, PullRequest) (Opened, error)
	RemoteURL(owner, repo string) string
	// Web is the host for links to files on pushed branches.
	Web() string
	// GitHeader authenticates git over HTTPS without storing the token in .git/config.
	GitHeader() string
}
