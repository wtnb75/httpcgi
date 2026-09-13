//go:build linux

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// setProcessRlimits limits the CPU time (seconds) and address space (bytes) of the
// process identified by pid, using prlimit(2) so it can be applied to a child process
// right after it starts. A zero value leaves that particular limit unset.
var setProcessRlimits = func(pid int, cpuSeconds int64, memBytes int64) error {
	if cpuSeconds > 0 {
		lim := unix.Rlimit{Cur: uint64(cpuSeconds), Max: uint64(cpuSeconds)}
		if err := unix.Prlimit(pid, unix.RLIMIT_CPU, &lim, nil); err != nil {
			return fmt.Errorf("rlimit cpu: %w", err)
		}
	}
	if memBytes > 0 {
		lim := unix.Rlimit{Cur: uint64(memBytes), Max: uint64(memBytes)}
		if err := unix.Prlimit(pid, unix.RLIMIT_AS, &lim, nil); err != nil {
			return fmt.Errorf("rlimit mem: %w", err)
		}
	}
	return nil
}
