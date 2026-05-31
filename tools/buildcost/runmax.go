//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: runmax cmd args...")
		os.Exit(2)
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	start := time.Now()
	err := cmd.Run()
	el := time.Since(start)
	var maxrss int64
	if cmd.ProcessState != nil {
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			maxrss = ru.Maxrss // KB on Linux
		}
	}
	fmt.Fprintf(os.Stderr, "WALL=%.2fs  PEAK_CHILD_RSS=%.1fMB\n", el.Seconds(), float64(maxrss)/1024.0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cmd error: %v\n", err)
		os.Exit(1)
	}
}