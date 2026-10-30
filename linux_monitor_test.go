package linuxmonitor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalcDirSize_RegularFiles(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "a.bin"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(d, "sub"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(d, "sub", "b.bin"), []byte("world!!"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := CalcDirSize(d)
	if err != nil {
		t.Fatalf("CalcDirSize returned error: %v", err)
	}
	if got != 12 {
		t.Errorf("got %d bytes, want 12", got)
	}
}

func TestCalcDirSize_EmptyDirectory(t *testing.T) {
	d := t.TempDir()
	got, err := CalcDirSize(d)
	if err != nil {
		t.Fatalf("CalcDirSize returned error: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d bytes, want 0", got)
	}
}

func TestCalcDirSize_NonexistentPath(t *testing.T) {
	_, err := CalcDirSize(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestCalcDirSize_SymlinksNotCounted(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "target.bin")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	link := filepath.Join(d, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := CalcDirSize(d)
	if err != nil {
		t.Fatalf("CalcDirSize returned error: %v", err)
	}
	if got != 4 {
		t.Errorf("got %d bytes, want 4 (symlink must not be counted)", got)
	}
}

func TestCalcDirSize_SymlinkToDirectoryIsNotFollowedForSize(t *testing.T) {
	// filepath.Walk does not follow symlinked directories, so the directory's
	// inode entry is skipped by the IsRegular() guard. This test pins that
	// behaviour explicitly.
	d := t.TempDir()
	realDir := filepath.Join(d, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "big.bin"), make([]byte, 100), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(d, "linkdir")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := CalcDirSize(d)
	if err != nil {
		t.Fatalf("CalcDirSize returned error: %v", err)
	}
	if got != 100 {
		t.Errorf("got %d, want 100 (symlinked dir contents must not be double-counted)", got)
	}
}

func TestMonitor_CheckExceedsThreshold(t *testing.T) {
	called := false
	var gotPath string
	var gotSize int64

	m := New("/some/path", 500, func(p string, s int64) {
		called = true
		gotPath = p
		gotSize = s
	})
	m.calc = func(path string) (int64, error) {
		if path != "/some/path" {
			t.Fatalf("calculator got path %q, want %q", path, "/some/path")
		}
		return 501, nil
	}

	size, err := m.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if size != 501 {
		t.Errorf("Check returned %d, want 501", size)
	}
	if !called {
		t.Fatal("callback was not invoked")
	}
	if gotPath != "/some/path" {
		t.Errorf("callback got path %q, want %q", gotPath, "/some/path")
	}
	if gotSize != 501 {
		t.Errorf("callback got size %d, want 501", gotSize)
	}
}

func TestMonitor_CheckBelowThresholdNoCallback(t *testing.T) {
	called := false
	m := New("/some/path", 500, func(p string, s int64) {
		called = true
	})
	m.calc = func(path string) (int64, error) { return 499, nil }

	size, err := m.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if size != 499 {
		t.Errorf("Check returned %d, want 499", size)
	}
	if called {
		t.Fatal("callback should not have been invoked")
	}
}

func TestMonitor_CheckEqualsThresholdNoCallback(t *testing.T) {
	// The contract is strictly-greater-than, so exactly at threshold must not
	// fire. This test pins that decision so a future refactor cannot drift to
	// >= without failing the suite.
	called := false
	m := New("/some/path", 500, func(p string, s int64) {
		called = true
	})
	m.calc = func(path string) (int64, error) { return 500, nil }

	size, err := m.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if size != 500 {
		t.Errorf("Check returned %d, want 500", size)
	}
	if called {
		t.Fatal("callback should not fire when size equals threshold")
	}
}

func TestMonitor_CheckCalcErrorPropagated(t *testing.T) {
	m := New("/some/path", 500, func(p string, s int64) {
		t.Fatal("callback should not be called on error")
	})
	m.calc = func(path string) (int64, error) { return 0, os.ErrNotExist }

	size, err := m.Check()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if size != 0 {
		t.Errorf("Check returned %d, want 0 on error", size)
	}
}

func TestMonitor_RealFilesystem(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "a.bin"), []byte("abcdefgh"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	called := false
	var gotSize int64
	m := New(d, 5, func(p string, s int64) {
		called = true
		gotSize = s
	})

	size, err := m.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if size != 8 {
		t.Errorf("got %d, want 8", size)
	}
	if !called {
		t.Fatal("callback should have fired for 8 > 5")
	}
	if gotSize != 8 {
		t.Errorf("callback got %d, want 8", gotSize)
	}
}
