package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	want := []byte("standalone executable bytes")
	if err := os.WriteFile(src, want, 0755); err != nil {
		t.Fatal(err)
	}

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src still exists after move (err=%v)", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("dst content = %q, want %q", got, want)
	}
}

// TestMoveFileOverwritesExisting matters specifically because the Windows
// rename fast-path fails when dst already exists (unlike the copy
// fallback, which truncates it) — callers (e.g. cmd/advpp-ide's
// buildStandalone) pre-remove the destination for exactly this reason.
func TestMoveFileOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	if err := os.WriteFile(src, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old-longer-content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("dst content = %q, want %q", got, "new")
	}
}

func TestWalkUpForModule(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	goMod := "module " + standaloneModule + "\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	if got := walkUpForModule(nested); got != root {
		t.Errorf("walkUpForModule(%q) = %q, want %q", nested, got, root)
	}
}

func TestWalkUpForModuleNotFound(t *testing.T) {
	dir := t.TempDir()
	if got := walkUpForModule(dir); got != "" {
		t.Errorf("walkUpForModule(%q) = %q, want empty (no go.mod anywhere up)", dir, got)
	}
}

func TestWalkUpForModuleWrongModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module something/else\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := walkUpForModule(dir); got != "" {
		t.Errorf("walkUpForModule with unrelated module = %q, want empty", got)
	}
}

func TestFindModuleRootADVPPSrcOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+standaloneModule+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADVPP_SRC", root)

	got, err := findModuleRoot()
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}
	if got != root {
		t.Errorf("findModuleRoot() = %q, want %q", got, root)
	}
}

func TestFindModuleRootNotFound(t *testing.T) {
	t.Setenv("ADVPP_SRC", t.TempDir())
	t.Chdir(t.TempDir())

	if _, err := findModuleRoot(); err == nil {
		t.Error("expected an error when no checkout of the module is findable")
	}
}
