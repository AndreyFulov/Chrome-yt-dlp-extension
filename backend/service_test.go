package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeMessages(t *testing.T) {
	var input, output bytes.Buffer
	requests := []string{
		`{"id":"hello","protocolVersion":1,"command":"hello"}`,
		`{"id":"bad-version","protocolVersion":2,"command":"hello"}`,
		`{"id":"watch","protocolVersion":1,"command":"inspect","url":"https://youtube.com.evil.test/watch?v=BaW_jenozKc"}`,
		`{"id":"job","protocolVersion":1,"command":"status","jobId":"missing"}`,
		`{"id":"start","protocolVersion":1,"command":"start","optionId":"--exec"}`,
		`{`,
	}
	for _, payload := range requests {
		if err := binary.Write(&input, binary.NativeEndian, uint32(len(payload))); err != nil {
			t.Fatal(err)
		}
		input.WriteString(payload)
	}
	versions := func() map[string]tool { return map[string]tool{"yt-dlp": {Status: "available", Version: "fixture"}} }
	if err := serve(&input, &output, versions); err != nil {
		t.Fatal(err)
	}
	codes := []string{"", "protocol_mismatch", "invalid_url", "job_not_found", "format_unavailable", "invalid_request"}
	for _, code := range codes {
		var size uint32
		if err := binary.Read(&output, binary.NativeEndian, &size); err != nil {
			t.Fatal(err)
		}
		var got response
		if err := json.Unmarshal(output.Next(int(size)), &got); err != nil {
			t.Fatal(err)
		}
		if got.Error != code {
			t.Fatalf("expected %q, got %+v", code, got)
		}
		if code == "" && (got.ID != "hello" || got.Tools["yt-dlp"].Version != "fixture") {
			t.Fatalf("invalid hello: %+v", got)
		}
	}
	if output.Len() != 0 {
		t.Fatal("unexpected trailing data")
	}
}

func TestClassifyDownloaderErrors(t *testing.T) {
	cases := []struct {
		diagnostic string
		cookies    bool
		code       string
	}{
		{"Sign in to confirm you’re not a bot", false, "login_required"},
		{"The provided YouTube account cookies are no longer valid", true, "cookies_expired"},
		{"Private video", true, "access_denied"},
		{"HTTP Error 403: Forbidden", true, "http_403"},
		{"HTTP Error 429: Too Many Requests", false, "rate_limited"},
		{"[ffmpeg] No space left on device", false, "disk_full"},
		{"WARNING: ffmpeg is installed\nERROR: failed to download media", false, "download_failed"},
		{"ERROR: Postprocessing: Conversion failed", false, "ffmpeg_failed"},
		{"ERROR: Postprocessing: moov atom not found", false, "media_invalid"},
		{"ERROR: Postprocessing: Unknown encoder 'libmp3lame'", false, "encoder_unavailable"},
		{"ERROR: Postprocessing: Could not write header", false, "ffmpeg_mux_failed"},
		{"Requested format is not available", false, "format_unavailable"},
	}
	for _, item := range cases {
		if got := classify(context.Background(), errors.New("exit status 1"), item.diagnostic, item.cookies); got != item.code {
			t.Errorf("%q: got %q, want %q", item.diagnostic, got, item.code)
		}
	}
}

func TestRetryDelay(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	for _, header := range []string{"retry-after: 90", "Retry-After: Wed, 25 Nov 2026 12:01:30 GMT"} {
		if got := retryDelay("rate_limited", header, now); got != 90 {
			t.Errorf("%q: got %d seconds", header, got)
		}
	}
	if got := retryDelay("http_403", "Retry-After: 90", now); got != 0 {
		t.Errorf("unexpected retry delay %d", got)
	}
}

func TestSelectOptions(t *testing.T) {
	rows := []mediaFormat{
		{ID: "140", Ext: "m4a", Video: "none", Audio: "mp4a.40.2", Bitrate: 128},
		{ID: "18", Ext: "mp4", Video: "avc1.1", Audio: "mp4a.40.2", Height: 360},
		{ID: "137", Ext: "mp4", Video: "avc1.1", Audio: "none", Height: 1080},
		{ID: "998", Ext: "mp4", Video: "avc1.1", Audio: "none", Height: 1440, DRM: true},
		{ID: "999", Ext: "webm", Video: "vp9", Audio: "none", Height: 2160},
	}
	got := selectOptions(rows)
	if len(got) != 6 || got[0].selector != "137+140" || got[1].ID != "mp4-360" || got[2].Format != "m4a" || got[5].ID != "mp3-320" {
		t.Fatalf("unexpected options: %+v", got)
	}
	if got := canonicalURL("https://youtu.be/BaW_jenozKc?list=a"); got != "https://www.youtube.com/watch?v=BaW_jenozKc" {
		t.Fatal(got)
	}
}

func TestFilterCookies(t *testing.T) {
	keep := "#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t\tTEST\tfixture"
	discard := ".notyoutube.com\tTRUE\t/\tTRUE\t0\tTEST\tdiscard"
	for _, newline := range []string{"\n", "\r\n"} {
		file := strings.Join([]string{"# Netscape HTTP Cookie File", keep, discard}, newline)
		got, err := filterCookies(file)
		if err != nil || !strings.Contains(got, keep+cookieNewline) || strings.Contains(got, discard) {
			t.Fatalf("unexpected result %q, %v", got, err)
		}
	}
	if _, err := filterCookies("# Netscape HTTP Cookie File\n.youtube.com\tbroken"); err == nil {
		t.Fatal("accepted malformed row")
	}
}

func TestPublishPreservesExistingFile(t *testing.T) {
	destination := t.TempDir()
	source := filepath.Join(t.TempDir(), "Кириллица.mp4")
	existing := filepath.Join(destination, filepath.Base(source))
	if err := os.WriteFile(source, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	published, code := publishFile(context.Background(), source, destination)
	if code != "" || filepath.Base(published) != "Кириллица (1).mp4" {
		t.Fatalf("%s: %s", published, code)
	}
	for path, want := range map[string]string{existing: "old", published: "new"} {
		b, err := os.ReadFile(path)
		if err != nil || string(b) != want {
			t.Fatalf("%s: %s, %v", path, b, err)
		}
	}
}
