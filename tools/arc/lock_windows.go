//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
)

// Windows uses an exclusive create lock; do not auto-clear a stale lock file.
// A crash requires an explicit investigation before manual recovery.
func withArcLock(root string, fn func() error) error {
	dir := filepath.Join(root, ".arc")
	if err := os.MkdirAll(dir, 0700); err != nil { return err }
	st, err := os.Lstat(dir)
	if err != nil { return err }
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 { return errors.New("unsafe .arc directory: symlinks are forbidden") }
	path := filepath.Join(dir, "arc.lock.exclusive")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) { return errors.New("arc.db lock busy; inspect stale lock before removing") }
	if err != nil { return err }
	defer func(){ f.Close(); os.Remove(path) }()
	return fn()
}
