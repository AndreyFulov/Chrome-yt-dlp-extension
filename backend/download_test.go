package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const fixtureVariable = "YTDLP_HOST_TEST_FIXTURE"

func TestMain(m *testing.M) {
	if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == "yt-dlp" {
		switch os.Getenv(fixtureVariable) {
		case "download", "cancel":
			os.Exit(fixtureDownloader())
		}
	}
	os.Exit(m.Run())
}

func fixtureDownloader() int {
	args := os.Args[1:]
	if slices.Contains(args, "--dump-single-json") {
		fmt.Println(`{"title":"Тестовый ролик","formats":[{"format_id":"140","ext":"m4a","vcodec":"none","acodec":"mp4a.40.2"},{"format_id":"137","ext":"mp4","vcodec":"avc1","acodec":"none","height":1080}]}`)
		return 0
	}
	template := slices.Index(args, "-o")
	selector := slices.Index(args, "-f")
	fragments := slices.Index(args, "--concurrent-fragments")
	if template < 0 || selector < 0 || fragments < 0 || args[fragments+1] != "4" || !slices.Contains(args, "--ignore-config") {
		return 1
	}
	extension := ""
	switch args[selector+1] {
	case "137+140":
		if slices.Contains(args, "--merge-output-format") {
			extension = ".mp4"
		}
	case "140":
		extension = ".m4a"
	case "bestaudio/best":
		if slices.Contains(args, "320K") && slices.Contains(args, "mp3") {
			extension = ".mp3"
		}
	}
	if extension == "" {
		return 1
	}
	path := filepath.Join(filepath.Dir(args[template+1]), "Тестовый ролик"+extension)
	fmt.Println("download:35.0%")
	if os.Getenv(fixtureVariable) == "cancel" {
		time.Sleep(30 * time.Second)
		return 1
	}
	fmt.Println("processing:")
	if os.WriteFile(path, []byte("fixture"), 0600) != nil {
		return 1
	}
	result, _ := json.Marshal(path)
	fmt.Printf("result:%s\n", result)
	return 0
}

func TestDownloadPipeline(t *testing.T) {
	fixtureDir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, name), data, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fixtureDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fixtureVariable, "download")
	s := &service{}
	address := "https://www.youtube.com/watch?v=BaW_jenozKc"
	title, options, code, _ := s.inspect(address)
	if code != "" || title != "Тестовый ролик" || len(options) != 5 {
		t.Fatalf("inspect: %q %v %s", title, options, code)
	}
	toolPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	if cachedTitle, _, cachedError, _ := s.inspect(address); cachedError != "" || cachedTitle != title {
		t.Fatalf("expected cached formats, got %q %s", cachedTitle, cachedError)
	}
	t.Setenv("PATH", toolPath)
	destination := t.TempDir()
	for _, index := range []int{0, 1, 4} {
		item := options[index]
		path, code := s.downloadTo(context.Background(), address, "", item, destination)
		if code != "" || filepath.Ext(path) != "."+item.Format || s.current.State != "processing" || s.current.Percent != 35 {
			t.Fatalf("%s: %q %q %+v", item.Format, path, code, s.current)
		}
		if content, err := os.ReadFile(path); err != nil || string(content) != "fixture" {
			t.Fatalf("%q %v", content, err)
		}
	}
	t.Setenv(fixtureVariable, "cancel")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, code := s.downloadTo(ctx, address, "", options[0], destination); code != "timeout" {
		t.Fatalf("expected timeout, got %q", code)
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 3 {
		t.Fatalf("unexpected files: %v %v", entries, err)
	}
	t.Setenv(fixtureVariable, "download")
	if _, code := s.start(address, options[0].ID, filepath.Join(destination, "missing")); code != "directory_unavailable" {
		t.Fatalf("expected unavailable folder, got %q", code)
	}
	custom := filepath.Join(destination, "Моя папка")
	if err := os.Mkdir(custom, 0700); err != nil {
		t.Fatal(err)
	}
	if _, code := s.start(address, options[0].ID, custom); code != "" {
		t.Fatal(code)
	}
	s.workers.Wait()
	if s.current.State != "done" || filepath.Dir(s.current.Path) != custom {
		t.Fatalf("job did not save in chosen directory: %+v", s.current)
	}
}
