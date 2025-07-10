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
	if err != nil {
		t.Fatalf("Failed to create mock yt-dlp: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Failed to create mock ffmpeg: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

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
	if err != nil {
		t.Fatalf("DownloadVideoToFile failed: %v", err)
	}

	if filePath == "" {
		t.Fatal("Returned file path is empty")
	}

	// Verify file exists and contains expected content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if !strings.Contains(string(content), "Converted content from") && !strings.Contains(string(content), "Mock video content") {
		t.Errorf("File content mismatch: %s", string(content))
	}

	// Verify file is in the correct directory
	if filepath.Dir(filePath) != downloadDir {
		t.Errorf("File downloaded to wrong directory. Expected %s, got %s", downloadDir, filepath.Dir(filePath))
	}

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.DownloadVideoToFile(url, format, resolution, codec)
	if err == nil {
		t.Error("Expected error when yt-dlp fails, got none")
	}
	if !strings.Contains(err.Error(), "yt-dlp video download failed") {
		t.Errorf("Expected yt-dlp failure error, got: %v", err)
	}

	// Test with ffmpeg failure (conversion)
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.DownloadVideoToFile(url, format, resolution, codec)
	if err == nil {
		t.Error("Expected error when ffmpeg fails, got none")
	}
	if !strings.Contains(err.Error(), "ffmpeg conversion failed") {
		t.Errorf("Expected ffmpeg conversion failure error, got: %v", err)
	}
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
	if err != nil {
		t.Fatalf("DownloadAudioToFile failed: %v", err)
	}

	if filePath == "" {
		t.Fatal("Returned file path is empty")
	}

	// Verify file exists and contains expected content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if !strings.Contains(string(content), "Converted content from") {
		t.Errorf("File content mismatch: %s", string(content))
	}

	// Verify file is in the correct directory
	if filepath.Dir(filePath) != downloadDir {
		t.Errorf("File downloaded to wrong directory. Expected %s, got %s", downloadDir, filepath.Dir(filePath))
	}

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.DownloadAudioToFile(url, outputFormat, codec, bitrate)
	if err == nil {
		t.Error("Expected error when yt-dlp fails, got none")
	}
	if !strings.Contains(err.Error(), "yt-dlp audio fetch failed") {
		t.Errorf("Expected yt-dlp failure error, got: %v", err)
	}

	// Test with ffmpeg failure (conversion)
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.DownloadAudioToFile(url, outputFormat, codec, bitrate)
	if err == nil {
		t.Error("Expected error when ffmpeg fails, got none")
	}
	if !strings.Contains(err.Error(), "ffmpeg conversion failed") {
		t.Errorf("Expected ffmpeg conversion failure error, got: %v", err)
	}
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
	if err != nil {
		t.Fatalf("StreamVideo failed: %v", err)
	}
	defer reader.Close()

	// Read from the stream
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		t.Fatalf("Failed to read from video stream: %v", err)
	}

	expectedContent := "mock stream data" // From mock yt-dlp, piped through mock ffmpeg
	if strings.TrimSpace(buf.String()) != expectedContent {
		t.Errorf("Stream content mismatch. Expected '%s', got '%s'", expectedContent, strings.TrimSpace(buf.String()))
	}

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.StreamVideo(url, format, resolution, codec)
	if err == nil {
		t.Error("Expected error when yt-dlp fails, got none")
	}
	if !strings.Contains(err.Error(), "failed to start yt-dlp") {
		t.Errorf("Expected yt-dlp failure error, got: %v", err)
	}

	// Test with ffmpeg failure
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.StreamVideo(url, format, resolution, codec)
	if err == nil {
		t.Error("Expected error when ffmpeg fails, got none")
	}
	if !strings.Contains(err.Error(), "failed to start ffmpeg") {
		t.Errorf("Expected ffmpeg failure error, got: %v", err)
	}
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
	if err != nil {
		t.Fatalf("StreamAudio failed: %v", err)
	}
	defer reader.Close()

	// Read from the stream
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		t.Fatalf("Failed to read from audio stream: %v", err)
	}

	expectedContent := "mock stream data" // From mock yt-dlp, piped through mock ffmpeg
	if strings.TrimSpace(buf.String()) != expectedContent {
		t.Errorf("Stream content mismatch. Expected '%s', got '%s'", expectedContent, strings.TrimSpace(buf.String()))
	}

	// Test with yt-dlp failure
	cfgFailYTDLP := createTestConfig(t, mockDir, downloadDir, "fail", "success")
	downloaderFailYTDLP := NewDownloader(cfgFailYTDLP)
	_, err = downloaderFailYTDLP.StreamAudio(url, outputFormat, codec, bitrate)
	if err == nil {
		t.Error("Expected error when yt-dlp fails, got none")
	}
	if !strings.Contains(err.Error(), "failed to start yt-dlp") {
		t.Errorf("Expected yt-dlp failure error, got: %v", err)
	}

	// Test with ffmpeg failure
	cfgFailFFMPEG := createTestConfig(t, mockDir, downloadDir, "success", "fail")
	downloaderFailFFMPEG := NewDownloader(cfgFailFFMPEG)
	_, err = downloaderFailFFMPEG.StreamAudio(url, outputFormat, codec, bitrate)
	if err == nil {
		t.Error("Expected error when ffmpeg fails, got none")
	}
	if !strings.Contains(err.Error(), "failed to start ffmpeg") {
		t.Errorf("Expected ffmpeg failure error, got: %v", err)
	}
}

func TestCommandReadCloserClose(t *testing.T) {
	// Create a dummy command that just exits
	cmd := exec.Command("bash", "-c", "echo 'test'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dummy command: %v", err)
	}

	crc := &commandReadCloser{
		ReadCloser: stdout,
		cmd:        cmd,
	}

	// Read some data to ensure pipe is active
	_, _ = io.ReadAll(crc)

	if err := crc.Close(); err != nil {
		t.Errorf("commandReadCloser.Close() failed: %v", err)
	}

	// Test with a command that fails
	cmdFail := exec.Command("bash", "-c", "exit 1")
	stdoutFail, err := cmdFail.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe for fail cmd: %v", err)
	}
	if err := cmdFail.Start(); err != nil {
		t.Fatalf("Failed to start dummy fail command: %v", err)
	}

	crcFail := &commandReadCloser{
		ReadCloser: stdoutFail,
		cmd:        cmdFail,
	}
	if err := crcFail.Close(); err == nil {
		t.Error("Expected error when command fails, got none")
	}
	if !strings.Contains(err.Error(), "command exited with error") {
		t.Errorf("Expected command exit error, got: %v", err)
	}
}

func TestPipedCommandReadCloserClose(t *testing.T) {
	// Create dummy commands that just exit
	cmd1 := exec.Command("bash", "-c", "echo 'data' && exit 0")
	cmd2 := exec.Command("bash", "-c", "cat /dev/stdin && exit 0")

	stdout1, err := cmd1.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe 1: %v", err)
	}
	cmd2.Stdin = stdout1

	stdout2, err := cmd2.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe 2: %v", err)
	}

	if err := cmd1.Start(); err != nil {
		t.Fatalf("Failed to start cmd1: %v", err)
	}
	if err := cmd2.Start(); err != nil {
		t.Fatalf("Failed to start cmd2: %v", err)
	}

	pcrc := &pipedCommandReadCloser{
		ReadCloser:   stdout2,
		primaryCmd:   cmd2,
		secondaryCmd: cmd1,
	}

	// Read some data to ensure pipe is active
	_, _ = io.ReadAll(pcrc)

	if err := pcrc.Close(); err != nil {
		t.Errorf("pipedCommandReadCloser.Close() failed: %v", err)
	}

	// Test with primary command failing
	cmd1Fail := exec.Command("bash", "-c", "echo 'data' && exit 0")
	cmd2Fail := exec.Command("bash", "-c", "exit 1") // Primary fails

	stdout1Fail, err := cmd1Fail.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe 1 fail: %v", err)
	}
	cmd2Fail.Stdin = stdout1Fail

	stdout2Fail, err := cmd2Fail.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe 2 fail: %v", err)
	}

	if err := cmd1Fail.Start(); err != nil {
		t.Fatalf("Failed to start cmd1Fail: %v", err)
	}
	if err := cmd2Fail.Start(); err != nil {
		t.Fatalf("Failed to start cmd2Fail: %v", err)
	}

	pcrcFailPrimary := &pipedCommandReadCloser{
		ReadCloser:   stdout2Fail,
		primaryCmd:   cmd2Fail,
		secondaryCmd: cmd1Fail,
	}
	if err := pcrcFailPrimary.Close(); err == nil {
		t.Error("Expected error when primary command fails, got none")
	}
	if !strings.Contains(err.Error(), "primary command exited with error") {
		t.Errorf("Expected primary command exit error, got: %v", err)
	}

	// Test with secondary command failing
	cmd1FailSecondary := exec.Command("bash", "-c", "exit 1") // Secondary fails
	cmd2FailSecondary := exec.Command("bash", "-c", "cat /dev/stdin && exit 0")

	stdout1FailSecondary, err := cmd1FailSecondary.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe 1 fail secondary: %v", err)
	}
	cmd2FailSecondary.Stdin = stdout1FailSecondary

	stdout2FailSecondary, err := cmd2FailSecondary.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe 2 fail secondary: %v", err)
	}

	if err := cmd1FailSecondary.Start(); err != nil {
		t.Fatalf("Failed to start cmd1FailSecondary: %v", err)
	}
	if err := cmd2FailSecondary.Start(); err != nil {
		t.Fatalf("Failed to start cmd2FailSecondary: %v", err)
	}

	pcrcFailSecondary := &pipedCommandReadCloser{
		ReadCloser:   stdout2FailSecondary,
		primaryCmd:   cmd2FailSecondary,
		secondaryCmd: cmd1FailSecondary,
	}
	if err := pcrcFailSecondary.Close(); err == nil {
		t.Error("Expected error when secondary command fails, got none")
	}
	if !strings.Contains(err.Error(), "secondary command exited with error") {
		t.Errorf("Expected secondary command exit error, got: %v", err)
	}
}
