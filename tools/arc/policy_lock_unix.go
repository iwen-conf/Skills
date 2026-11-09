//go:build linux || darwin || freebsd || netbsd || openbsd

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// withPolicyTaskLock follows the same flock protocol as arc-policy.store.task_lock.
// Lock order: policy task.lock -> arc.lock. Do not call back into arc-policy
// while holding these locks; its status may invoke the graph verifier.
func withPolicyTaskLock(root string, fn func() error) error {
	dir := filepath.Join(root, ".arc", "policy")
	if err:=os.MkdirAll(dir,0700);err!=nil{return err}
	st, err := os.Lstat(dir)
	if err != nil { return fmt.Errorf("policy task lock directory missing: %w", err) }
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe policy task lock directory")
	}
	path := filepath.Join(dir, "task.lock")
	// Fail closed if an existing lock path is not an ordinary file. This
	// avoids following a symlink into a file outside the current project.
	if existing, statErr := os.Lstat(path); statErr == nil {
		if !existing.Mode().IsRegular() { return errors.New("unsafe Arc Policy task.lock path") }
	} else if !errors.Is(statErr, os.ErrNotExist) { return statErr }
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil { return err }
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("Arc Policy task is being changed or verified; retry completion: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
