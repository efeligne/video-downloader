// Package downloader provides a small API wrapper around an external yt-dlp binary.
package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

var (
	errEmptyBinaryPath     = errors.New("binary path is empty")
	errURLRequired         = errors.New("url is required")
	errBinaryNotConfigured = errors.New("yt-dlp binary is not configured")
)

// Downloader wraps an external yt-dlp binary and exposes a small API for running it.
type Downloader struct {
	binPath string
}

// New creates a downloader for the provided yt-dlp binary path.
func New(binaryPath string) (*Downloader, error) {
	if binaryPath == "" {
		return nil, errEmptyBinaryPath
	}

	if _, err := os.Stat(binaryPath); err != nil {
		return nil, fmt.Errorf("stat yt-dlp binary: %w", err)
	}

	return &Downloader{binPath: binaryPath}, nil
}

// NewWithBinary is kept for backward compatibility; it is equivalent to New.
func NewWithBinary(path string) (*Downloader, error) {
	return New(path)
}

// Close is kept for compatibility; no cleanup is needed for an external binary.
func (d *Downloader) Close() error {
	_ = d // receiver retained for API compatibility
	return nil
}

// Options control a single yt-dlp invocation.
type Options struct {
	OutputTemplate string            // e.g. "%(title)s.%(ext)s"
	Format         string            // e.g. "bestvideo+bestaudio/best"
	Proxy          string            // e.g. "socks5://127.0.0.1:9050"
	CookiesFile    string            // path to a Netscape cookies.txt file
	Headers        map[string]string // extra headers to send
	ExtraArgs      []string          // raw args passed to yt-dlp before the URL
	WorkDir        string            // optional working directory for the command
	Stdout         io.Writer         // tee stdout to this writer
	Stderr         io.Writer         // tee stderr to this writer
	Progress       func(ProgressUpdate)
}

// Result carries captured output from a yt-dlp run.
type Result struct {
	Stdout []byte
	Stderr []byte
}

// ProgressUpdate represents a single update emitted by yt-dlp during download.
type ProgressUpdate struct {
	Percent float64
	ETA     string
	Speed   string
	Raw     string
}

const (
	progressTemplate   = "%(progress._percent_str)s|%(progress._eta_str)s|%(progress._speed_str)s"
	progressPartsCount = 3
	floatBitSize       = 64
)

// Download runs yt-dlp with the provided options. Output is captured and also
// optionally streamed to the provided writers in Options.
func (d *Downloader) Download(ctx context.Context, url string, opts Options) (*Result, error) {
	if url == "" {
		return nil, errURLRequired
	}

	if d.binPath == "" {
		return nil, errBinaryNotConfigured
	}

	args := buildArgs(url, opts)

	//nolint:gosec // command and args are controlled by caller for yt-dlp
	cmd := exec.CommandContext(ctx, d.binPath, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	stdoutWriter, stdoutBuf := progressWriter(opts.Progress, opts.Stdout)
	stderrWriter, stderrBuf := teeWriter(opts.Stderr)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	err := cmd.Run()

	result := &Result{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
	}

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, fmt.Errorf("context done: %w", ctxErr)
		}

		return result, fmt.Errorf("yt-dlp failed: %w\n%s", err, bytes.TrimSpace(result.Stderr))
	}

	return result, nil
}

func buildArgs(url string, opts Options) []string {
	args := []string{"--newline"}

	if opts.Progress != nil {
		args = append(args, "--progress-template", progressTemplate)
	} else {
		args = append(args, "--no-progress")
	}

	if opts.OutputTemplate != "" {
		args = append(args, "-o", opts.OutputTemplate)
	}

	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	}

	if opts.Proxy != "" {
		args = append(args, "--proxy", opts.Proxy)
	}

	if opts.CookiesFile != "" {
		args = append(args, "--cookies", opts.CookiesFile)
	}

	if len(opts.Headers) > 0 {
		keys := make([]string, 0, len(opts.Headers))
		for headerKey := range opts.Headers {
			keys = append(keys, headerKey)
		}

		sort.Strings(keys)

		for _, headerKey := range keys {
			v := strings.TrimSpace(opts.Headers[headerKey])
			if v == "" {
				continue
			}

			args = append(args, "--add-header", fmt.Sprintf("%s:%s", headerKey, v))
		}
	}

	args = append(args, opts.ExtraArgs...)
	args = append(args, url)

	return args
}

func teeWriter(extra io.Writer) (io.Writer, *bytes.Buffer) {
	var buf bytes.Buffer

	if extra == nil {
		return &buf, &buf
	}

	return io.MultiWriter(extra, &buf), &buf
}

func progressWriter(callback func(ProgressUpdate), extra io.Writer) (io.Writer, *bytes.Buffer) {
	if callback == nil {
		return teeWriter(extra)
	}

	var (
		buf     bytes.Buffer
		lineBuf strings.Builder
	)

	writer := func(chunk []byte) (int, error) {
		if extra != nil {
			if _, err := extra.Write(chunk); err != nil {
				return 0, fmt.Errorf("write passthrough: %w", err)
			}
		}

		if _, err := buf.Write(chunk); err != nil {
			return 0, fmt.Errorf("buffer output: %w", err)
		}

		for _, b := range chunk {
			lineBuf.WriteByte(b)

			if b == '\n' {
				line := strings.TrimSpace(lineBuf.String())
				lineBuf.Reset()

				if upd, ok := parseProgress(line); ok {
					callback(upd)
				}
			}
		}

		return len(chunk), nil
	}

	return writerFunc(writer), &buf
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(data []byte) (int, error) {
	return f(data)
}

func parseProgress(line string) (ProgressUpdate, bool) {
	parts := strings.Split(line, "|")
	if len(parts) != progressPartsCount {
		return ProgressUpdate{Raw: line}, false
	}

	rawPercent := strings.TrimSpace(strings.TrimSuffix(parts[0], "%"))
	eta := strings.TrimSpace(parts[1])
	speed := strings.TrimSpace(parts[2])

	if rawPercent == "" || strings.EqualFold(rawPercent, "N/A") {
		return ProgressUpdate{Raw: line}, false
	}

	percent, err := strconv.ParseFloat(rawPercent, floatBitSize)
	if err != nil {
		return ProgressUpdate{Raw: line}, false
	}

	return ProgressUpdate{
		Percent: percent,
		ETA:     eta,
		Speed:   speed,
		Raw:     line,
	}, true
}
