package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jinho-yoo-jack/devsquad/internal/pathguard"
)

func Prepare(ctx context.Context, root, task, source string) (string, error) {
	if !pathguard.SafeRelative(task) || strings.Contains(task, "/") || task == "." {
		return "", fmt.Errorf("invalid task_id")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", fmt.Errorf("project.local_path is required")
	}
	src, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", err
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(abs, task)
	if _, err = os.Stat(dest); err == nil {
		return "", fmt.Errorf("workspace already exists")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if dest == src || strings.HasPrefix(dest, src+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace must be outside source project")
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(abs, ".preparing-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	sourceRoot, err := os.OpenRoot(src)
	if err != nil {
		return "", err
	}
	defer sourceRoot.Close()
	err = filepath.WalkDir(src, func(name string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		rel, e := filepath.Rel(src, name)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if pathguard.Protected(rel) || d.Name() == ".next" || d.Name() == "build" || d.Name() == "target" || d.Name() == ".gradle" || d.Name() == "__pycache__" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		target := filepath.Join(temp, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if e = pathguard.NoSymlinks(sourceRoot, rel); e != nil {
			return e
		}
		in, e := sourceRoot.Open(rel)
		if e != nil {
			return e
		}
		defer in.Close()
		st, e := in.Stat()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, st.Mode().Perm()&0755)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, in)
		closeErr := out.Close()
		if e != nil {
			return e
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	if err = initGit(ctx, temp); err != nil {
		return "", err
	}
	if err = os.Rename(temp, dest); err != nil {
		return "", err
	}
	return dest, nil
}
