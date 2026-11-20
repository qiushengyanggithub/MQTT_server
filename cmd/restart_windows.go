//go:build windows

package main

import (
	"os"
	"os/exec"
)

func restartProcess(exe string) error {
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Dir, _ = os.Getwd()
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
