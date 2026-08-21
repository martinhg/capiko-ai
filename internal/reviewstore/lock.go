package reviewstore

import (
	"os"

	"golang.org/x/sys/unix"
)

// flockAcquire opens (creating if necessary) the lock file at path and
// acquires an exclusive, blocking advisory lock on it via
// unix.Flock(fd, LOCK_EX). It blocks until the lock becomes available —
// callers do not need to retry or poll. The caller MUST release the
// returned file via flockRelease once the critical section is done;
// otherwise the lock is held (and the fd leaked) until process exit.
//
// This is the single-writer primitive the authority store's
// read-modify-write cycle depends on (design "Single-writer locking",
// spec rdd-authority-store "Concurrent RMW serialized"). It is a
// package-level var seam so callers in other packages (e.g. Store, added
// in a later PR) can stub it in tests without a real filesystem or flock
// syscall.
var flockAcquire = func(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// flockRelease releases the advisory lock held on f via
// unix.Flock(fd, LOCK_UN) and then closes f. Package-level var seam,
// mirroring flockAcquire.
var flockRelease = func(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
