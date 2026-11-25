package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type option struct {
	ID       string `json:"id"`
	Format   string `json:"format"`
	Label    string `json:"label"`
	selector string
	height   int
}

type job struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Percent    int    `json:"percent"`
	Path       string `json:"path,omitempty"`
	Error      string `json:"error,omitempty"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

type service struct {
	mu                sync.Mutex
	videoURL          string
	options           []option
	videoTitle        string
	inspectionExpires time.Time
	cookies           string
	current           job
	counter           uint64
	cancel            context.CancelFunc
	workers           sync.WaitGroup
}

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var formatID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func canonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return ""
	}
	var id string
	switch strings.ToLower(u.Host) {
	case "youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		}
		if strings.HasPrefix(u.Path, "/shorts/") {
			id = strings.TrimPrefix(u.Path, "/shorts/")
		}
	}
	if !videoID.MatchString(id) {
		return ""
	}
	return "https://www.youtube.com/watch?v=" + id
}

func (s *service) handle(req request, reply *response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch req.Command {
	case "chooseDirectory":
		if s.cancel != nil {
			reply.Error = "busy"
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		initial, _ := selectedDirectory(req.Directory)
		path, err := pickDownloadDirectory(ctx, initial)
		if err != nil {
			reply.Error = "directory_dialog_failed"
		} else if path != "" {
			reply.Directory, reply.Error = selectedDirectory(path)
		}
	case "inspect", "chooseCookies":
		if s.cancel != nil {
			reply.Error = "busy"
			return
		}
		if req.Command == "chooseCookies" {
			reply.Error = s.chooseCookies()
			return
		}
		address := canonicalURL(req.URL)
		if address == "" {
			reply.Error = "invalid_url"
			return
		}
		if s.videoURL != "" && s.videoURL != address {
			s.clearCookies()
		}
		if req.Cookies != "" {
			reply.Error = s.importCookies(req.Cookies)
			if reply.Error != "" {
				return
			}
		}
		if req.Force {
			s.inspectionExpires = time.Time{}
		}
		reply.Title, reply.Options, reply.Error, reply.RetryAfter = s.inspect(req.URL)
		if reply.Error != "" {
			s.clearCookies()
		}
	case "start":
		reply.Job, reply.Error = s.start(req.URL, req.OptionID, req.Directory)
	case "status", "cancel":
		if s.current.ID == "" || (req.JobID != "" && req.JobID != s.current.ID) {
			reply.Error = "job_not_found"
			return
		}
		if req.Command == "cancel" && s.cancel != nil {
			s.cancel()
		}
		snapshot := s.current
		reply.Job = &snapshot
	default:
		reply.Error = "unknown_command"
	}
}

func (s *service) close() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.workers.Wait()
	s.clearCookies()
}

func (s *service) inspect(raw string) (string, []option, string, int) {
	address := canonicalURL(raw)
	if address == "" {
		return "", nil, "invalid_url", 0
	}
	if address == s.videoURL && s.cookies == "" && time.Now().Before(s.inspectionExpires) {
		return s.videoTitle, s.options, "", 0
	}
	s.videoURL, s.options, s.inspectionExpires = "", nil, time.Time{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := downloader(ctx, s.cookies, "--dump-single-json", "--", address)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		code := classify(ctx, err, diagnostics.String(), s.cookies != "")
		if code == "download_failed" {
			code = "extract_failed"
		}
		return "", nil, code, retryDelay(code, diagnostics.String(), time.Now())
	}
	var metadata struct {
		Title   string        `json:"title"`
		Formats []mediaFormat `json:"formats"`
	}
	if json.Unmarshal(data, &metadata) != nil {
		return "", nil, "extract_failed", 0
	}
	options := selectOptions(metadata.Formats)
	if len(options) == 0 {
		return "", nil, "format_unavailable", 0
	}
	s.videoURL, s.options = address, options
	s.videoTitle = metadata.Title
	if s.cookies == "" {
		s.inspectionExpires = time.Now().Add(30 * time.Second)
	}
	return metadata.Title, options, "", 0
}

type mediaFormat struct {
	ID      string  `json:"format_id"`
	Ext     string  `json:"ext"`
	Video   string  `json:"vcodec"`
	Audio   string  `json:"acodec"`
	Height  int     `json:"height"`
	Bitrate float64 `json:"abr"`
	DRM     bool    `json:"has_drm"`
}

func selectOptions(formats []mediaFormat) []option {
	var options []option
	var bestAAC string
	hasAudio := false
	for _, f := range formats {
		if f.DRM || !formatID.MatchString(f.ID) || f.Audio == "" || f.Audio == "none" {
			continue
		}
		hasAudio = true
		if f.Video != "none" || f.Ext != "m4a" || !strings.HasPrefix(f.Audio, "mp4a") {
			continue
		}
		bestAAC = f.ID
		label := "AAC · " + f.ID
		if f.Bitrate > 0 {
			label = fmt.Sprintf("AAC · %.0f кбит/с · %s", f.Bitrate, f.ID)
		}
		options = append(options, option{ID: "m4a-" + f.ID, Format: "m4a", Label: label, selector: f.ID})
	}
	video := map[int]option{}
	for _, f := range formats {
		if f.DRM || f.Ext != "mp4" || !strings.HasPrefix(f.Video, "avc") || f.Height <= 0 || !formatID.MatchString(f.ID) {
			continue
		}
		selector := f.ID
		if f.Audio == "none" && bestAAC != "" {
			selector += "+" + bestAAC
		} else if !strings.HasPrefix(f.Audio, "mp4a") {
			continue
		}
		height := strconv.Itoa(f.Height)
		video[f.Height] = option{ID: "mp4-" + height, Format: "mp4", Label: height + "p", selector: selector, height: f.Height}
	}
	var resolutions []option
	for _, item := range video {
		resolutions = append(resolutions, item)
	}
	sort.Slice(resolutions, func(i, j int) bool { return resolutions[i].height > resolutions[j].height })
	options = append(resolutions, options...)
	if hasAudio {
		for _, rate := range []string{"128", "192", "320"} {
			options = append(options, option{ID: "mp3-" + rate, Format: "mp3", Label: rate + " кбит/с", selector: "bestaudio/best"})
		}
	}
	return options
}

func downloader(ctx context.Context, cookies string, extra ...string) *exec.Cmd {
	args := []string{"--ignore-config", "--no-plugin-dirs", "--no-playlist", "--encoding", "utf-8", "--socket-timeout", "15", "--retries", "2", "--fragment-retries", "2", "--extractor-retries", "1", "--retry-sleep", "http:exp=2:8"}
	if cookies != "" {
		args = append(args, "--cookies", cookies)
	}
	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	ffprobePath, ffprobeErr := exec.LookPath("ffprobe")
	if ffmpegErr == nil && ffprobeErr == nil && filepath.Dir(ffmpegPath) == filepath.Dir(ffprobePath) {
		args = append(args, "--ffmpeg-location", filepath.Dir(ffmpegPath))
	}
	cmd := exec.CommandContext(ctx, "yt-dlp", append(args, extra...)...)
	configureProcess(cmd)
	return cmd
}

func (s *service) start(raw, id, directory string) (*job, string) {
	if s.cancel != nil {
		return nil, "busy"
	}
	if s.videoURL == "" || (raw != "" && canonicalURL(raw) != s.videoURL) {
		return nil, "format_unavailable"
	}
	for _, item := range s.options {
		if item.ID != id {
			continue
		}
		if directory != "" {
			var code string
			directory, code = selectedDirectory(directory)
			if code != "" {
				return nil, code
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.inspectionExpires = time.Time{}
		s.counter++
		s.cancel = cancel
		s.current = job{ID: strconv.FormatUint(s.counter, 10), State: "downloading"}
		snapshot := s.current
		s.workers.Add(1)
		go func(address, cookies, destination string) {
			defer s.workers.Done()
			defer cancel()
			path, code := s.download(ctx, address, cookies, item, destination)
			s.mu.Lock()
			defer s.mu.Unlock()
			s.clearCookies()
			if ctx.Err() != nil && path == "" {
				s.current.State = "cancelled"
			} else if code != "" {
				s.current.State, s.current.Error = "failed", code
			} else {
				s.current.State, s.current.Path, s.current.Percent = "done", path, 100
			}
			s.cancel = nil
		}(s.videoURL, s.cookies, directory)
		return &snapshot, ""
	}
	return nil, "format_unavailable"
}

func selectedDirectory(path string) (string, string) {
	if !filepath.IsAbs(path) {
		return "", "directory_unavailable"
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", "directory_unavailable"
	}
	return path, ""
}

func (s *service) download(ctx context.Context, address, cookies string, item option, directory string) (string, string) {
	if directory != "" {
		return s.downloadTo(ctx, address, cookies, item, directory)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "write_failed"
	}
	destination := downloadsDirectory(home)
	if err := os.MkdirAll(destination, 0755); err != nil {
		return "", directoryWriteError(ctx, err)
	}
	return s.downloadTo(ctx, address, cookies, item, destination)
}

func (s *service) downloadTo(ctx context.Context, address, cookies string, item option, destination string) (string, string) {
	temp, err := os.MkdirTemp(destination, ".yt-dlp-")
	if err != nil {
		return "", directoryWriteError(ctx, err)
	}
	defer os.RemoveAll(temp)
	args := []string{"--no-simulate", "--windows-filenames", "--newline", "--progress", "--progress-delta", "0.5", "--concurrent-fragments", "4",
		"--progress-template", "download:download:%(progress._percent_str)s", "--progress-template", "postprocess:processing:",
		"--print", "after_move:result:%(filepath)j", "--no-quiet", "-f", item.selector,
		"-o", filepath.Join(strings.ReplaceAll(temp, "%", "%%"), "%(title).120B [%(id)s].%(ext)s")}
	switch item.Format {
	case "mp4":
		args = append(args, "--merge-output-format", "mp4")
	case "mp3":
		args = append(args, "-x", "--audio-format", "mp3", "--audio-quality", strings.TrimPrefix(item.ID, "mp3-")+"K")
	}
	cmd := downloader(ctx, cookies, append(args, "--", address)...)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	reader, err := cmd.StdoutPipe()
	if err != nil {
		return "", "download_failed"
	}
	if err := cmd.Start(); err != nil {
		return "", classify(ctx, err, "", cookies != "")
	}
	var result string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		prefix, value, _ := strings.Cut(scanner.Text(), ":")
		switch prefix {
		case "result":
			_ = json.Unmarshal([]byte(value), &result)
		case "processing", "download":
			s.mu.Lock()
			if prefix == "processing" {
				s.current.State = "processing"
			} else if progress, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "%"), 64); err == nil {
				s.current.State = "downloading"
				s.current.Percent = max(0, min(99, int(progress)))
			}
			s.mu.Unlock()
		}
	}
	if scanner.Err() != nil {
		_ = cmd.Cancel()
	}
	if err := cmd.Wait(); err != nil {
		code := classify(ctx, err, diagnostics.String(), cookies != "")
		if delay := retryDelay(code, diagnostics.String(), time.Now()); delay > 0 {
			s.mu.Lock()
			s.current.RetryAfter = delay
			s.mu.Unlock()
		}
		return "", code
	}
	if ctx.Err() != nil {
		return "", "cancelled"
	}
	if filepath.Clean(filepath.Dir(result)) != filepath.Clean(temp) || filepath.Ext(result) != "."+item.Format {
		return "", "download_failed"
	}
	if info, err := os.Lstat(result); err != nil || !info.Mode().IsRegular() {
		return "", "download_failed"
	}
	return publishFile(ctx, result, destination)
}

func directoryWriteError(ctx context.Context, err error) string {
	if os.IsNotExist(err) {
		return "directory_unavailable"
	}
	code := classify(ctx, err, "", false)
	if code == "download_failed" {
		return "write_failed"
	}
	return code
}

func retryDelay(code, diagnostics string, now time.Time) int {
	if code != "rate_limited" {
		return 0
	}
	for _, line := range strings.Split(diagnostics, "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found || !strings.EqualFold(name, "Retry-After") {
			continue
		}
		value = strings.TrimSpace(value)
		if seconds, err := strconv.Atoi(value); err == nil {
			return max(0, seconds)
		}
		if until, err := http.ParseTime(value); err == nil {
			return max(0, int((until.Sub(now)+time.Second-1)/time.Second))
		}
	}
	return 0
}

func publishFile(ctx context.Context, source, destination string) (string, string) {
	extension := filepath.Ext(source)
	name := strings.TrimSuffix(filepath.Base(source), extension)
	for index := 0; index < 1000; index++ {
		filename := name + extension
		if index != 0 {
			filename = fmt.Sprintf("%s (%d)%s", name, index, extension)
		}
		target := filepath.Join(destination, filename)
		err := commitFile(source, target)
		if err == nil {
			return target, ""
		}
		if !os.IsExist(err) {
			code := classify(ctx, err, "", false)
			if code == "download_failed" {
				code = "write_failed"
			}
			return "", code
		}
	}
	return "", "write_failed"
}

func classify(ctx context.Context, err error, diagnostics string, cookies bool) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "timeout"
	}
	if ctx.Err() == context.Canceled {
		return "cancelled"
	}
	if os.IsPermission(err) {
		return "write_failed"
	}
	if _, ok := err.(*exec.Error); ok {
		return "missing_tool"
	}
	message := strings.ToLower(diagnostics + " " + err.Error())
	switch {
	case strings.Contains(message, "http error 429"):
		return "rate_limited"
	case strings.Contains(message, "cookies are no longer valid"), strings.Contains(message, "cookies have expired"):
		return "cookies_expired"
	case strings.Contains(message, "private video") && cookies:
		return "access_denied"
	case strings.Contains(message, "sign in"), strings.Contains(message, "login required"), strings.Contains(message, "private video"):
		if cookies {
			return "access_denied"
		}
		return "login_required"
	case strings.Contains(message, "http error 403"):
		return "http_403"
	case strings.Contains(message, "requested format is not available"):
		return "format_unavailable"
	case strings.Contains(message, "video unavailable"), strings.Contains(message, "has been removed"):
		return "video_unavailable"
	case strings.Contains(message, "ffmpeg not found"), strings.Contains(message, "ffprobe not found"):
		return "missing_tool"
	case strings.Contains(message, "no space left"), strings.Contains(message, "not enough space"), strings.Contains(message, "disk full"):
		return "disk_full"
	case strings.Contains(message, "permission denied"), strings.Contains(message, "access is denied"):
		return "write_failed"
	case strings.Contains(message, "timed out"):
		return "timeout"
	case strings.Contains(message, "connection"), strings.Contains(message, "getaddrinfo"), strings.Contains(message, "network"):
		return "network_error"
	case strings.Contains(message, "invalid data found when processing input"), strings.Contains(message, "moov atom not found"):
		return "media_invalid"
	case strings.Contains(message, "unknown encoder"), strings.Contains(message, "encoder not found"):
		return "encoder_unavailable"
	case strings.Contains(message, "could not write header"), strings.Contains(message, "error initializing output stream"):
		return "ffmpeg_mux_failed"
	case strings.Contains(message, "error: postprocessing:"), strings.Contains(message, "ffmpeg exited with code"), strings.Contains(message, "ffmpeg exited with non-zero code"), strings.Contains(message, "error running ffmpeg"):
		return "ffmpeg_failed"
	default:
		return "download_failed"
	}
}
