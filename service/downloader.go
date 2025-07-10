package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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

// DownloadVideoToFile downloads a video from the given URL to a file.
// It returns the path to the downloaded file.
func (d *Downloader) DownloadVideoToFile(url string, format string, resolution string, codec string) (string, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "720"
	}
	if codec == "" {
		codec = "avc1"
	}

	// Use %(filepath)s to get the final path after yt-dlp's processing
	outputTemplate := filepath.Join(d.cfg.DownloadDir, "%(title)s")

	args := []string{
		"--format", fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec),
		"--output", outputTemplate,
		"--no-progress",
		"--restrict-filenames",   // Helps with predictable filenames
		"--no-playlist",          // Assume single video download
		"--recode-video", format, // Instruct yt-dlp to convert to the desired format
		url,
	}

	cmd := exec.Command(d.cfg.YTDLPPath, args...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for video download: %s %s", d.cfg.YTDLPPath, strings.Join(args, " ")))

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp video download failed: %v\nStdout: %s\nStderr: %s", err, stdoutBuf.String(), stderrBuf.String()))
		return "", fmt.Errorf("yt-dlp video download failed: %w, stderr: %s", err, stderrBuf.String())
	}

	// Parse stdout to find the downloaded filename using %(filepath)s
	// yt-dlp prints the final filepath when using --print filepath
	// However, when not using --print, it's usually in the last few lines of output.
	// A more robust way is to use --print filepath and capture that specific output.
	// For now, let's try to parse the "Destination" line which is usually reliable.
	outputLines := strings.Split(stdoutBuf.String(), "\n")
	var downloadedFilePath string
	for _, line := range outputLines {
		// Look for lines indicating the final destination after all processing
		if strings.Contains(line, d.cfg.DownloadDir) {
			downloadedFilePath = line
			break
		}
	}

	if downloadedFilePath == "" {
		return "", errors.New("could not find downloaded video file path in yt-dlp output")
	}

	downloadedFilePath = fmt.Sprintf("%s.%s", downloadedFilePath, format)

	_, err = os.Stat(downloadedFilePath)
	if err != nil {
		return "", fmt.Errorf("could not find downloaded video file: %w", err)
	}

	slog.Info(fmt.Sprintf("Video downloaded to: %s", downloadedFilePath))
	return downloadedFilePath, nil
}

// DownloadAudioToFile downloads audio from the given URL to a file.
// It returns the path to the downloaded file.
func (d *Downloader) DownloadAudioToFile(url string, outputFormat string, codec string, bitrate string) (string, error) {
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	if codec == "" {
		codec = "libmp3lame"
	}
	if bitrate == "" {
		bitrate = "128k"
	}

	outputTemplate := filepath.Join(d.cfg.DownloadDir, "%(title)s.%(ext)s")

	// yt-dlp command to download audio and convert
	args := []string{
		"--extract-audio",
		"--audio-format", outputFormat,
		"--audio-quality", bitrate, // Corresponds to bitrate for audio quality
		"--postprocessor-args", fmt.Sprintf("ffmpeg:-acodec %s", codec), // Specify audio codec for ffmpeg
		"--output", outputTemplate,
		"--restrict-filenames",
		"--no-playlist",
		url,
	}

	cmd := exec.Command(d.cfg.YTDLPPath, args...)
	slog.Debug(fmt.Sprintf("Executing yt-dlp for audio download: %s %s", d.cfg.YTDLPPath, strings.Join(args, " ")))

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		slog.Error(fmt.Sprintf("yt-dlp audio fetch failed: %v\nStdout: %s\nStderr: %s", err, stdoutBuf.String(), stderrBuf.String()))
		return "", fmt.Errorf("yt-dlp audio fetch failed: %w, stderr: %s", err, stderrBuf.String())
	}

	// Parse stdout to find the downloaded filename
	outputLines := strings.Split(stdoutBuf.String(), "\n")
	var downloadedFilePath string
	for _, line := range outputLines {
		// yt-dlp often renames the file after post-processing.
		// Look for the line indicating the final file.
		// Example: `[ExtractAudio] Destination: My Audio Title.mp3`
		// Or `[ffmpeg] Destination: My Audio Title.mp3`
		if strings.Contains(line, "Destination:") && strings.Contains(line, d.cfg.DownloadDir) {
			parts := strings.Split(line, "Destination:")
			if len(parts) > 1 {
				potentialPath := strings.TrimSpace(parts[1])
				if filepath.IsAbs(potentialPath) && strings.HasPrefix(potentialPath, d.cfg.DownloadDir) {
					downloadedFilePath = potentialPath
					break
				}
			}
		}
	}

	if downloadedFilePath == "" {
		return "", errors.New("could not find downloaded audio file path in yt-dlp output")
	}

	slog.Info(fmt.Sprintf("Audio downloaded to: %s", downloadedFilePath))
	return downloadedFilePath, nil
}

// StreamVideo streams video from the given URL.
func (d *Downloader) StreamVideo(url string, format string, resolution string, codec string) (io.ReadCloser, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "720"
	}
	if codec == "" {
		codec = "avc1"
	}

	// yt-dlp command to output raw stream to stdout
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
