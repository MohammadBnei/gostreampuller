package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	YTDLPPath  = "yt-dlp"
	FFMPEGPath = "ffmpeg"
)

func SetYTDLPPath(path string) {
	YTDLPPath = path
}

func SetFFMPEGPath(path string) {
	FFMPEGPath = path
}

// DownloadVideo downloads a video, allowing optional format, resolution, and codec parameters.
// If any parameter is empty, defaults will be used.
func DownloadVideo(url string, format string, resolution string, codec string) (string, error) {
	if format == "" {
		format = "mp4"
	}
	if resolution == "" {
		resolution = "720"
	}
	if codec == "" {
		codec = "avc1"
	}

	temp := fmt.Sprintf("video_%d.%%(ext)s", time.Now().UnixNano())
	selector := fmt.Sprintf("bestvideo[height<=%s][vcodec*=%s]+bestaudio/best", resolution, codec)

	cmd := exec.Command(YTDLPPath, "-f", selector, "-o", temp, url)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp video download failed: %w", err)
	}

	// Find the actual downloaded file by checking common extensions
	var downloaded string
	possibleExtensions := []string{"mkv", "mp4", "webm", "avi", "mov", "flv"}

	for _, ext := range possibleExtensions {
		candidate := strings.Replace(temp, "%(ext)s", ext, 1)
		if _, err := os.Stat(candidate); err == nil {
			downloaded = candidate
			break
		}
	}

	if downloaded == "" {
		return "", fmt.Errorf("could not find downloaded video file")
	}

	// If format is different from downloaded format, convert it
	finalOutput := strings.Replace(temp, "%(ext)s", format, 1)
	if downloaded != finalOutput {
		ffmpeg := exec.Command(FFMPEGPath, "-i", downloaded, "-c", "copy", "-y", finalOutput)
		if err := ffmpeg.Run(); err != nil {
			return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
		}
		defer os.Remove(downloaded)
		return filepath.Abs(finalOutput)
	}

	return filepath.Abs(downloaded)
}

// DownloadAudio downloads audio, allowing optional output format, codec, and bitrate parameters.
// If any parameter is empty, defaults will be used.
func DownloadAudio(url string, outputFormat string, codec string, bitrate string) (string, error) {
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	if codec == "" {
		codec = "libmp3lame"
	}
	if bitrate == "" {
		bitrate = "128k"
	}

	temp := fmt.Sprintf("audio_%d.%%(ext)s", time.Now().UnixNano())
	cmd := exec.Command(YTDLPPath, "-f", "bestaudio", "-o", temp, url)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp audio fetch failed: %w", err)
	}

	original := strings.Replace(temp, "%(ext)s", "webm", 1)
	output := strings.Replace(temp, "%(ext)s", outputFormat, 1)

	ffmpeg := exec.Command(FFMPEGPath, "-i", original, "-vn", "-acodec", codec, "-ab", bitrate, "-y", output)
	if err := ffmpeg.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	defer os.Remove(original)
	return filepath.Abs(output)
}
