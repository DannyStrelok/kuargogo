//go:build !windows

package osutil

import (
	"os"
	"os/exec"
)

func isAdmin() bool {
	return os.Geteuid() == 0
}

func elevate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{exe}
	args = append(args, os.Args[1:]...)

	// Use sudo to restart the process
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
