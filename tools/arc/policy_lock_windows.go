//go:build windows

package main

import "errors"

// Fail closed until a Windows msvcrt byte-range lock adapter is implemented.
// arc-policy uses msvcrt.locking, not an exclusive-create lockfile.
func withPolicyTaskLock(root string, fn func() error) error {
	return errors.New("Arc Policy task completion requires a compatible Windows policy lock; not implemented")
}
