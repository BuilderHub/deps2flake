package python

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUVLockRunnerCopiesToOutputDir(t *testing.T) {
	source := t.TempDir()
	out := filepath.Join(t.TempDir(), "dist")
	writePyproject(t, source)
	if err := os.WriteFile(filepath.Join(source, "uv.lock"), []byte("version = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := (uvLockRunner{}).ensure(context.Background(), source, out, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pyproject.toml", "uv.lock"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestCopyProjectFileRejectsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "uv.lock")
	if err := os.WriteFile(dest, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	err := copyProjectFile(filepath.Join(dir, "src"), dest, false)
	if err == nil {
		t.Fatal("expected exclusive create error")
	}
}

func TestParseProjectDefaultVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte("[project]\nname = \"demo\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ProjectVersion != "0.1.0" {
		t.Fatalf("version = %q", meta.ProjectVersion)
	}
}
