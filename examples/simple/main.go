package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	dloader "github.com/efeligne/video-downloader/downloader"
)

var errMissingYTDLPPath = errors.New("set YTDLP_PATH to yt-dlp executable path")

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	ytDLPPath := os.Getenv("YTDLP_PATH")
	if ytDLPPath == "" {
		return errMissingYTDLPPath
	}

	downloaderClient, err := dloader.New(ytDLPPath)
	if err != nil {
		return fmt.Errorf("create downloader: %w", err)
	}
	defer downloaderClient.Close()

	opts := dloader.Options{
		OutputTemplate: "%(title)s.%(ext)s",
		Format:         "bestvideo+bestaudio/best",
		Progress: func(update dloader.ProgressUpdate) {
			fmt.Printf(
				"\rProgress: %6.2f%% | ETA %s | %s",
				update.Percent,
				update.ETA,
				update.Speed,
			)
		},
	}

	res, err := downloaderClient.Download(ctx, "https://disk.yandex.ru/i/VU3FlHnD_gcwzw", opts)
	if err != nil {
		return fmt.Errorf("download failed: %w\nstderr:\n%s", err, res.Stderr)
	}

	fmt.Print("\n")
	log.Printf("download finished\nstdout:\n%s", res.Stdout)

	return nil
}
