package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gostreampuller/config"
)

// Downloader provides functionality to download and stream videos/audio.
type Downloader struct {
	cfg *config.Config
}

// NewDownloader creates a new Downloader instance.
func NewDownloader(cfg *config.Config) *Downloader {
	return &Downloader{
		cfg: cfg,
	}
}

// VideoInfo represents a subset of yt-dlp's info.json output.
type VideoInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	OriginalURL string `json:"original_url"`
	Ext         string `json:"ext"`
	Duration    int    `json:"duration"` // in seconds
	Uploader    string `json:"uploader"`
	UploadDate  string `json:"upload_date"` // YYYYMMDD
	Thumbnail   string `json:"thumbnail"`   // URL to thumbnail
}

// DownloadVideoToFile downloads a video from the given URL to a file.
// It returns the path to the downloaded file and its metadata.
func (d *Downloader) DownloadVideoToFile(ctx context.Context, url string, format string, resolution string, codec string) (string, *VideoInfo, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "720"
	}
	if codec == "" {
		codec = "avc1"
	}

	// Step 1: Get video info using --dump-json
	infoArgs := []string{
		"--dump-json",
		"--no-playlist",
		"--restrict-filenames", // To get a clean title for the filename
		url,
	}
	infoCmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, infoArgs...) // Use CommandContext
	slog.Debug(fmt.Sprintf("Executing yt-dlp for video info: %s %s", d.cfg.YTDLPPath, strings.Join(infoArgs, " ")))

	var infoStdout, infoStderr bytes.Buffer
	infoCmd.Stdout = &infoStdout
	infoCmd.Stderr = &infoStderr

	err := infoCmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp info dump failed: %v\nStdout: %s\nStderr: %s", err, infoStdout.String(), infoStderr.String()))
		return "", nil, fmt.Errorf("yt-dlp info dump failed: %w, stderr: %s", err, infoStderr.String())
	}

	var videoInfo VideoInfo
	if err := json.Unmarshal(infoStdout.Bytes(), &videoInfo); err != nil {
		return "", nil, fmt.Errorf("failed to parse yt-dlp info json: %w", err)
	}

	// Generate a unique filename using timestamp and original extension
	uniqueFilename := fmt.Sprintf("%d-%s.%s", time.Now().UnixNano(), videoInfo.ID, format)
	finalFilePath := filepath.Join(d.cfg.DownloadDir, uniqueFilename)

	// Step 2: Download the video to the specific filename
	downloadArgs := []string{
		"--format", fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec),
		"--output", finalFilePath,
		"--no-progress",
		"--no-playlist",          // Assume single video download
		"--recode-video", format, // Instruct yt-dlp to convert to the desired format
		url,
	}

	downloadCmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, downloadArgs...) // Use CommandContext
	slog.Debug(fmt.Sprintf("Executing yt-dlp for video download: %s %s", d.cfg.YTDLPPath, strings.Join(downloadArgs, " ")))

	var downloadStdout, downloadStderr bytes.Buffer
	downloadCmd.Stdout = &downloadStdout
	downloadCmd.Stderr = &downloadStderr

	err = downloadCmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp video download failed: %v\nStdout: %s\nStderr: %s", err, downloadStdout.String(), downloadStderr.String()))
		return "", nil, fmt.Errorf("yt-dlp video download failed: %w, stderr: %s", err, downloadStderr.String())
	}

	// Verify the file exists
	if _, err := os.Stat(finalFilePath); err != nil {
		return "", nil, fmt.Errorf("downloaded video file not found at %s: %w", finalFilePath, err)
	}

	slog.Info(fmt.Sprintf("Video downloaded to: %s", finalFilePath))
	return finalFilePath, &videoInfo, nil
}

// DownloadAudioToFile downloads audio from the given URL to a file.
// It returns the path to the downloaded file and its metadata.
func (d *Downloader) DownloadAudioToFile(ctx context.Context, url string, outputFormat string, codec string, bitrate string) (string, *VideoInfo, error) {
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	if codec == "" {
		codec = "libmp3lame"
	}
	if bitrate == "" {
		bitrate = "128k"
	}

	// Step 1: Get video info using --dump-json
	infoArgs := []string{
		"--dump-json",
		"--no-playlist",
		"--restrict-filenames", // To get a clean title for the filename
		url,
	}
	infoCmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, infoArgs...) // Use CommandContext
	slog.Debug(fmt.Sprintf("Executing yt-dlp for audio info: %s %s", d.cfg.YTDLPPath, strings.Join(infoArgs, " ")))

	var infoStdout, infoStderr bytes.Buffer
	infoCmd.Stdout = &infoStdout
	infoCmd.Stderr = &infoStderr

	err := infoCmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp info dump failed: %v\nStdout: %s\nStderr: %s", err, infoStdout.String(), infoStderr.String()))
		return "", nil, fmt.Errorf("yt-dlp info dump failed: %w, stderr: %s", err, infoStderr.String())
	}

	var videoInfo VideoInfo // Re-use VideoInfo struct for audio metadata
	if err := json.Unmarshal(infoStdout.Bytes(), &videoInfo); err != nil {
		return "", nil, fmt.Errorf("failed to parse yt-dlp info json: %w", err)
	}

	// Generate a unique filename using timestamp and desired output format
	uniqueFilename := fmt.Sprintf("%d-%s.%s", time.Now().UnixNano(), videoInfo.ID, outputFormat)
	finalFilePath := filepath.Join(d.cfg.DownloadDir, uniqueFilename)

	// Step 2: Download the audio to the specific filename
	downloadArgs := []string{
		"--extract-audio",
		"--audio-format", outputFormat,
		"--audio-quality", bitrate, // Corresponds to bitrate for audio quality
		"--postprocessor-args", fmt.Sprintf("ffmpeg:-acodec %s", codec), // Specify audio codec for ffmpeg
		"--output", finalFilePath,
		"--no-progress",
		"--no-playlist",
		url,
	}

	downloadCmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, downloadArgs...) // Use CommandContext
	slog.Debug(fmt.Sprintf("Executing yt-dlp for audio download: %s %s", d.cfg.YTDLPPath, strings.Join(downloadArgs, " ")))

	var downloadStdout, downloadStderr bytes.Buffer
	downloadCmd.Stdout = &downloadStdout
	downloadCmd.Stderr = &downloadStderr

	err = downloadCmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp audio fetch failed: %v\nStdout: %s\nStderr: %s", err, downloadStdout.String(), downloadStderr.String()))
		return "", nil, fmt.Errorf("yt-dlp audio fetch failed: %w, stderr: %s", err, downloadStderr.String())
	}

	// Verify the file exists
	if _, err := os.Stat(finalFilePath); err != nil {
		return "", nil, fmt.Errorf("downloaded audio file not found at %s: %w", finalFilePath, err)
	}

	slog.Info(fmt.Sprintf("Audio downloaded to: %s", finalFilePath))
	return finalFilePath, &videoInfo, nil
}

// StreamVideo streams video from the given URL.
func (d *Downloader) StreamVideo(ctx context.Context, url string, format string, resolution string, codec string) (io.ReadCloser, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "720" // Default to 720p for streaming if not specified
	}
	if codec == "" {
		codec = "avc1"
	}

	// Use --downloader ffmpeg to let yt-dlp handle the piping and conversion internally.
	// This is more reliable than external piping.
	// Format string: bestvideo[height<=RES]+bestaudio/best --recode-video FORMAT
	// This tells yt-dlp to select the best video/audio and then recode it to the desired format.
	ytDLPArgs := []string{
		"--downloader", "ffmpeg",
		"--downloader-args", fmt.Sprintf("ffmpeg_i:-c:v %s", codec), // Pass video codec to ffmpeg
		"--format", fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best", resolution),
		"--recode-video", format,
		"-o", "-", // Output to stdout
		url,
	}
	cmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, ytDLPArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for video stream: %s %s", d.cfg.YTDLPPath, strings.Join(ytDLPArgs, " ")))

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for yt-dlp: %w", err)
	}
	cmd.Stderr = os.Stderr // Direct yt-dlp errors to stderr for debugging

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp command for video stream: %w", err)
	}

	return &commandReadCloser{
		ReadCloser: stdoutPipe,
		cmd:        cmd,
	}, nil
}

// StreamAudio streams audio from the given URL.
func (d *Downloader) StreamAudio(ctx context.Context, url string, outputFormat string, codec string, bitrate string) (io.ReadCloser, error) {
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	if codec == "" {
		codec = "libmp3lame"
	}
	if bitrate == "" {
		bitrate = "128k"
	}

	// Use --downloader ffmpeg to let yt-dlp handle the piping and conversion internally.
	ytDLPArgs := []string{
		"--extract-audio",
		"--audio-format", outputFormat,
		"--audio-quality", bitrate, // Corresponds to bitrate for audio quality
		"--postprocessor-args", fmt.Sprintf("ffmpeg:-acodec %s", codec), // Specify audio codec for ffmpeg
		"--downloader", "ffmpeg",
		"-o", "-", // Output to stdout
		url,
	}
	cmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, ytDLPArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for audio stream: %s %s", d.cfg.YTDLPPath, strings.Join(ytDLPArgs, " ")))

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for yt-dlp: %w", err)
	}
	cmd.Stderr = os.Stderr // Direct yt-dlp errors to stderr for debugging

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp command for audio stream: %w", err)
	}

	return &commandReadCloser{
		ReadCloser: stdoutPipe,
		cmd:        cmd,
	}, nil
}

// commandReadCloser wraps an io.ReadCloser and an exec.Cmd,
// ensuring the command is waited upon when the reader is closed.
type commandReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
	// Add a mutex to protect access to cmd.Wait() if Close() could be called concurrently
	// or if cmd.Wait() could be called multiple times.
	// For this use case, it's typically called once.
	waitOnce sync.Once
	waitErr  error
}

// Close closes the underlying reader and waits for the command to exit.
func (crc *commandReadCloser) Close() error {
	// Close the pipe first
	pipeCloseErr := crc.ReadCloser.Close()

	// Wait for the command to exit, ensuring it's only called once
	crc.waitOnce.Do(func() {
		crc.waitErr = crc.cmd.Wait()
	})

	if pipeCloseErr != nil {
		return fmt.Errorf("error closing pipe: %w; command wait error: %v", pipeCloseErr, crc.waitErr)
	}
	if crc.waitErr != nil {
		return fmt.Errorf("command exited with error: %w", crc.waitErr)
	}
	return nil
}

// pipedCommandReadCloser is no longer needed as we are now using --downloader ffmpeg
// and only one command is executed.
// This type is kept for reference if a multi-command pipe is ever needed again.
type pipedCommandReadCloser struct {
	io.ReadCloser
	primaryCmd   *exec.Cmd // The command whose stdout is being read (e.g., ffmpeg)
	secondaryCmd *exec.Cmd // The command whose stdout is piped to primaryCmd's stdin (e.g., yt-dlp)
	waitOnce     sync.Once
	waitErrs     []error
}

// Close closes the underlying reader and waits for both commands to exit.
func (pcrc *pipedCommandReadCloser) Close() error {
	var errs []error

	// Close the pipe from the primary command
	if err := pcrc.ReadCloser.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing primary pipe: %w", err))
	}

	// Wait for both commands to exit, ensuring it's only called once
	pcrc.waitOnce.Do(func() {
		// Wait for primary command
		if err := pcrc.primaryCmd.Wait(); err != nil {
			pcrc.waitErrs = append(pcrc.waitErrs, fmt.Errorf("primary command exited with error: %w", err))
		}

		// Wait for secondary command
		if err := pcrc.secondaryCmd.Wait(); err != nil {
			pcrc.waitErrs = append(pcrc.waitErrs, fmt.Errorf("secondary command exited with error: %w", err))
		}
	})

	errs = append(errs, pcrc.waitErrs...)

	if len(errs) > 0 {
		// Combine errors if multiple occurred
		var combinedError strings.Builder
		for i, e := range errs {
			if i > 0 {
				combinedError.WriteString("; ")
			}
			combinedError.WriteString(e.Error())
		}
		return errors.New(combinedError.String())
	}
	return nil
}
