package reviewstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFlockAcquireRelease_MutualExclusion exercises the real unix.Flock
// syscall (not stubbed): two independent flockAcquire calls on the same
// path must serialize, proving the exclusive-lock invariant the authority
// store's CAS read-modify-write depends on (design "Single-writer
// locking").
func TestFlockAcquireRelease_MutualExclusion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	f1, err := flockAcquire(path)
	if err != nil {
		t.Fatalf("first flockAcquire() error = %v, want nil", err)
	}

	acquired := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		f2, err := flockAcquire(path)
		close(acquired)
		if err != nil {
			done <- err
			return
		}
		done <- flockRelease(f2)
	}()

	select {
	case <-acquired:
		t.Fatal("second flockAcquire() returned before the first lock was released")
	case <-time.After(100 * time.Millisecond):
		// expected: second acquire is still blocked on the held lock
	}

	if err := flockRelease(f1); err != nil {
		t.Fatalf("flockRelease(f1) error = %v, want nil", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("second flockAcquire/flockRelease error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second flockAcquire() did not complete after the first lock was released")
	}
}

// TestFlockAcquire_ErrorOnUnwritablePath asserts flockAcquire surfaces the
// underlying os.OpenFile error rather than panicking or silently ignoring
// it, when the lock file's parent directory does not exist.
func TestFlockAcquire_ErrorOnUnwritablePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-subdir", ".lock")

	if _, err := flockAcquire(path); err == nil {
		t.Fatal("flockAcquire() error = nil, want error for a path inside a nonexistent directory")
	}
}

// TestFlockRelease_ErrorOnClosedFile asserts flockRelease propagates the
// unix.Flock error when the file descriptor was already closed out from
// under it, rather than swallowing it.
func TestFlockRelease_ErrorOnClosedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	f, err := flockAcquire(path)
	if err != nil {
		t.Fatalf("flockAcquire() error = %v, want nil", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("f.Close() error = %v, want nil", err)
	}

	if err := flockRelease(f); err == nil {
		t.Error("flockRelease() error = nil, want error for an already-closed file")
	}
}

// TestFlockAcquireSeam_Swappable proves flockAcquire is a package-level var
// seam: callers in other packages (e.g. reviewstore.Store, in a later PR)
// can inject an error without a real filesystem or flock syscall.
func TestFlockAcquireSeam_Swappable(t *testing.T) {
	original := flockAcquire
	t.Cleanup(func() { flockAcquire = original })

	wantErr := errors.New("boom")
	flockAcquire = func(path string) (*os.File, error) {
		return nil, wantErr
	}

	if _, err := flockAcquire("irrelevant"); !errors.Is(err, wantErr) {
		t.Errorf("flockAcquire() error = %v, want %v", err, wantErr)
	}
}
