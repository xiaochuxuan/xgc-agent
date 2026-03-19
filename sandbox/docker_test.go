package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDockerSandboxExecuteSuccess(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "docker_args.txt")
	fakeDocker := writeLifecycleFakeDocker(t, argsFile)

	sb := NewDockerSandbox(Policy{AllowedCmds: []string{"echo"}}, "alpine:3.20")
	sb.DockerBinary = fakeDocker

	res, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo", Args: []string{"hello"}})
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if !strings.Contains(res.Stdout, "docker-ok") {
		t.Fatalf("stdout mismatch, got: %q", res.Stdout)
	}
	if !strings.Contains(res.Stderr, "docker-err") {
		t.Fatalf("stderr mismatch, got: %q", res.Stderr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code mismatch, got: %d", res.ExitCode)
	}

	if err := sb.Close(context.Background()); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}
}

func TestDockerSandboxCommandBlocked(t *testing.T) {
	sb := NewDockerSandbox(Policy{AllowedCmds: []string{"ls"}}, "alpine:3.20")

	_, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo"})
	if !errors.Is(err, ErrCommandBlocked) {
		t.Fatalf("expected ErrCommandBlocked, got: %v", err)
	}
}

func TestDockerSandboxPathBlocked(t *testing.T) {
	blockedRoot := t.TempDir()
	sb := NewDockerSandbox(Policy{
		AllowedCmds: []string{"echo"},
		BlockedPaths: []string{blockedRoot},
	}, "alpine:3.20")

	_, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo", WorkDir: blockedRoot})
	if !errors.Is(err, ErrPathBlocked) {
		t.Fatalf("expected ErrPathBlocked, got: %v", err)
	}
}

func TestDockerSandboxOutputTooLarge(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "docker_args.txt")
	outFile := filepath.Join(tmp, "docker_out.txt")
	if err := os.WriteFile(outFile, []byte("abcdef\n"), 0o644); err != nil {
		t.Fatalf("failed writing out file: %v", err)
	}
	unixScript := fmt.Sprintf("echo \"$*\" >> %q\nif [ \"$1\" = \"run\" ]; then\n  echo cid-1\n  exit 0\nfi\nif [ \"$1\" = \"exec\" ]; then\n  cat %q\n  exit 0\nfi\nif [ \"$1\" = \"rm\" ]; then\n  exit 0\nfi\nexit 1\n", argsFile, outFile)
	windowsScript := fmt.Sprintf("@echo off\r\necho %%*>>%q\r\nif \"%%1\"==\"run\" (\r\n  echo cid-1\r\n  exit /b 0\r\n)\r\nif \"%%1\"==\"exec\" (\r\n  type %q\r\n  exit /b 0\r\n)\r\nif \"%%1\"==\"rm\" (\r\n  exit /b 0\r\n)\r\nexit /b 1\r\n", argsFile, outFile)
	fakeDocker := writeFakeDocker(t, unixScript, windowsScript)

	sb := NewDockerSandbox(Policy{
		AllowedCmds:    []string{"echo"},
		MaxOutputBytes: 4,
	}, "alpine:3.20")
	sb.DockerBinary = fakeDocker

	res, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo"})
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge, got: %v", err)
	}
	if !strings.Contains(res.Stdout, "abcdef") {
		t.Fatalf("expected captured output, got: %q", res.Stdout)
	}

	if err := sb.Close(context.Background()); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}
}

func TestDockerSandboxReusesContainerUntilClose(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "docker_args.txt")
	fakeDocker := writeLifecycleFakeDocker(t, argsFile)

	workDir := t.TempDir()
	sb := NewDockerSandbox(Policy{AllowedCmds: []string{"echo"}}, "img:v1")
	sb.DockerBinary = fakeDocker
	sb.ReadOnlyRootFS = true

	_, err := sb.Execute(context.Background(), ExecuteRequest{
		Command: "echo",
		Args:    []string{"a", "b"},
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	_, err = sb.Execute(context.Background(), ExecuteRequest{
		Command: "echo",
		Args:    []string{"c"},
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("Execute second call returned unexpected error: %v", err)
	}
	if err := sb.Close(context.Background()); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed reading args file: %v", err)
	}
	args := string(data)

	if strings.Count(args, "run -d --rm") != 1 {
		t.Fatalf("expected one docker run, got args: %q", args)
	}
	if strings.Count(args, "exec -i --workdir /workspace cid-1 echo") != 2 {
		t.Fatalf("expected two docker exec calls, got args: %q", args)
	}
	if strings.Count(args, "rm -f cid-1") != 1 {
		t.Fatalf("expected one docker rm -f, got args: %q", args)
	}

	mustContainAll(t, args,
		"--network none",
		"--read-only",
		"--mount",
		"--workdir /workspace",
		"img:v1 sh -c",
		"while true; do sleep 3600; done",
	)
	if !strings.Contains(args, "source="+workDir) {
		t.Fatalf("docker args missing source mount, got: %q", args)
	}
	if !strings.Contains(args, "target=/workspace") {
		t.Fatalf("docker args missing target mount, got: %q", args)
	}
}

func TestDockerSandboxCloseIsIdempotent(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "docker_args.txt")
	fakeDocker := writeLifecycleFakeDocker(t, argsFile)

	sb := NewDockerSandbox(Policy{AllowedCmds: []string{"echo"}}, "alpine:3.20")
	sb.DockerBinary = fakeDocker

	if _, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo"}); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if err := sb.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned unexpected error: %v", err)
	}
	if err := sb.Close(context.Background()); err != nil {
		t.Fatalf("second Close returned unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed reading args file: %v", err)
	}
	args := string(data)

	if strings.Count(args, "rm -f cid-1") != 1 {
		t.Fatalf("expected exactly one rm call, got args: %q", args)
	}
}

func TestDockerSandboxRejectsWorkDirChangeAfterStart(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "docker_args.txt")
	fakeDocker := writeLifecycleFakeDocker(t, argsFile)

	sb := NewDockerSandbox(Policy{AllowedCmds: []string{"echo"}}, "alpine:3.20")
	sb.DockerBinary = fakeDocker

	if _, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo", WorkDir: t.TempDir()}); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	_, err := sb.Execute(context.Background(), ExecuteRequest{Command: "echo", WorkDir: t.TempDir()})
	if err == nil {
		t.Fatalf("expected error when changing workdir after container starts")
	}
}

func writeFakeDocker(t *testing.T, unixBody, windowsBody string) string {
	t.Helper()
	dir := t.TempDir()
	var (
		path string
		body string
	)
	if runtime.GOOS == "windows" {
		path = filepath.Join(dir, "fake-docker.cmd")
		body = windowsBody
	} else {
		path = filepath.Join(dir, "fake-docker.sh")
		body = "#!/bin/sh\n" + unixBody
	}

	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("failed to write fake docker binary: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatalf("failed to chmod fake docker binary: %v", err)
		}
	}
	return path
}

func writeLifecycleFakeDocker(t *testing.T, argsFile string) string {
	t.Helper()
	unixBody := fmt.Sprintf("echo \"$*\" >> %q\nif [ \"$1\" = \"run\" ]; then\n  echo cid-1\n  exit 0\nfi\nif [ \"$1\" = \"exec\" ]; then\n  echo docker-ok\n  echo docker-err 1>&2\n  exit 0\nfi\nif [ \"$1\" = \"rm\" ]; then\n  exit 0\nfi\nexit 1\n", argsFile)
	windowsBody := fmt.Sprintf("@echo off\r\necho %%*>>%q\r\nif \"%%1\"==\"run\" (\r\n  echo cid-1\r\n  exit /b 0\r\n)\r\nif \"%%1\"==\"exec\" (\r\n  echo docker-ok\r\n  echo docker-err 1>&2\r\n  exit /b 0\r\n)\r\nif \"%%1\"==\"rm\" (\r\n  exit /b 0\r\n)\r\nexit /b 1\r\n", argsFile)
	return writeFakeDocker(t, unixBody, windowsBody)
}

func mustContainAll(t *testing.T, text string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(text, sub) {
			t.Fatalf("missing substring %q in %q", sub, text)
		}
	}
}

