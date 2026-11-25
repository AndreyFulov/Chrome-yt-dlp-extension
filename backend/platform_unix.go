//go:build !windows

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const cookieNewline = "\n"

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

func pickCookieFile(ctx context.Context) (string, error) {
	script := `try
 return POSIX path of (choose file with prompt "Select YouTube cookies.txt")
 on error number -128
 return ""
 end try`
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	configureProcess(cmd)
	output, err := cmd.Output()
	return strings.TrimSuffix(string(output), "\n"), err
}

func pickDownloadDirectory(ctx context.Context, initial string) (string, error) {
	script := `on run argv
 try
  if item 1 of argv is "" then
   return POSIX path of (choose folder with prompt "Select the download folder")
  end if
  return POSIX path of (choose folder with prompt "Select the download folder" default location (POSIX file (item 1 of argv)))
 on error number -128
  return ""
 end try
 end run`
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script, initial)
	configureProcess(cmd)
	output, err := cmd.Output()
	return strings.TrimSuffix(string(output), "\n"), err
}

func commitFile(source, target string) error {
	if err := os.Link(source, target); !errors.Is(err, syscall.ENOTSUP) && !errors.Is(err, syscall.ENOSYS) {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	if err = errors.Join(copyErr, output.Close()); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func downloadsDirectory(home string) string { return filepath.Join(home, "Downloads") }

func processExited(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}
