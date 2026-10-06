package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"mvdan.cc/sh/v3/shell"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/pathguard"
)

type ToolSpec = llm.ToolSpec
type ToolArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Pattern string `json:"pattern"`
	Glob    string `json:"glob"`
}
type Sandbox struct {
	Root    *os.Root
	Agent   agentdef.AgentSpec
	Timeout time.Duration
	Allowed []string
}

func (s Sandbox) check(name string, write bool) error {
	if !pathguard.SafeRelative(name) || pathguard.Protected(name) {
		return fmt.Errorf("protected or invalid path: %s", name)
	}
	resolved, err := pathguard.Resolve(s.Root.Name(), name)
	if err != nil {
		return err
	}
	if write {
		if s.Agent.ToolProfile == "publisher" {
			return fmt.Errorf("publisher is read-only")
		}
		for _, wp := range s.Agent.WritePaths {
			if pathguard.GlobMatch(name, wp) && pathguard.GlobMatch(resolved, wp) {
				return nil
			}
		}
		return fmt.Errorf("path is outside write_paths=%q", s.Agent.WritePaths)
	}
	return nil
}
func (s Sandbox) Read(name string) (string, error) {
	if err := s.check(name, false); err != nil {
		return "", err
	}
	resolved, err := pathguard.Resolve(s.Root.Name(), name)
	if err != nil {
		return "", err
	}
	name = resolved
	f, err := s.Root.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, 200001))
	if len(data) > 200000 {
		return string(data[:200000]) + "\n[truncated]", err
	}
	return string(data), err
}
func (s Sandbox) Write(name, content string) error {
	if err := s.check(name, true); err != nil {
		return err
	}
	if len(content) > 2_000_000 {
		return fmt.Errorf("file exceeds 2 MB")
	}
	resolved, err := pathguard.Resolve(s.Root.Name(), name)
	if err != nil {
		return err
	}
	name = resolved
	if err := s.Root.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	return s.Root.WriteFile(name, []byte(content), 0600)
}
func (s Sandbox) Tools() []ToolSpec {
	tools := []ToolSpec{
		{Name: "read_file", Description: "Read a repository file using a relative path.", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)},
		{Name: "list_dir", Description: "List repository files in a directory.", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)},
		{Name: "search", Description: "Search file contents with a regular expression.", Schema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"glob":{"type":"string"}},"required":["pattern"]}`)},
	}
	if s.Agent.ToolProfile != "publisher" && len(s.Agent.WritePaths) > 0 {
		tools = append(tools, ToolSpec{Name: "write_file", Description: fmt.Sprintf("Write a file. Allowed paths: %v", s.Agent.WritePaths), Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)})
	}
	if s.Agent.ToolProfile == "code-writer" && s.Agent.TestCommand != "" {
		tools = append(tools, ToolSpec{Name: "run_tests", Description: "Run the repository's configured test command.", Schema: json.RawMessage(`{"type":"object","properties":{}}`)})
	}
	if s.Allowed == nil {
		return tools
	}
	out := []ToolSpec{}
	for _, t := range tools {
		for _, name := range s.Allowed {
			if t.Name == name {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

type limitedOutput struct {
	head, tail []rune
	pending    []byte
	total      int
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	b.pending = append(b.pending, p...)
	for len(b.pending) > 0 {
		if !utf8.FullRune(b.pending) {
			break
		}
		r, n := utf8.DecodeRune(b.pending)
		b.pending = b.pending[n:]
		b.total++
		if len(b.head) < 10000 {
			b.head = append(b.head, r)
		} else {
			b.tail = append(b.tail, r)
			if len(b.tail) > 10000 {
				b.tail = b.tail[len(b.tail)-10000:]
			}
		}
	}
	return len(p), nil
}
func (b *limitedOutput) String() string {
	sep := ""
	if b.total > 20000 {
		sep = "\n[truncated]\n"
	}
	return string(b.head) + sep + string(b.tail) + string(b.pending)
}
func (s Sandbox) Call(ctx context.Context, name string, raw json.RawMessage) (string, error) {
	allowed := false
	for _, t := range s.Tools() {
		if t.Name == name {
			allowed = true
		}
	}
	if !allowed {
		return "", fmt.Errorf("tool not allowed: %s", name)
	}
	var args ToolArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	switch name {
	case "read_file":
		return s.Read(args.Path)
	case "write_file":
		if err := s.Write(args.Path, args.Content); err != nil {
			return "", err
		}
		return "wrote " + args.Path, nil
	case "list_dir":
		if args.Path == "" {
			args.Path = "."
		}
		if err := s.check(args.Path, false); err != nil {
			return "", err
		}
		entries, err := fs.ReadDir(s.Root.FS(), args.Path)
		if err != nil {
			return "", err
		}
		out := []string{}
		for _, e := range entries {
			p := filepath.ToSlash(filepath.Join(args.Path, e.Name()))
			if s.check(p, false) != nil {
				continue
			}
			if e.IsDir() {
				p += "/"
			}
			out = append(out, p)
			if len(out) >= 500 {
				break
			}
		}
		return strings.Join(out, "\n"), nil
	case "search":
		rx, err := regexp.Compile(args.Pattern)
		if err != nil {
			return "", err
		}
		if args.Glob == "" {
			args.Glob = "**/*"
		}
		hits := []string{}
		err = fs.WalkDir(s.Root.FS(), ".", func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if s.check(p, false) != nil {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() || !pathguard.GlobMatch(p, args.Glob) {
				return nil
			}
			text, e := s.Read(p)
			if e != nil {
				return nil
			}
			for i, line := range strings.Split(text, "\n") {
				if rx.MatchString(line) {
					hits = append(hits, fmt.Sprintf("%s:%d: %s", p, i+1, truncate(line, 200)))
					if len(hits) >= 100 {
						return fs.SkipAll
					}
				}
			}
			return nil
		})
		return strings.Join(hits, "\n"), err
	case "run_tests":
		// The command is trusted repository configuration, never supplied by the model.
		runCtx, cancel := context.WithTimeout(ctx, timeout(s.Timeout))
		defer cancel()
		var cmd *exec.Cmd
		if strings.Contains(s.Agent.TestCommand, "&&") {
			cmd = exec.CommandContext(runCtx, "bash", "-lc", s.Agent.TestCommand)
		} else {
			args, e := shell.Fields(s.Agent.TestCommand, func(string) string { return "" })
			if e != nil || len(args) == 0 {
				return "", fmt.Errorf("invalid test_command: %v", e)
			}
			cmd = exec.CommandContext(runCtx, args[0], args[1:]...)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if err == syscall.ESRCH {
				return os.ErrProcessDone
			}
			return err
		}
		cmd.Dir = s.Root.Name()
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + s.Root.Name(), "TMPDIR=" + os.TempDir()}
		cmd.WaitDelay = 2 * time.Second
		out := &limitedOutput{}
		cmd.Stdout = out
		cmd.Stderr = out
		err := cmd.Run()
		code := 0
		if err != nil {
			code = -1
			var exit *exec.ExitError
			if errorsAsExit(err, &exit) {
				code = exit.ExitCode()
			}
		}
		return fmt.Sprintf("exit=%d\n%s", code, out.String()), nil
	}
	return "", fmt.Errorf("unknown tool")
}
func errorsAsExit(err error, target **exec.ExitError) bool {
	v, ok := err.(*exec.ExitError)
	if ok {
		*target = v
	}
	return ok
}
func truncate(text string, n int) string {
	r := []rune(text)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return text
}

func timeout(d time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return 10 * time.Minute
}
