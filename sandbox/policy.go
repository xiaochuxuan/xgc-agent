package sandbox

import "time"

// Policy defines security constraints for sandbox execution.
type Policy struct {
	AllowedPaths   []string      // path prefixes allowed for file access
	BlockedPaths   []string      // path prefixes explicitly blocked
	AllowedCmds    []string      // commands allowed to execute
	MaxExecTime    time.Duration // max execution time per command
	MaxOutputBytes int           // max bytes of stdout+stderr
}

// DefaultPolicy returns a restrictive default policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxExecTime:    30 * time.Second,
		MaxOutputBytes: 1 << 20, // 1MB
	}
}
