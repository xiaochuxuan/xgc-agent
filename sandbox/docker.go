package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"strings"
)

const (
	defaultDockerBinary  = "docker"
	defaultContainerWork = "/workspace"
)

// DockerSandbox executes commands in a reusable Docker container.
type DockerSandbox struct {
	Policy           Policy
	Image            string // container image to run
	DockerBinary     string // docker executable path, defaults to "docker"
	ContainerWorkDir string // workdir path inside container
	DisableNetwork   bool   // if true use --network none
	ReadOnlyRootFS   bool   // if true use --read-only

	mu          sync.Mutex
	containerID string
	hostWorkDir string
}

// NewDockerSandbox creates a Docker-backed sandbox with safe defaults.
func NewDockerSandbox(p Policy, image string) *DockerSandbox {
	if image == "" {
		image = "alpine:3.20"
	}
	return &DockerSandbox{
		Policy:           p,
		Image:            image,
		DockerBinary:     defaultDockerBinary,
		ContainerWorkDir: defaultContainerWork,
		DisableNetwork:   true,
		ReadOnlyRootFS:   false,
	}
}

func (s *DockerSandbox) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
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

	containerID, err := s.ensureContainer(ctx, req.WorkDir)
	if err != nil {
		return ExecuteResult{}, err
	}

	dockerArgs := []string{"exec", "-i"}
	if req.WorkDir != "" {
		dockerArgs = append(dockerArgs, "--workdir", s.containerWorkDir())
	}
	dockerArgs = append(dockerArgs, containerID, req.Command)
	dockerArgs = append(dockerArgs, req.Args...)

	cmd := exec.CommandContext(ctx, s.dockerBinary(), dockerArgs...)
	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

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

// Close explicitly stops and removes the reused container.
func (s *DockerSandbox) Close(ctx context.Context) error {
	s.mu.Lock()
	containerID := s.containerID
	if containerID == "" {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	cmd := exec.CommandContext(ctx, s.dockerBinary(), "rm", "-f", containerID)
	if err := cmd.Run(); err != nil {
		return err
	}

	s.mu.Lock()
	if s.containerID == containerID {
		s.containerID = ""
		s.hostWorkDir = ""
	}
	s.mu.Unlock()

	return nil
}

func (s *DockerSandbox) ensureContainer(ctx context.Context, workDir string) (string, error) {
	s.mu.Lock()
	if s.containerID != "" {
		if err := s.validateWorkDirLocked(workDir); err != nil {
			s.mu.Unlock()
			return "", err
		}
		id := s.containerID
		s.mu.Unlock()
		return id, nil
	}

	dockerArgs := []string{"run", "-d", "--rm"}
	if s.DisableNetwork {
		dockerArgs = append(dockerArgs, "--network", "none")
	}
	if s.ReadOnlyRootFS {
		dockerArgs = append(dockerArgs, "--read-only")
	}
	if workDir != "" {
		mountSpec := fmt.Sprintf("type=bind,source=%s,target=%s", workDir, s.containerWorkDir())
		dockerArgs = append(dockerArgs, "--mount", mountSpec, "--workdir", s.containerWorkDir())
	}
	dockerArgs = append(dockerArgs, s.Image, "sh", "-c", "while true; do sleep 3600; done")

	cmd := exec.CommandContext(ctx, s.dockerBinary(), dockerArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	s.mu.Unlock()

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("start sandbox container: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("start sandbox container: %w", err)
	}

	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		return "", fmt.Errorf("start sandbox container: empty container id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.containerID == "" {
		s.containerID = containerID
		s.hostWorkDir = workDir
		return s.containerID, nil
	}

	// Another goroutine won the race; cleanup duplicate container and reuse existing one.
	if err := exec.CommandContext(ctx, s.dockerBinary(), "rm", "-f", containerID).Run(); err != nil {
		return "", err
	}
	if err := s.validateWorkDirLocked(workDir); err != nil {
		return "", err
	}
	return s.containerID, nil
}

func (s *DockerSandbox) validateWorkDirLocked(workDir string) error {
	if workDir == "" || s.hostWorkDir == workDir {
		return nil
	}
	if s.hostWorkDir == "" {
		return fmt.Errorf("sandbox: container started without workdir mount; cannot execute with workdir %q", workDir)
	}
	return fmt.Errorf("sandbox: container already bound to workdir %q, got %q", s.hostWorkDir, workDir)
}

func (s *DockerSandbox) dockerBinary() string {
	if s.DockerBinary != "" {
		return s.DockerBinary
	}
	return defaultDockerBinary
}

func (s *DockerSandbox) containerWorkDir() string {
	if s.ContainerWorkDir != "" {
		return s.ContainerWorkDir
	}
	return defaultContainerWork
}

func (s *DockerSandbox) commandAllowed(cmd string) bool {
	if len(s.Policy.AllowedCmds) == 0 {
		return true
	}
	for _, allowed := range s.Policy.AllowedCmds {
		if cmd == allowed {
			return true
		}
	}
	return false
}

func (s *DockerSandbox) pathAllowed(path string) bool {
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

var _ Sandbox = (*DockerSandbox)(nil)

