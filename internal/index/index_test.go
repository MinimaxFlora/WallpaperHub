package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func relPaths(images []Image) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		out = append(out, img.RelPath)
	}
	return out
}

func TestRefreshFiltersAndSorts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "zebra.jpg"), "a")
	writeFile(t, filepath.Join(root, "Alpha.PNG"), "b")
	writeFile(t, filepath.Join(root, "nested", "beta.webp"), "c")
	writeFile(t, filepath.Join(root, "notes.txt"), "d")
	writeFile(t, filepath.Join(root, ".DS_Store"), "e")
	writeFile(t, filepath.Join(root, "nested", "readme.md"), "f")

	ix := New(root)
	count, err := ix.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3 (%v)", count, relPaths(ix.Snapshot()))
	}

	got := relPaths(ix.Snapshot())
	want := []string{"Alpha.PNG", "nested/beta.webp", "zebra.jpg"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path[%d] = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}

	for _, img := range ix.Snapshot() {
		if img.RelPath == "Alpha.PNG" && img.Name != "Alpha" {
			t.Fatalf("Name = %q, want %q", img.Name, "Alpha")
		}
	}
}

func TestRefreshMissingDirKeepsOldSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "one.jpg"), "a")
	ix := New(root)
	if _, err := ix.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh: %v", err)
	}
	if ix.Len() != 1 {
		t.Fatalf("Len = %d, want 1", ix.Len())
	}

	missing := New(filepath.Join(root, "does-not-exist"))
	if _, err := missing.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh on missing dir: expected error")
	}
	if missing.Len() != 0 {
		t.Fatalf("missing dir Len = %d, want 0", missing.Len())
	}
}

func TestRefreshEmptyDir(t *testing.T) {
	root := t.TempDir()
	ix := New(root)
	count, err := ix.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if count != 0 || ix.Len() != 0 {
		t.Fatalf("count = %d, Len = %d, want 0", count, ix.Len())
	}
	if ix.Snapshot() == nil {
		t.Fatal("Snapshot must not be nil for an empty index")
	}
}

func TestRefreshReflectsAddedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"), "a")
	ix := New(root)
	if _, err := ix.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	writeFile(t, filepath.Join(root, "b.jpg"), "b")
	if _, err := ix.Refresh(context.Background()); err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	if ix.Len() != 2 {
		t.Fatalf("Len = %d, want 2", ix.Len())
	}
}
