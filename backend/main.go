package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const protocolVersion = 1
const maxMessageSize = 1 << 20

type request struct {
	ID              string `json:"id"`
	ProtocolVersion int    `json:"protocolVersion"`
	Command         string `json:"command"`
	URL             string `json:"url"`
	OptionID        string `json:"optionId"`
	JobID           string `json:"jobId"`
	Cookies         string `json:"cookies"`
	Directory       string `json:"directory"`
	Force           bool   `json:"force"`
}

type response struct {
	ID              string          `json:"id"`
	ProtocolVersion int             `json:"protocolVersion"`
	Tools           map[string]tool `json:"tools,omitempty"`
	Error           string          `json:"error,omitempty"`
	RetryAfter      int             `json:"retryAfter,omitempty"`
	Title           string          `json:"title,omitempty"`
	Options         []option        `json:"options,omitempty"`
	Job             *job            `json:"job,omitempty"`
	Directory       string          `json:"directory,omitempty"`
}

type tool struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

func main() {
	configureToolPath()
	cleanStaleCookies()
	if err := serve(os.Stdin, os.Stdout, detectTools); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func configureToolPath() {
	paths := []string{os.Getenv("PATH")}
	if exe, err := os.Executable(); err == nil {
		paths = append([]string{filepath.Join(filepath.Dir(exe), "tools")}, paths...)
	}
	if runtime.GOOS == "darwin" {
		paths = append(paths, "/opt/homebrew/bin", "/usr/local/bin")
	}
	_ = os.Setenv("PATH", strings.Join(paths, string(os.PathListSeparator)))
}

func serve(input io.Reader, output io.Writer, tools func() map[string]tool) error {
	host := &service{}
	defer host.close()
	for {
		var size uint32
		err := binary.Read(input, binary.NativeEndian, &size)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if size == 0 || size > maxMessageSize {
			return fmt.Errorf("invalid native message size: %d", size)
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(input, payload); err != nil {
			return err
		}

		var req request
		reply := response{ProtocolVersion: protocolVersion}
		switch {
		case json.Unmarshal(payload, &req) != nil || req.ID == "" || len(req.ID) > 128:
			reply.Error = "invalid_request"
		case req.ProtocolVersion != protocolVersion:
			reply.Error = "protocol_mismatch"
		case req.Command == "hello":
			reply.Tools = tools()
		default:
			host.handle(req, &reply)
		}
		if reply.Error != "invalid_request" {
			reply.ID = req.ID
		}
		data, err := json.Marshal(reply)
		if err != nil {
			return err
		}
		if len(data) > maxMessageSize {
			return errors.New("native response exceeds message limit")
		}
		if err := binary.Write(output, binary.NativeEndian, uint32(len(data))); err != nil {
			return err
		}
		if _, err := output.Write(data); err != nil {
			return err
		}
	}
}

func detectTools() map[string]tool {
	ytDLP, ejs := detectYTDLP()
	return map[string]tool{
		"yt-dlp":  ytDLP,
		"ffmpeg":  probe("ffmpeg", "-version"),
		"ffprobe": probe("ffprobe", "-version"),
		"deno":    probe("deno", "--version"),
		"ejs":     ejs,
	}
}

// Use local metadata so the diagnostics do not require a YouTube request.
func detectYTDLP() (tool, tool) {
	fallback := func() (tool, tool) {
		return probe("yt-dlp", "--ignore-config", "--version"), tool{Status: "unverified"}
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fallback()
	}
	file, err := os.CreateTemp("", "ytdlp-host-probe-*.json")
	if err != nil {
		return fallback()
	}
	defer os.Remove(file.Name())
	_, err = io.WriteString(file, `{"id":"probe","title":"probe","url":"https://example.invalid/probe.mp4","ext":"mp4"}`)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return fallback()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := downloader(ctx, "", "--verbose", "--simulate", "--load-info-json", file.Name())
	output, _ := cmd.CombinedOutput()
	ytDLP := tool{Status: "unavailable"}
	ejs := tool{Status: "unverified"}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if details, found := strings.CutPrefix(line, "[debug] yt-dlp version "); found {
			version, _, _ := strings.Cut(details, " ")
			if _, number, found := strings.Cut(version, "@"); found {
				version = number
			}
			if version != "" {
				ytDLP = tool{Status: "available", Version: version}
			}
		}
		libraries, found := strings.CutPrefix(line, "[debug] Optional libraries: ")
		if !found {
			continue
		}
		ejs = tool{Status: "missing"}
		for _, library := range strings.Split(libraries, ", ") {
			if version, found := strings.CutPrefix(library, "yt_dlp_ejs-"); found {
				ejs = tool{Status: "available", Version: version}
				break
			}
		}
	}
	if ytDLP.Status == "unavailable" {
		ytDLP = probe("yt-dlp", "--ignore-config", "--version")
	}
	return ytDLP, ejs
}

func probe(name string, args ...string) tool {
	path, err := exec.LookPath(name)
	if errors.Is(err, exec.ErrNotFound) {
		return tool{Status: "missing"}
	}
	if err != nil {
		return tool{Status: "unavailable"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	configureProcess(cmd)
	output, err := cmd.Output()
	if err != nil {
		return tool{Status: "unavailable"}
	}
	version, _, _ := strings.Cut(string(output), "\n")
	version = strings.TrimSpace(version)
	if version == "" || len(version) > 512 {
		return tool{Status: "unavailable"}
	}
	return tool{Status: "available", Version: version}
}
