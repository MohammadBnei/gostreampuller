package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// GetDownloadDir returns the configured download directory.
func (d *Downloader) GetDownloadDir() string {
	return d.cfg.DownloadDir
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
	// Add fields for direct stream URL and file size
	DirectStreamURL string `json:"url"` // The actual direct URL of the stream
	FileSize        int64  `json:"filesize"`
	FormatID        string `json:"format_id"`
	FormatNote      string `json:"format_note"`
	VCodec          string `json:"vcodec"`
	ACodec          string `json:"acodec"`
	FPS             float64 `json:"fps"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	// Formats is a slice of available formats, used by GetStreamInfo
	Formats []VideoInfo `json:"formats"`
}

// GetVideoInfo fetches video metadata without downloading the file.
// This is for general info, not necessarily for direct streaming.
func (d *Downloader) GetVideoInfo(ctx context.Context, url string) (*VideoInfo, error) {
	infoArgs := []string{
		"--dump-json",
		"--no-playlist",
		"--restrict-filenames",
		url,
	}
	cmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, infoArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for video info: %s %s", d.cfg.YTDLPPath, strings.Join(infoArgs, " ")))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp info dump failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String()))
		return nil, fmt.Errorf("yt-dlp info dump failed: %w, stderr: %s", err, stderr.String())
	}

	var videoInfo VideoInfo
	if err := json.Unmarshal(stdout.Bytes(), &videoInfo); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp info json: %w", err)
	}
	return &videoInfo, nil
}

// GetStreamInfo fetches detailed stream information, including direct URLs.
// It tries to find a suitable video stream based on resolution and codec.
// This method is still useful for getting detailed format information, even if not directly proxying.
func (d *Downloader) GetStreamInfo(ctx context.Context, url string, resolution string, codec string) (*VideoInfo, error) {
	// Use --list-formats to get all available formats
	infoArgs := []string{
		"--dump-json",
		"--no-playlist",
		"--restrict-filenames",
		url,
	}
	cmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, infoArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for stream info: %s %s", d.cfg.YTDLPPath, strings.Join(infoArgs, " ")))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp stream info dump failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String()))
		return nil, fmt.Errorf("yt-dlp stream info dump failed: %w, stderr: %s", err, stderr.String())
	}

	var fullInfo VideoInfo // Use VideoInfo directly as it now contains Formats
	if err := json.Unmarshal(stdout.Bytes(), &fullInfo); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp full info json: %w", err)
	}

	// Default resolution if not provided
	targetHeight := 720 // Default to 720p
	if resolution != "" {
		if h, err := strconv.Atoi(resolution); err == nil {
			targetHeight = h
		}
	}

	// Default codec if not provided
	if codec == "" {
		codec = "avc1" // Default to H.264
	}

	var bestFormat *VideoInfo
	for i := range fullInfo.Formats {
		f := &fullInfo.Formats[i]
		// Prioritize formats with direct URLs and video streams
		if f.DirectStreamURL != "" && f.VCodec != "none" {
			// Try to match resolution and codec
			if f.Height == targetHeight && strings.Contains(f.VCodec, codec) {
				bestFormat = f
				break // Found a perfect match
			}
			// If no perfect match, try to find the closest resolution with the preferred codec
			// Preference: exact codec match, then closest resolution
			if strings.Contains(f.VCodec, codec) {
				if bestFormat == nil ||
					(f.Height <= targetHeight && f.Height > bestFormat.Height) || // Closer to target from below
					(bestFormat.Height > targetHeight && f.Height < bestFormat.Height) { // Closer to target from above
					bestFormat = f
				}
			}
		}
	}

	if bestFormat == nil {
		// Fallback: if no specific video format found, try to find the best overall video stream
		for i := range fullInfo.Formats {
			f := &fullInfo.Formats[i]
			if f.DirectStreamURL != "" && f.VCodec != "none" {
				if bestFormat == nil || f.FileSize > bestFormat.FileSize { // Simple heuristic: largest file size
					bestFormat = f
				}
			}
		}
	}

	if bestFormat == nil {
		return nil, fmt.Errorf("no suitable direct stream URL found for video: %s", url)
	}

	// Populate top-level video info from fullInfo
	bestFormat.ID = fullInfo.ID
	bestFormat.Title = fullInfo.Title
	bestFormat.OriginalURL = fullInfo.OriginalURL
	bestFormat.Ext = fullInfo.Ext
	bestFormat.Duration = fullInfo.Duration
	bestFormat.Uploader = fullInfo.Uploader
	bestFormat.UploadDate = fullInfo.UploadDate
	bestFormat.Thumbnail = fullInfo.Thumbnail

	return bestFormat, nil
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

	videoInfo, err := d.GetVideoInfo(ctx, url)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get video info: %w", err)
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
	return finalFilePath, videoInfo, nil
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

	videoInfo, err := d.GetVideoInfo(ctx, url)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get audio info: %w", err)
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
	return finalFilePath, videoInfo, nil
}

// StreamVideo streams video from the given URL by piping yt-dlp output.
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
		"--format", fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec),
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

// StreamAudio streams audio from the given URL by piping yt-dlp output.
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

// DownloadVideoToTempFile downloads a video to a temporary file on the server.
// Returns the path to the temporary file and any error.
func (d *Downloader) DownloadVideoToTempFile(ctx context.Context, url string, format string, resolution string, codec string) (string, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "720"
	}
	if codec == "" {
		codec = "avc1"
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "video-download-*.mp4")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempFilePath := tempFile.Name()
	tempFile.Close() // Close immediately, yt-dlp will write to it

	downloadArgs := []string{
		"--format", fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec),
		"--output", tempFilePath,
		"--no-progress",
		"--no-playlist",
		"--recode-video", format,
		url,
	}

	downloadCmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, downloadArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for temp video download: %s %s", d.cfg.YTDLPPath, strings.Join(downloadArgs, " ")))

	var downloadStderr bytes.Buffer
	downloadCmd.Stderr = &downloadStderr

	err = downloadCmd.Run()
	if err != nil {
		os.Remove(tempFilePath) // Clean up partial download
		slog.Error(fmt.Sprintf("yt-dlp temp video download failed: %v\nStderr: %s", err, downloadStderr.String()))
		return "", fmt.Errorf("yt-dlp temp video download failed: %w, stderr: %s", err, downloadStderr.String())
	}

	slog.Info(fmt.Sprintf("Video downloaded to temporary file: %s", tempFilePath))
	return tempFilePath, nil
}

// DownloadAudioToTempFile downloads audio to a temporary file on the server.
// Returns the path to the temporary file and any error.
func (d *Downloader) DownloadAudioToTempFile(ctx context.Context, url string, outputFormat string, codec string, bitrate string) (string, error) {
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	if codec == "" {
		codec = "libmp3lame"
	}
	if bitrate == "" {
		bitrate = "128k"
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "audio-download-*.mp3") // Use .mp3 as default extension
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempFilePath := tempFile.Name()
	tempFile.Close() // Close immediately, yt-dlp will write to it

	downloadArgs := []string{
		"--extract-audio",
		"--audio-format", outputFormat,
		"--audio-quality", bitrate,
		"--postprocessor-args", fmt.Sprintf("ffmpeg:-acodec %s", codec),
		"--output", tempFilePath,
		"--no-progress",
		"--no-playlist",
		url,
	}

	downloadCmd := exec.CommandContext(ctx, d.cfg.YTDLPPath, downloadArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for temp audio download: %s %s", d.cfg.YTDLPPath, strings.Join(downloadArgs, " ")))

	var downloadStderr bytes.Buffer
	downloadCmd.Stderr = &downloadStderr

	err = downloadCmd.Run()
	if err != nil {
		os.Remove(tempFilePath) // Clean up partial download
		slog.Error(fmt.Sprintf("yt-dlp temp audio download failed: %v\nStderr: %s", err, downloadStderr.String()))
		return "", fmt.Errorf("yt-dlp temp audio download failed: %w, stderr: %s", err, downloadStderr.String())
	}

	slog.Info(fmt.Sprintf("Audio downloaded to temporary file: %s", tempFilePath))
	return tempFilePath, nil
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
