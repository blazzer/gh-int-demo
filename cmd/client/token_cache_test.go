package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteTokenCache_FileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file mode 0600 is not enforced on Windows")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	*noCache = false
	token := "gho_test_secret_token"
	if err := writeTokenCache(token); err != nil {
		t.Fatalf("writeTokenCache() error = %v", err)
	}

	path := filepath.Join(dir, ".gh-int-demo-token")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestWriteTokenCache_NoCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	*noCache = true
	if err := writeTokenCache("gho_test"); err != nil {
		t.Fatalf("writeTokenCache() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gh-int-demo-token")); !os.IsNotExist(err) {
		t.Fatal("expected no cache file when --no-cache")
	}
	*noCache = false
}
