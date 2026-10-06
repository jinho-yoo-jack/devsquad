package pathguard

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func SafeRelative(name string) bool {
	if name == "" || strings.HasPrefix(name, "~") || strings.Contains(name, "\\") || filepath.IsAbs(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
func Protected(name string) bool {
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		switch part {
		case ".git", ".ssh", ".aws", "secrets", "node_modules", ".venv", ".devsquad-runtime":
			return true
		}
		for _, pattern := range []string{".env*", "*.pem", "*.key", "*.p12", "*.jks", "id_rsa*", "id_ed25519*"} {
			if ok, _ := path.Match(pattern, part); ok {
				return true
			}
		}
	}
	return false
}
func NoSymlinks(root *os.Root, name string) error {
	if !SafeRelative(name) {
		return fmt.Errorf("relative paths without traversal are required")
	}
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i := range parts {
		st, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", name)
		}
	}
	return nil
}

func GlobMatch(name, pattern string) bool { ok, _ := doublestar.PathMatch(pattern, name); return ok }

// Resolve includes existing symlink ancestors of a new file. Return a canonical
// relative path so later I/O does not follow the caller's alias to a secret file.
func Resolve(root, name string) (string, error) {
	if !SafeRelative(name) || Protected(name) {
		return "", fmt.Errorf("protected or invalid path: %s", name)
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return "", err
	}
	target := filepath.Join(base, name)
	suffix := []string{}
	for {
		resolved, e := filepath.EvalSymlinks(target)
		if e == nil {
			target = resolved
			break
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(target)
		if parent == target {
			return "", e
		}
		suffix = append(suffix, filepath.Base(target))
		target = parent
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		target = filepath.Join(target, suffix[i])
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || !SafeRelative(filepath.ToSlash(rel)) || Protected(filepath.ToSlash(rel)) {
		return "", fmt.Errorf("resolved path is outside allowed workspace: %s", name)
	}
	return filepath.ToSlash(rel), nil
}
