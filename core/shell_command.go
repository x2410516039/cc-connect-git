package core

import (
	"context"
	"os/exec"
	"runtime"
)

func nativeShellCommandContext(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}
