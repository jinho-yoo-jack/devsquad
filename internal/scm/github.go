package scm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub uses a personal access token for the REST API and git over HTTPS.
// WebURL, used for links in pull request bodies, defaults to GitURL.
type GitHub struct {
	Token  string
	APIURL string
	GitURL string
	WebURL string
	Client *http.Client
}
type pull struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
}

func (g *GitHub) RemoteURL(owner, repo string) string {
	return strings.TrimRight(g.GitURL, "/") + "/" + owner + "/" + repo + ".git"
}
func (g *GitHub) Web() string {
	if g.WebURL != "" {
		return strings.TrimRight(g.WebURL, "/")
	}
	return strings.TrimRight(g.GitURL, "/")
}
func (g *GitHub) GitHeader() string {
	return "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+g.Token))
}
func (g *GitHub) Open(ctx context.Context, pr PullRequest) (Opened, error) {
	found, e := g.find(ctx, pr)
	if e != nil {
		return Opened{}, e
	}
	if found == nil {
		var created pull
		status, e := g.call(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls", pr.Owner, pr.Repo), map[string]string{"title": pr.Title, "head": pr.Head, "base": pr.Base, "body": pr.Body}, &created)
		if e == nil {
			return Opened{URL: created.URL, Number: created.Number, Created: true}, nil
		}
		// 422 also means another publisher opened it first.
		if status != http.StatusUnprocessableEntity {
			return Opened{}, e
		}
		if found, _ = g.find(ctx, pr); found == nil {
			return Opened{}, e
		}
	}
	if _, e = g.call(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", pr.Owner, pr.Repo, found.Number), map[string]string{"title": pr.Title, "body": pr.Body}, nil); e != nil {
		return Opened{}, e
	}
	return Opened{URL: found.URL, Number: found.Number}, nil
}
func (g *GitHub) find(ctx context.Context, pr PullRequest) (*pull, error) {
	var open []pull
	query := url.Values{"head": {pr.Owner + ":" + pr.Head}, "state": {"open"}}
	if _, e := g.call(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls?%s", pr.Owner, pr.Repo, query.Encode()), nil, &open); e != nil || len(open) == 0 {
		return nil, e
	}
	return &open[0], nil
}
func (g *GitHub) call(ctx context.Context, method, path string, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, e := json.Marshal(body)
		if e != nil {
			return 0, e
		}
		reader = bytes.NewReader(raw)
	}
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(g.APIURL, "/")+path, reader)
	if e != nil {
		return 0, e
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	res, e := client.Do(req)
	if e != nil {
		return 0, e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if e != nil {
		return res.StatusCode, e
	}
	if res.StatusCode >= 300 {
		var failure struct {
			Message string `json:"message"`
			Errors  []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(raw, &failure)
		messages := []string{failure.Message}
		for _, d := range failure.Errors {
			messages = append(messages, d.Message)
		}
		return res.StatusCode, fmt.Errorf("github %s %s: %d %s", method, strings.SplitN(path, "?", 2)[0], res.StatusCode, strings.Join(messages, "; "))
	}
	if out == nil {
		return res.StatusCode, nil
	}
	if e = json.Unmarshal(raw, out); e != nil {
		return res.StatusCode, errors.Join(fmt.Errorf("github %s %s: invalid response", method, path), e)
	}
	return res.StatusCode, nil
}
