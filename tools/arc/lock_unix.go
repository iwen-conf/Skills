//go:build linux || darwin || freebsd || netbsd || openbsd

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// withArcLock serializes all arc.db operations, including verification subprocesses.
// Legacy direct arc-verify invocations do not participate; migration therefore requires
// an explicit offline assertion.
func withArcLock(root string, fn func() error) error {
	dir := filepath.Join(root, ".arc")
	if err := os.MkdirAll(dir, 0700); err != nil { return err }
	st, err := os.Lstat(dir)
	if err != nil { return err }
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 { return errors.New("unsafe .arc directory: symlinks are forbidden") }
	lock, err := os.OpenFile(filepath.Join(dir, "arc.lock"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil { return err }
	defer lock.Close()
	var last error
	for deadline := time.Now().Add(8*time.Second); ; {
		last = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if last == nil { break }
		if !errors.Is(last, syscall.EWOULDBLOCK) && !errors.Is(last, syscall.EAGAIN) {
			return fmt.Errorf("arc lock failed: %w", last)
		}
		if time.Now().After(deadline) { return errors.New("arc.db is busy; another arc operation holds the repository lock") }
		time.Sleep(50*time.Millisecond)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}
