package service

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gostreampuller/config"

	"github.com/stretchr/testify/assert"
)

// setupMockExecutables creates mock yt-dlp and ffmpeg executables in a temporary directory.
// These mocks can be configured to simulate success or failure.
func setupMockExecutables(t *testing.T, mockDir string, ytDLPBehavior, ffmpegBehavior string) (string, string) {
	ytDLPPath := filepath.Join(mockDir, "yt-dlp")
	ffmpegPath := filepath.Join(mockDir, "ffmpeg")

	// Create mock yt-dlp
	ytDLPContent := `#!/bin/bash
if [ "$1" == "--version" ]; then
    echo "yt-dlp mock version"
    exit 0
fi
if [ "$1" == "-f" ] && [ "$3" == "-o" ]; then
    # Simulate file download
    if [ "` + ytDLPBehavior + `" == "fail" ]; then
        echo "yt-dlp mock failed to download" >&2
        exit 1
    fi
    output_template=$4
    # Replace %(ext)s with a common extension like mp4 or webm
    output_file=$(echo "$output_template" | sed 's/%\(ext\)s/mp4/g' | sed 's/%\(title\)s/mock_video/g')
    echo "Mock video content" > "$output_file"
    exit 0
elif [ "$1" == "-f" ] && [ "$2" == "bestaudio" ] && [ "$3" == "-o" ]; then
    # Simulate audio file download
    if [ "` + ytDLPBehavior + `" == "fail" ]; then
        echo "yt-dlp mock failed to download audio" >&2
        exit 1
    fi
    output_template=$4
    output_file=$(echo "$output_template" | sed 's/%\(ext\)s/webm/g') # yt-dlp often downloads audio as webm
    echo "Mock audio content" > "$output_file"
    exit 0
elif [ "$1" == "-f" ] && [ "$3" == "-o" ] && [ "$4" == "-" ]; then
    # Simulate streaming to stdout
    if [ "` + ytDLPBehavior + `" == "fail" ]; then
        echo "yt-dlp mock failed to stream" >&2
        exit 1
    fi
    echo "mock stream data"
    exit 0
fi
echo "yt-dlp mock: Unknown command: $*" >&2
exit 1
`
	err := os.WriteFile(ytDLPPath, []byte(ytDLPContent), 0755)
	assert.NoError(t, err, "Failed to create mock yt-dlp")

	// Create mock ffmpeg
	ffmpegContent := `#!/bin/bash
if [ "$1" == "-version" ]; then
    echo "ffmpeg mock version"
    exit 0
fi
if [ "$1" == "-i" ] && [ "$2" == "pipe:0" ] && [ "$NF" == "pipe:1" ]; then
    # Simulate streaming conversion
    if [ "` + ffmpegBehavior + `" == "fail" ]; then
        echo "ffmpeg mock failed to convert stream" >&2
        exit 1
    fi
    cat /dev/stdin # Read from stdin and write to stdout
    exit 0
elif [ "$1" == "-i" ] && [ "$3" == "-c" ] && [ "$4" == "copy" ]; then
    # Simulate file conversion
    if [ "` + ffmpegBehavior + `" == "fail" ]; then
        echo "ffmpeg mock failed to convert file" >&2
        exit 1
    fi
    input_file=$2
    output_file=$6
    echo "Converted content from $input_file" > "$output_file"
    exit 0
fi
echo "ffmpeg mock: Unknown command: $*" >&2
exit 1
`
	err = os.WriteFile(ffmpegPath, []byte(ffmpegContent), 0755)
	assert.NoError(t, err, "Failed to create mock ffmpeg")

	return ytDLPPath, ffmpegPath
}

// createTestConfig creates a config.Config for testing with mock executables.
func createTestConfig(t *testing.T, mockDir, downloadDir, ytDLPBehavior, ffmpegBehavior string) *config.Config {
	ytDLPPath, ffmpegPath := setupMockExecutables(t, mockDir, ytDLPBehavior, ffmpegBehavior)

	// Temporarily set env vars for config.New() to pick up mock paths
	os.Setenv("YTDLP_PATH", ytDLPPath)
	os.Setenv("FFMPEG_PATH", ffmpegPath)
	os.Setenv("DOWNLOAD_DIR", downloadDir)
	os.Setenv("LOCAL_MODE", "true") // Bypass auth for tests

	cfg, err := config.New()
	assert.NoError(t, err, "Failed to create test config")

	// Unset env vars to avoid affecting other tests if not using t.Setenv
	os.Unsetenv("YTDLP_PATH")
	os.Unsetenv("FFMPEG_PATH")
	os.Unsetenv("DOWNLOAD_DIR")
	os.Unsetenv("LOCAL_MODE")

	return cfg
}

func TestDownloadVideoToFile(t *testing.T) {
	mockDir := t.TempDir()
	downloadDir := t.TempDir()
	cfg := createTestConfig(t, mockDir, downloadDir, "success", "success")
	downloader := NewDownloader(cfg)

	url := "http://example.com/video"
	format := "mp4"
	resolution := "720"
	codec := "h264"

	filePath, err := downloader.DownloadVideoToFile(url, format, resolution, codec)
	assert.NoError(t, err, "DownloadVideoToFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")

	// Verify file exists and contains expected content
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err, "Failed to read downloaded file")
	assert.True(t, strings.Contains(string(content), "Converted content from") || strings.Contains(string(content), "Mock video content"), "File content mismatch")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.DownloadVideoToFile(url, format, resolution, codec)
	assert.Error(t, err, "Expected error when yt-dlp fails")
	assert.Contains(t, err.Error(), "yt-dlp video download failed", "Expected yt-dlp failure error message")

	// Test with ffmpeg failure (conversion)
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.DownloadVideoToFile(url, format, resolution, codec)
	assert.Error(t, err, "Expected error when ffmpeg fails")
	assert.Contains(t, err.Error(), "ffmpeg conversion failed", "Expected ffmpeg conversion failure error message")
}

func TestDownloadAudioToFile(t *testing.T) {
	mockDir := t.TempDir()
	downloadDir := t.TempDir()
	cfg := createTestConfig(t, mockDir, downloadDir, "success", "success")
	downloader := NewDownloader(cfg)

	url := "http://example.com/audio"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	filePath, err := downloader.DownloadAudioToFile(url, outputFormat, codec, bitrate)
	assert.NoError(t, err, "DownloadAudioToFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")

	// Verify file exists and contains expected content
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err, "Failed to read downloaded file")
	assert.Contains(t, string(content), "Converted content from", "File content mismatch")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.DownloadAudioToFile(url, outputFormat, codec, bitrate)
	assert.Error(t, err, "Expected error when yt-dlp fails")
	assert.Contains(t, err.Error(), "yt-dlp audio fetch failed", "Expected yt-dlp failure error message")

	// Test with ffmpeg failure (conversion)
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.DownloadAudioToFile(url, outputFormat, codec, bitrate)
	assert.Error(t, err, "Expected error when ffmpeg fails")
	assert.Contains(t, err.Error(), "ffmpeg conversion failed", "Expected ffmpeg conversion failure error message")
}

func TestStreamVideo(t *testing.T) {
	mockDir := t.TempDir()
	downloadDir := t.TempDir() // Still needed for config creation, though not used by streaming
	cfg := createTestConfig(t, mockDir, downloadDir, "success", "success")
	downloader := NewDownloader(cfg)

	url := "http://example.com/stream_video"
	format := "mp4"
	resolution := "720"
	codec := "h264"

	reader, err := downloader.StreamVideo(url, format, resolution, codec)
	assert.NoError(t, err, "StreamVideo should not fail on success")
	defer reader.Close()

	// Read from the stream
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	assert.NoError(t, err, "Failed to read from video stream")

	expectedContent := "mock stream data" // From mock yt-dlp, piped through mock ffmpeg
	assert.Equal(t, expectedContent, strings.TrimSpace(buf.String()), "Stream content mismatch")

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.StreamVideo(url, format, resolution, codec)
	assert.Error(t, err, "Expected error when yt-dlp fails")
	assert.Contains(t, err.Error(), "failed to start yt-dlp", "Expected yt-dlp failure error message")

	// Test with ffmpeg failure
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.StreamVideo(url, format, resolution, codec)
	assert.Error(t, err, "Expected error when ffmpeg fails")
	assert.Contains(t, err.Error(), "failed to start ffmpeg", "Expected ffmpeg failure error message")
}

func TestStreamAudio(t *testing.T) {
	mockDir := t.TempDir()
	downloadDir := t.TempDir() // Still needed for config creation, though not used by streaming
	cfg := createTestConfig(t, mockDir, downloadDir, "success", "success")
	downloader := NewDownloader(cfg)

	url := "http://example.com/stream_audio"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	reader, err := downloader.StreamAudio(url, outputFormat, codec, bitrate)
	assert.NoError(t, err, "StreamAudio should not fail on success")
	defer reader.Close()

	// Read from the stream
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	assert.NoError(t, err, "Failed to read from audio stream")

	expectedContent := "mock stream data" // From mock yt-dlp, piped through mock ffmpeg
	assert.Equal(t, expectedContent, strings.TrimSpace(buf.String()), "Stream content mismatch")

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.StreamAudio(url, outputFormat, codec, bitrate)
	assert.Error(t, err, "Expected error when yt-dlp fails")
	assert.Contains(t, err.Error(), "failed to start yt-dlp", "Expected yt-dlp failure error message")

	// Test with ffmpeg failure
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.StreamAudio(url, outputFormat, codec, bitrate)
	assert.Error(t, err, "Expected error when ffmpeg fails")
	assert.Contains(t, err.Error(), "failed to start ffmpeg", "Expected ffmpeg failure error message")
}

func TestCommandReadCloserClose(t *testing.T) {
	// Create a dummy command that just exits
	cmd := exec.Command("bash", "-c", "echo 'test'")
	stdout, err := cmd.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe")
	err = cmd.Start()
	assert.NoError(t, err, "Failed to start dummy command")

	crc := &commandReadCloser{
		ReadCloser: stdout,
		cmd:        cmd,
	}

	// Read some data to ensure pipe is active
	_, _ = io.ReadAll(crc)

	err = crc.Close()
	assert.NoError(t, err, "commandReadCloser.Close() should not fail on success")

	// Test with a command that fails
	cmdFail := exec.Command("bash", "-c", "exit 1")
	stdoutFail, err := cmdFail.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe for fail cmd")
	err = cmdFail.Start()
	assert.NoError(t, err, "Failed to start dummy fail command")

	crcFail := &commandReadCloser{
		ReadCloser: stdoutFail,
		cmd:        cmdFail,
	}
	err = crcFail.Close()
	assert.Error(t, err, "Expected error when command fails")
	assert.Contains(t, err.Error(), "command exited with error", "Expected command exit error message")
}

func TestPipedCommandReadCloserClose(t *testing.T) {
	// Create dummy commands that just exit
	cmd1 := exec.Command("bash", "-c", "echo 'data' && exit 0")
	cmd2 := exec.Command("bash", "-c", "cat /dev/stdin && exit 0")

	stdout1, err := cmd1.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe 1")
	cmd2.Stdin = stdout1

	stdout2, err := cmd2.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe 2")

	err = cmd1.Start()
	assert.NoError(t, err, "Failed to start cmd1")
	err = cmd2.Start()
	assert.NoError(t, err, "Failed to start cmd2")

	pcrc := &pipedCommandReadCloser{
		ReadCloser:   stdout2,
		primaryCmd:   cmd2,
		secondaryCmd: cmd1,
	}

	// Read some data to ensure pipe is active
	_, _ = io.ReadAll(pcrc)

	err = pcrc.Close()
	assert.NoError(t, err, "pipedCommandReadCloser.Close() should not fail on success")

	// Test with primary command failing
	cmd1Fail := exec.Command("bash", "-c", "echo 'data' && exit 0")
	cmd2Fail := exec.Command("bash", "-c", "exit 1") // Primary fails

	stdout1Fail, err := cmd1Fail.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe 1 fail")
	cmd2Fail.Stdin = stdout1Fail

	stdout2Fail, err := cmd2Fail.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe 2 fail")

	err = cmd1Fail.Start()
	assert.NoError(t, err, "Failed to start cmd1Fail")
	err = cmd2Fail.Start()
	assert.NoError(t, err, "Failed to start cmd2Fail")

	pcrcFailPrimary := &pipedCommandReadCloser{
		ReadCloser:   stdout2Fail,
		primaryCmd:   cmd2Fail,
		secondaryCmd: cmd1Fail,
	}
	err = pcrcFailPrimary.Close()
	assert.Error(t, err, "Expected error when primary command fails")
	assert.Contains(t, err.Error(), "primary command exited with error", "Expected primary command exit error message")

	// Test with secondary command failing
	cmd1FailSecondary := exec.Command("bash", "-c", "exit 1") // Secondary fails
	cmd2FailSecondary := exec.Command("bash", "-c", "cat /dev/stdin && exit 0")

	stdout1FailSecondary, err := cmd1FailSecondary.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe 1 fail secondary")
	cmd2FailSecondary.Stdin = stdout1FailSecondary

	stdout2FailSecondary, err := cmd2FailSecondary.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe 2 fail secondary")

	err = cmd1FailSecondary.Start()
	assert.NoError(t, err, "Failed to start cmd1FailSecondary")
	err = cmd2FailSecondary.Start()
	assert.NoError(t, err, "Failed to start cmd2FailSecondary")

	pcrcFailSecondary := &pipedCommandReadCloser{
		ReadCloser:   stdout2FailSecondary,
		primaryCmd:   cmd2FailSecondary,
		secondaryCmd: cmd1FailSecondary,
	}
	err = pcrcFailSecondary.Close()
	assert.Error(t, err, "Expected error when secondary command fails")
	assert.Contains(t, err.Error(), "secondary command exited with error", "Expected secondary command exit error message")
}
