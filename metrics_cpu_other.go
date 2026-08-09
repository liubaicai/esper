//go:build !linux

package esper

import "time"

// Go does not expose a portable current-goroutine or process CPU clock.
func statementMetricsCPUTime() time.Duration { return 0 }
