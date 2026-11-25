#!/bin/sh
set -eu

host_name=com.ytdlp.downloader
host_directory="$HOME/Library/Application Support/YtDlpChromeHost"
manifests="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts"
manifest="$manifests/$host_name.json"
host="$host_directory/host"
source_directory=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)

if [ "${1:-}" = "--uninstall" ]; then
    rm -f "$manifest" "$host"
    for tool in yt-dlp ffmpeg ffprobe deno; do
        rm -f "$host_directory/tools/$tool"
    done
    exit 0
fi

binary=${1:-}
tools=${2:-}
if [ -z "$binary" ] && [ -f "$source_directory/host" ]; then
    binary="$source_directory/host"
fi
if [ -z "$tools" ] && [ -d "$source_directory/tools" ]; then
    tools="$source_directory/tools"
fi
if [ -n "$tools" ]; then
    for tool in yt-dlp ffmpeg ffprobe deno; do
        if [ ! -f "$tools/$tool" ]; then
            printf 'Missing executable in tools directory: %s\n' "$tool" >&2
            exit 1
        fi
    done
fi

mkdir -p "$host_directory" "$manifests"
if [ -n "$binary" ]; then
    if ! cmp -s "$binary" "$host" || [ ! -x "$host" ]; then
        install -m 755 "$binary" "$host"
    fi
else
    (cd "$source_directory" && go build -trimpath -o "$host" .)
fi
if [ -n "$tools" ]; then
    mkdir -p "$host_directory/tools"
    for tool in yt-dlp ffmpeg ffprobe deno; do
        target="$host_directory/tools/$tool"
        if ! cmp -s "$tools/$tool" "$target" || [ ! -x "$target" ]; then
            install -m 755 "$tools/$tool" "$target"
        fi
    done
fi

tmp_manifest=$(mktemp "$manifests/.ytdlp-manifest.XXXXXX")
trap 'rm -f "$tmp_manifest"' EXIT
/usr/bin/osascript -l JavaScript -s h -e '
function run(args) {
    return JSON.stringify({
        name: args[0],
        description: "chrome yt-dlp extension local helper",
        path: args[1],
        type: "stdio",
        allowed_origins: ["chrome-extension://ifjcfdebfppkjagopedodglfheiljeap/"]
    });
}' "$host_name" "$host" > "$tmp_manifest"
test -s "$tmp_manifest"
mv -f "$tmp_manifest" "$manifest"
printf 'Installed %s\n' "$host_name"
