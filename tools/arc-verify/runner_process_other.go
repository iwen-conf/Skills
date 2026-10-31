//go:build !darwin && !linux

package main

import (
	"errors"
	"os/exec"
)

func configureRunnerProcess(command *exec.Cmd) error {
	return errors.New("command runner requires Darwin or Linux process-group cancellation")
}

func terminateRunnerProcess(command *exec.Cmd) error {
	return errors.New("command runner process-group cancellation is unavailable")
}
