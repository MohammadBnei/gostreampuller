package service

import (
	"bytes"
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
	WebpageURL  string `json:"webpage_url"`
	Ext         string `json:"ext"`
	Duration    int    `json:"duration"` // in seconds
	Uploader    string `json:"uploader"`
	UploadDate  string `json:"upload_date"` // YYYYMMDD
	Thumbnail   string `json:"thumbnail"`   // URL to thumbnail
}

// DownloadVideoToFile downloads a video from the given URL to a file.
// It returns the path to the downloaded file and its metadata.
func (d *Downloader) DownloadVideoToFile(url string, format string, resolution string, codec string) (string, *VideoInfo, error) {
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
	infoCmd := exec.Command(d.cfg.YTDLPPath, infoArgs...)
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
	uniqueFilename := fmt.Sprintf("%d-%s.%s", time.Now().UnixNano(), videoInfo.ID, videoInfo.Ext)
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

	downloadCmd := exec.Command(d.cfg.YTDLPPath, downloadArgs...)
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
func (d *Downloader) DownloadAudioToFile(url string, outputFormat string, codec string, bitrate string) (string, *VideoInfo, error) {
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
	infoCmd := exec.Command(d.cfg.YTDLPPath, infoArgs...)
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

	downloadCmd := exec.Command(d.cfg.YTDLPPath, downloadArgs...)
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
func (d *Downloader) StreamVideo(url string, format string, resolution string, codec string) (io.ReadCloser, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "700" // Default to 700p for streaming if not specified
	}
	if codec == "" {
		codec = "avc1"
	}

	// yt-dlp command to output raw stream to stdout
	// Use bestvideo[ext=mp4][height<=720]+bestaudio/best[ext=mp4][height<=720]/best
	// This selects the best video and audio streams that match the desired format and resolution.
	// If no such stream exists, it falls back to the general "best" stream.
	ytDLPArgs := []string{
		"--format", fmt.Sprintf("bestvideo[ext=%s][height<=%s]+bestaudio/best[ext=%s][height<=%s]/best", format, resolution, format, resolution),
		"-o", "-", // Output to stdout
		url,
	}
	ytDLPcmd := exec.Command(d.cfg.YTDLPPath, ytDLPArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for video stream pipe: %s %s", d.cfg.YTDLPPath, strings.Join(ytDLPArgs, " ")))

	ytDLPStdoutPipe, err := ytDLPcmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for yt-dlp: %w", err)
	}
	ytDLPcmd.Stderr = os.Stderr // Direct yt-dlp errors to stderr for debugging

	// ffmpeg command to process the stream from stdin and output to stdout
	ffmpegArgs := []string{
		"-i", "pipe:0", // Input from stdin
		"-f", format,
		"-c:v", codec,
		"-preset", "veryfast", // Optimize for streaming
		"-movflags", "frag_keyframe+empty_moov", // Essential for fragmented MP4 streaming
		"pipe:1", // Output to stdout
	}
	ffmpegCmd := exec.Command(d.cfg.FFMPEGPath, ffmpegArgs...)
	slog.Debug(fmt.Sprintf("Executing ffmpeg for video stream pipe: %s %s", d.cfg.FFMPEGPath, strings.Join(ffmpegArgs, " ")))

	ffmpegCmd.Stdin = ytDLPStdoutPipe
	ffmpegStdoutPipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for ffmpeg: %w", err)
	}
	ffmpegCmd.Stderr = os.Stderr // Direct ffmpeg errors to stderr for debugging

	if err := ytDLPcmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp command for video stream: %w", err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		// If ffmpeg fails to start, ensure yt-dlp is killed
		ytDLPcmd.Process.Kill()
		return nil, fmt.Errorf("failed to start ffmpeg command for video stream: %w", err)
	}

	return &pipedCommandReadCloser{
		ReadCloser:   ffmpegStdoutPipe,
		primaryCmd:   ffmpegCmd,
		secondaryCmd: ytDLPcmd,
	}, nil
}

// StreamAudio streams audio from the given URL.
func (d *Downloader) StreamAudio(url string, outputFormat string, codec string, bitrate string) (io.ReadCloser, error) {
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	if codec == "" {
		codec = "libmp3lame"
	}
	if bitrate == "" {
		bitrate = "128k"
	}

	// yt-dlp command to output raw audio stream to stdout
	ytDLPArgs := []string{
		"--extract-audio",
		"--audio-format", "best", // Let yt-dlp choose best audio format
		"-o", "-", // Output to stdout
		url,
	}
	ytDLPcmd := exec.Command(d.cfg.YTDLPPath, ytDLPArgs...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for audio stream pipe: %s %s", d.cfg.YTDLPPath, strings.Join(ytDLPArgs, " ")))

	ytDLPStdoutPipe, err := ytDLPcmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for yt-dlp: %w", err)
	}
	ytDLPcmd.Stderr = os.Stderr // Direct yt-dlp errors to stderr for debugging

	// ffmpeg command to process the stream from stdin and output to stdout
	ffmpegArgs := []string{
		"-i", "pipe:0", // Input from stdin
		"-f", outputFormat,
		"-c:a", codec,
		"-b:a", bitrate,
		"pipe:1", // Output to stdout
	}
	ffmpegCmd := exec.Command(d.cfg.FFMPEGPath, ffmpegArgs...)
	slog.Debug(fmt.Sprintf("Executing ffmpeg for audio stream pipe: %s %s", d.cfg.FFMPEGPath, strings.Join(ffmpegArgs, " ")))

	ffmpegCmd.Stdin = ytDLPStdoutPipe
	ffmpegStdoutPipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for ffmpeg: %w", err)
	}
	ffmpegCmd.Stderr = os.Stderr // Direct ffmpeg errors to stderr for debugging

	if err := ytDLPcmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp command for audio stream: %w", err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		// If ffmpeg fails to start, ensure yt-dlp is killed
		ytDLPcmd.Process.Kill()
		return nil, fmt.Errorf("failed to start ffmpeg command for audio stream: %w", err)
	}

	return &pipedCommandReadCloser{
		ReadCloser:   ffmpegStdoutPipe,
		primaryCmd:   ffmpegCmd,
		secondaryCmd: ytDLPcmd,
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

// pipedCommandReadCloser manages two piped commands, ensuring both are waited on when closed.
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
