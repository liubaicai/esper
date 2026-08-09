//go:build linux

package esper

import (
	"syscall"
	"time"
)

// statementMetricsCPUTime uses process CPU time because a goroutine may move
// between OS threads while evaluating one statement. This is the closest
// stable Go runtime analogue to Esper's JVM current-thread CPU sampler.
func statementMetricsCPUTime() time.Duration {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond +
		time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond
}
