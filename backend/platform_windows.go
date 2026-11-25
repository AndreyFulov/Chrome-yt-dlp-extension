package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const cookieNewline = "\r\n"

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		kill := exec.CommandContext(ctx, "taskkill.exe", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if kill.Run() == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
}

func pickCookieFile(ctx context.Context) (string, error) {
	script := `[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
 Add-Type -AssemblyName System.Windows.Forms
 $dialog = [Windows.Forms.OpenFileDialog]::new()
 $dialog.Title = 'Select YouTube cookies.txt'
 $dialog.Filter = 'Text files (*.txt)|*.txt|All files (*.*)|*.*'
 if ($dialog.ShowDialog() -eq [Windows.Forms.DialogResult]::OK) { [Console]::Write($dialog.FileName) }`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", script)
	configureProcess(cmd)
	output, err := cmd.Output()
	return string(output), err
}

func pickDownloadDirectory(ctx context.Context, initial string) (string, error) {
	script := `[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
 Add-Type -AssemblyName System.Windows.Forms
 $dialog = [Windows.Forms.FolderBrowserDialog]::new()
 $dialog.Description = 'Select the download folder'
 $dialog.ShowNewFolderButton = $true
 if ($env:YTDLP_HOST_START_DIRECTORY) { $dialog.SelectedPath = $env:YTDLP_HOST_START_DIRECTORY }
 $owner = [Windows.Forms.Form]::new()
 $owner.ShowInTaskbar = $false
 $owner.TopMost = $true
 $owner.Opacity = 0
 try {
     $owner.Show()
     if ($dialog.ShowDialog($owner) -eq [Windows.Forms.DialogResult]::OK) { [Console]::Write($dialog.SelectedPath) }
 } finally {
     $dialog.Dispose()
     $owner.Dispose()
 }`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", script)
	cmd.Env = append(os.Environ(), "YTDLP_HOST_START_DIRECTORY="+initial)
	configureProcess(cmd)
	output, err := cmd.Output()
	return string(output), err
}

func commitFile(source, target string) error {
	longPath := func(path string) string {
		if len(path) < 248 || strings.HasPrefix(path, `\\?\`) {
			return path
		}
		if strings.HasPrefix(path, `\\`) {
			return `\\?\UNC\` + path[2:]
		}
		return `\\?\` + path
	}
	from, err := syscall.UTF16PtrFromString(longPath(source))
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(longPath(target))
	if err != nil {
		return err
	}
	if err = syscall.MoveFile(from, to); err != nil {
		return &os.LinkError{Op: "move", Old: source, New: target, Err: err}
	}
	return nil
}

func downloadsDirectory(home string) string {
	script := `[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
 $shellFolders = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders'
 $path = (Get-ItemProperty -LiteralPath $shellFolders).'{374DE290-123F-4565-9164-39C4925E467B}'
 if ($path) { [Console]::Write([Environment]::ExpandEnvironmentVariables($path)) }`
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	configureProcess(cmd)
	output, err := cmd.Output()
	path := strings.TrimSpace(string(output))
	if err == nil && filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(home, "Downloads")
}

func processExited(pid int) bool {
	const synchronize = 0x00100000
	const invalidParameter = syscall.Errno(87)
	handle, err := syscall.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		return errors.Is(err, invalidParameter)
	}
	defer syscall.CloseHandle(handle)
	status, err := syscall.WaitForSingleObject(handle, 0)
	return err == nil && status == syscall.WAIT_OBJECT_0
}
