package pathguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeRelative(t *testing.T) {
	for name, want := range map[string]bool{"docs/a.md": true, "a": true, "docs/./a.md": true, "": false, "~/x": false, "/etc/passwd": false, "../x": false, "docs/../../x": false, `docs\a.md`: false} {
		if got := SafeRelative(name); got != want {
			t.Errorf("SafeRelative(%q)=%v want %v", name, got, want)
		}
	}
}

// 19-Go-통합-서비스-설계 §9 and 17-Agent-정의-가이드 §1.3: every profile is denied these paths.
func TestProtectedSecretsAndDirectories(t *testing.T) {
	for _, name := range []string{".env", ".env.local", "nested/.env.prod", "tls/server.pem", "app.key", "cert.p12", "keystore.jks", "id_rsa", "home/id_rsa.pub", "id_ed25519", "secrets/token", "a/secrets/b", ".git/config", ".ssh/known_hosts", ".aws/credentials", "web/node_modules/x/index.js", ".venv/bin/python", ".devsquad-runtime/definition.json"} {
		if !Protected(name) {
			t.Errorf("not protected: %s", name)
		}
	}
	for _, name := range []string{"docs/env.md", "environment.md", "src/key.go", "docs/secrets.md", "keys/readme.md", "docs/git.md"} {
		if Protected(name) {
			t.Errorf("protected by mistake: %s", name)
		}
	}
}

// macOS and Windows resolve names case-insensitively, so ".ENV" opens ".env".
func TestProtectedIgnoresCase(t *testing.T) {
	for _, name := range []string{".ENV", "nested/.Env.prod", "TLS/SERVER.PEM", "Secrets/token", ".GIT/config", ".Devsquad-Runtime/definition.json", "ID_RSA"} {
		if !Protected(name) {
			t.Errorf("case variant not protected: %s", name)
		}
	}
}

func TestResolveCanonicalizesAliases(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("docs/real.md", "ok")
	write(".env", "secret")
	for link, target := range map[string]string{"docs/alias.md": "real.md", "docs/env.md": "../.env", "out": outside} {
		if e := os.Symlink(target, filepath.Join(dir, link)); e != nil {
			t.Fatal(e)
		}
	}
	for name, want := range map[string]string{"docs/alias.md": "docs/real.md", "docs/new/file.md": "docs/new/file.md", "docs/real.md": "docs/real.md"} {
		got, e := Resolve(dir, name)
		if e != nil || got != want {
			t.Errorf("Resolve(%q)=%q,%v want %q", name, got, e, want)
		}
	}
	for _, name := range []string{"docs/env.md", "out/file.md", "../x", ".env"} {
		if got, e := Resolve(dir, name); e == nil {
			t.Errorf("Resolve(%q) allowed as %q", name, got)
		}
	}
}

func TestNoSymlinks(t *testing.T) {
	dir := t.TempDir()
	if e := os.MkdirAll(filepath.Join(dir, "docs"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("docs", filepath.Join(dir, "link")); e != nil {
		t.Fatal(e)
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	if e = NoSymlinks(root, "docs/missing.md"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"link/a.md", "link", "../docs"} {
		if NoSymlinks(root, name) == nil {
			t.Errorf("accepted %s", name)
		}
	}
}
