//go:build !linux

package main

import "log/slog"

// setProcessRlimits is a no-op outside Linux: prlimit(2) has no portable equivalent, so
// --rlimit-cpu/--rlimit-mem are accepted but not enforced on other platforms.
var setProcessRlimits = func(pid int, cpuSeconds int64, memBytes int64) error {
	slog.Warn("rlimit is not supported on this platform, ignoring", "cpu-seconds", cpuSeconds, "mem-bytes", memBytes)
	return nil
}
