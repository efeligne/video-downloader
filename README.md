# video-downloader

Small Go library that runs an external `yt-dlp` binary to download videos. Provide the path to the installed binary (for example, `/usr/local/bin/yt-dlp`).

## Usage

```go
package main

import (
 "context"
 "fmt"
 "log"
 "os"

 "github.com/efeligne/video-downloader/downloader"
)

func main() {
 ytDLPPath := os.Getenv("YTDLP_PATH") // set the path directly or via env var
 if ytDLPPath == "" {
  log.Fatal("set YTDLP_PATH to yt-dlp executable")
 }

 dl, err := downloader.New(ytDLPPath)
 if err != nil {
  log.Fatal(err)
 }
 defer dl.Close()

 opts := downloader.Options{
  OutputTemplate: "%(title)s.%(ext)s",
  Format:         "bestvideo+bestaudio/best",
  Progress: func(p downloader.ProgressUpdate) {
   fmt.Printf("\rProgress: %6.2f%% | ETA %s | %s", p.Percent, p.ETA, p.Speed)
  },
 }

 res, err := dl.Download(context.Background(), "https://youtu.be/dQw4w9WgXcQ", opts)
 if err != nil {
  log.Fatalf("download failed: %v\nstderr: %s", err, res.Stderr)
 }

 fmt.Printf("\nyt-dlp stdout:\n%s\n", res.Stdout)
}
```

## Ready-to-run example

Run from the repo root:

```bash
go run ./examples/simple
```

## Options

- `OutputTemplate` - output filename template (`%(title)s.%(ext)s`).
- `Format` - format string (`best`, `bestvideo+bestaudio/best`, etc.).
- `Proxy` - proxy in a format understood by `yt-dlp`.
- `CookiesFile` - path to `cookies.txt`.
- `Headers` - extra headers (`"Authorization": "Bearer ..."`, etc.).
- `ExtraArgs` - arbitrary arguments passed before the URL.
- `WorkDir`, `Stdout`, `Stderr` - working directory and where to stream output.
- `Progress` - callback that receives progress updates (percent, ETA, speed).
- Pass the `yt-dlp` path to the constructor. Install `yt-dlp` yourself (`brew install yt-dlp`, `pip install yt-dlp`, deb/rpm, etc.).
