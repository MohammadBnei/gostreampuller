package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gostreampuller/config" // Import the config package
)

// Downloader provides methods to download video and audio content.
type Downloader struct {
	cfg *config.Config
}

// NewDownloader creates a new Downloader instance.
func NewDownloader(cfg *config.Config) *Downloader {
	return &Downloader{
		cfg: cfg,
	}
}

// DownloadVideoToFile downloads a video to a local file, allowing optional format, resolution, and codec parameters.
// If any parameter is empty, defaults will be used.
// It returns the absolute path to the downloaded file.
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

	// Construct the temporary file path within the configured download directory
	tempFileName := "%(title)s.%(ext)s"
	tempFilePath := filepath.Join(d.cfg.DownloadDir, tempFileName)

	selector := fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec)

	cmd := exec.Command(d.cfg.YTDLPPath, "-f", selector, "-o", tempFilePath, url)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp video download failed: %w", err)
	}

	// Find the actual downloaded file by checking common extensions
	var downloaded string
	possibleExtensions := []string{"mkv", "mp4", "webm", "avi", "mov", "flv"}

	for _, ext := range possibleExtensions {
		candidate := strings.Replace(tempFilePath, "%(ext)s", ext, 1)
		if _, err := os.Stat(candidate); err == nil {
			downloaded = candidate
			break
		}
	}

	if downloaded == "" {
		return "", fmt.Errorf("could not find downloaded video file")
	}

	// If format is different from downloaded format, convert it
	finalOutput := strings.Replace(tempFilePath, "%(ext)s", format, 1)
	if downloaded != finalOutput {
		ffmpeg := exec.Command(d.cfg.FFMPEGPath, "-i", downloaded, "-c", "copy", "-y", finalOutput)
		if err := ffmpeg.Run(); err != nil {
			return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
		}
		defer os.Remove(downloaded)
		return filepath.Abs(finalOutput)
	}

	return filepath.Abs(downloaded)
}

// DownloadAudioToFile downloads audio to a local file, allowing optional output format, codec, and bitrate parameters.
// If any parameter is empty, defaults will be used.
// It returns the absolute path to the downloaded file.
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

	// Construct the temporary file path within the configured download directory
	tempFileName := fmt.Sprintf("audio_%d.%%(ext)s", time.Now().UnixNano())
	tempFilePath := filepath.Join(d.cfg.DownloadDir, tempFileName)

	cmd := exec.Command(d.cfg.YTDLPPath, "-f", "bestaudio", "-o", tempFilePath, url)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp audio fetch failed: %w", err)
	}

	original := strings.Replace(tempFilePath, "%(ext)s", "webm", 1) // yt-dlp often downloads audio as webm
	output := strings.Replace(tempFilePath, "%(ext)s", outputFormat, 1)

	ffmpeg := exec.Command(d.cfg.FFMPEGPath, "-i", original, "-vn", "-acodec", codec, "-ab", bitrate, "-y", output)
	if err := ffmpeg.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	defer os.Remove(original)
	return filepath.Abs(output)
}

// StreamVideo streams a video directly, allowing optional format, resolution, and codec parameters.
// It returns an io.ReadCloser that provides the video stream.
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

	// yt-dlp command to output to stdout
	// Use -o - to output to stdout
	// Use -f for format selection
	selector := fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec)
	ytDLPArgs := []string{"-f", selector, "-o", "-", url}

	ytDLPcmd := exec.Command(d.cfg.YTDLPPath, ytDLPArgs...)

	// Get stdout pipe from yt-dlp
	ytDLPStdout, err := ytDLPcmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get yt-dlp stdout pipe: %w", err)
	}

	// Start yt-dlp command
	if err := ytDLPcmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	// For simplicity, we'll return the yt-dlp stdout directly.
	// If a specific format is requested that yt-dlp cannot directly output to stdout,
	// we would need to pipe through ffmpeg here.
	// Example for piping through ffmpeg if needed:
	// ffmpegArgs := []string{"-i", "pipe:0", "-f", format, "-c", "copy", "pipe:1"} // Adjust codecs as needed
	// ffmpegCmd := exec.Command(d.cfg.FFMPEGPath, ffmpegArgs...)
	// ffmpegCmd.Stdin = ytDLPStdout
	// ffmpegStdout, err := ffmpegCmd.StdoutPipe()
	// if err != nil { /* handle error */ }
	// if err := ffmpegCmd.Start(); err != nil { /* handle error */ }
	// return &pipedCommandReadCloser{ReadCloser: ffmpegStdout, primaryCmd: ffmpegCmd, secondaryCmd: ytDLPcmd}, nil

	return &commandReadCloser{
		ReadCloser: ytDLPStdout,
		cmd:        ytDLPcmd,
	}, nil
}

// StreamAudio streams audio directly, allowing optional output format, codec, and bitrate parameters.
// It returns an io.ReadCloser that provides the audio stream.
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

	// yt-dlp command to output best audio to stdout
	ytDLPArgs := []string{"-f", "bestaudio", "-o", "-", url}
	ytDLPcmd := exec.Command(d.cfg.YTDLPPath, ytDLPArgs...)

	ytDLPStdout, err := ytDLPcmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get yt-dlp stdout pipe: %w", err)
	}

	// ffmpeg command to convert the audio stream from yt-dlp's stdout to the desired format
	// -i pipe:0 reads from stdin
	// -f <outputFormat> sets the output format
	// -vn no video
	// -acodec <codec> sets the audio codec
	// -ab <bitrate> sets the audio bitrate
	// -o pipe:1 outputs to stdout
	ffmpegArgs := []string{"-i", "pipe:0", "-f", outputFormat, "-vn", "-acodec", codec, "-ab", bitrate, "-y", "pipe:1"}
	ffmpegCmd := exec.Command(d.cfg.FFMPEGPath, ffmpegArgs...)

	ffmpegCmd.Stdin = ytDLPStdout // Pipe yt-dlp's stdout to ffmpeg's stdin

	ffmpegStdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get ffmpeg stdout pipe: %w", err)
	}

	// Start both commands
	if err := ytDLPcmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp: %w", err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		// If ffmpeg fails to start, ensure yt-dlp is killed
		ytDLPcmd.Process.Kill()
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Return a ReadCloser that manages both commands
	return &pipedCommandReadCloser{
		ReadCloser:   ffmpegStdout,
		primaryCmd:   ffmpegCmd,
		secondaryCmd: ytDLPcmd, // Ensure yt-dlp is also waited on
	}, nil
}

// commandReadCloser wraps an io.ReadCloser and ensures the associated command is waited on when closed.
type commandReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (crc *commandReadCloser) Close() error {
	err := crc.ReadCloser.Close() // Close the pipe
	waitErr := crc.cmd.Wait()     // Wait for the command to exit

	if err != nil {
		return fmt.Errorf("error closing pipe: %w; command wait error: %v", err, waitErr)
	}
	if waitErr != nil {
		return fmt.Errorf("command exited with error: %w", waitErr)
	}
	return nil
}

// pipedCommandReadCloser manages two piped commands, ensuring both are waited on when closed.
type pipedCommandReadCloser struct {
	io.ReadCloser
	primaryCmd   *exec.Cmd // The command whose stdout is being read (e.g., ffmpeg)
	secondaryCmd *exec.Cmd // The command whose stdout is piped to primaryCmd's stdin (e.g., yt-dlp)
}

func (pcrc *pipedCommandReadCloser) Close() error {
	var errs []error

	// Close the pipe from the primary command
	if err := pcrc.ReadCloser.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing primary pipe: %w", err))
	}

	// Wait for the primary command to exit
	if err := pcrc.primaryCmd.Wait(); err != nil {
		errs = append(errs, fmt.Errorf("primary command exited with error: %w", err))
	}

	// Wait for the secondary command to exit
	if err := pcrc.secondaryCmd.Wait(); err != nil {
		errs = append(errs, fmt.Errorf("secondary command exited with error: %w", err))
	}

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
