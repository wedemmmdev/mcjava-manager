//go:build unix

package instance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareReturnsPermissionError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	root := t.TempDir()
	locked := filepath.Join(root, "locked")

	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o755); err != nil {
			t.Errorf("cleanup Chmod() error = %v", err)
		}
	})

	_, err := Prepare(locked)
	if err == nil {
		t.Fatal("Prepare() error = nil, want a permission error")
	}

	var permissionErr *PermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("Prepare() error = %T, want *PermissionError", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Prepare() error = %v, want os.ErrPermission", err)
	}
}
