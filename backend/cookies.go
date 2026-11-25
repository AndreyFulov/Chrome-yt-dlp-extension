package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const cookieLimit = 1 << 20

var cookieExpiry = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func (s *service) chooseCookies() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	path, err := pickCookieFile(ctx)
	if err != nil {
		return "cookies_dialog_failed"
	}
	if path == "" {
		return "cookies_cancelled"
	}
	source, err := os.Open(path)
	if err != nil {
		return "cookies_unreadable"
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "cookies_unreadable"
	}
	if info.Size() > cookieLimit {
		return "cookies_invalid"
	}
	data, err := io.ReadAll(io.LimitReader(source, cookieLimit+1))
	if err != nil {
		return "cookies_unreadable"
	}
	if len(data) > cookieLimit {
		return "cookies_invalid"
	}
	return s.importCookies(string(data))
}

func (s *service) importCookies(data string) string {
	if len(data) > cookieLimit {
		return "cookies_invalid"
	}
	filtered, err := filterCookies(data)
	if err != nil {
		return "cookies_invalid"
	}
	root, err := cookieRoot()
	if err != nil {
		return "write_failed"
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "write_failed"
	}
	dir, err := os.MkdirTemp(root, fmt.Sprintf("cookies-%d-", os.Getpid()))
	if err != nil {
		return "write_failed"
	}
	working := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(working, []byte(filtered), 0600); err != nil {
		os.RemoveAll(dir)
		return "write_failed"
	}
	s.clearCookies()
	s.cookies = working
	s.videoURL, s.options = "", nil
	return ""
}

func filterCookies(data string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	if !scanner.Scan() {
		return "", errors.New("missing cookie header")
	}
	header := strings.TrimPrefix(scanner.Text(), "\ufeff")
	if header != "# HTTP Cookie File" && header != "# Netscape HTTP Cookie File" {
		return "", errors.New("invalid cookie header")
	}
	lines := []string{"# Netscape HTTP Cookie File"}
	for scanner.Scan() {
		line := scanner.Text()
		domainLine := strings.TrimPrefix(line, "#HttpOnly_")
		if strings.TrimSpace(domainLine) == "" || strings.HasPrefix(domainLine, "#") {
			continue
		}
		fields := strings.Split(domainLine, "\t")
		if len(fields) != 7 || fields[2] == "" || fields[2][0] != '/' || (fields[5] == "" && fields[6] == "") || strings.ContainsAny(line, "\x00\r") {
			return "", errors.New("invalid cookie row")
		}
		if (fields[1] != "TRUE" && fields[1] != "FALSE") || (fields[3] != "TRUE" && fields[3] != "FALSE") {
			return "", errors.New("invalid cookie flags")
		}
		if fields[4] != "" && !cookieExpiry.MatchString(fields[4]) {
			return "", errors.New("invalid cookie expiry")
		}
		domain := strings.TrimPrefix(strings.ToLower(fields[0]), ".")
		if domain == "youtube.com" || strings.HasSuffix(domain, ".youtube.com") {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(lines) == 1 {
		return "", errors.New("no YouTube cookies")
	}
	return strings.Join(lines, cookieNewline) + cookieNewline, nil
}

func (s *service) clearCookies() {
	if s.cookies == "" {
		return
	}
	_ = os.RemoveAll(filepath.Dir(s.cookies))
	s.cookies = ""
}

func cleanStaleCookies() {
	root, err := cookieRoot()
	if err != nil {
		return
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		name, valid := strings.CutPrefix(entry.Name(), "cookies-")
		if !entry.IsDir() || !valid {
			continue
		}
		pidText, _, found := strings.Cut(name, "-")
		pid, err := strconv.Atoi(pidText)
		if found && err == nil && pid > 0 && processExited(pid) {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

func cookieRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "YtDlpChromeHost", "sessions"), nil
}
