package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	ErrCommandBlocked = errors.New("sandbox: command not allowed by policy")
	ErrPathBlocked    = errors.New("sandbox: path not allowed by policy")
	ErrOutputTooLarge = errors.New("sandbox: output exceeded max bytes")
)

// ExecuteRequest describes a command to run inside the sandbox.
type ExecuteRequest struct {
	Command string   // the program to run
	Args    []string // arguments
	WorkDir string   // working directory (optional)
	Stdin   string   // optional stdin input
}

// ExecuteResult holds the output of a sandboxed execution.
type ExecuteResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Sandbox provides a safe execution environment.
type Sandbox interface {
	Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error)
}

// LocalSandbox executes commands locally with Policy checks.
type LocalSandbox struct {
	Policy Policy
}

func NewLocalSandbox(p Policy) *LocalSandbox {
	return &LocalSandbox{Policy: p}
}

func (s *LocalSandbox) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	if !s.commandAllowed(req.Command) {
		return ExecuteResult{}, fmt.Errorf("%w: %s", ErrCommandBlocked, req.Command)
	}
	if req.WorkDir != "" && !s.pathAllowed(req.WorkDir) {
		return ExecuteResult{}, fmt.Errorf("%w: %s", ErrPathBlocked, req.WorkDir)
	}

	timeout := s.Policy.MaxExecTime
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}
	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ExecuteResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if s.Policy.MaxOutputBytes > 0 && (stdout.Len()+stderr.Len()) > s.Policy.MaxOutputBytes {
		return result, ErrOutputTooLarge
	}

	return result, err
}

func (s *LocalSandbox) commandAllowed(cmd string) bool {
	if len(s.Policy.AllowedCmds) == 0 {
		return true // no whitelist means allow all
	}
	for _, allowed := range s.Policy.AllowedCmds {
		if cmd == allowed {
			return true
		}
	}
	return false
}

func (s *LocalSandbox) pathAllowed(path string) bool {
	for _, blocked := range s.Policy.BlockedPaths {
		if strings.HasPrefix(path, blocked) {
			return false
		}
	}
	if len(s.Policy.AllowedPaths) == 0 {
		return true
	}
	for _, allowed := range s.Policy.AllowedPaths {
		if strings.HasPrefix(path, allowed) {
			return true
		}
	}
	return false
}

var _ Sandbox = (*LocalSandbox)(nil)
