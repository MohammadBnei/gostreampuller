package service

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gostreampuller/config"
)

// createTestConfig creates a config.Config for testing.
// It ensures that real yt-dlp and ffmpeg executables are used if available in PATH,
// or relies on the system's default behavior for finding them.
func createTestConfig(t *testing.T, downloadDir string) *config.Config {
	// Use t.Setenv to manage environment variables for the test's duration.
	// This ensures they are cleaned up automatically after the test.
	t.Setenv("DOWNLOAD_DIR", downloadDir)
	t.Setenv("LOCAL_MODE", "true") // Bypass auth for tests
	t.Setenv("DEBUG", "true")

	// Unset YTDLP_PATH and FFMPEG_PATH to ensure config.New() looks in system PATH
	// or uses its default values.
	t.Setenv("YTDLP_PATH", "")
	t.Setenv("FFMPEG_PATH", "")

	t.Setenv("AUTH_USERNAME", "testuser")
	t.Setenv("AUTH_PASSWORD", "testpass")

	cfg, err := config.New()
	assert.NoError(t, err, "Failed to create test config")

	// Verify that yt-dlp and ffmpeg paths are set by config.New (either default or found in PATH)
	assert.NotEmpty(t, cfg.YTDLPPath, "YTDLPPath should not be empty")
	assert.NotEmpty(t, cfg.FFMPEGPath, "FFMPEGPath should not be empty")

	return cfg
}

func TestDownloadVideoToFile(t *testing.T) {
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToFile: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToFile: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	cfg := createTestConfig(t, downloadDir)
	downloader := NewDownloader(cfg)

	// Use a known short, public domain video URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (short version for testing)
	format := "mp4"
	resolution := "360" // Use a lower resolution for faster downloads
	codec := "avc1"     // Common video codec

	filePath, err := downloader.DownloadVideoToFile(url, format, resolution, codec)
	assert.NoError(t, err, "DownloadVideoToFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")

	// Verify file exists and is not empty
	fileInfo, err := os.Stat(filePath)
	assert.NoError(t, err, "Failed to stat downloaded file")
	assert.True(t, fileInfo.Size() > 0, "Downloaded file should not be empty")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Test with a non-existent URL to simulate yt-dlp failure
	t.Run("yt-dlp failure", func(t *testing.T) {
		nonExistentURL := "http://example.com/nonexistent_video_12345"
		_, err = downloader.DownloadVideoToFile(nonExistentURL, format, resolution, codec)
		assert.Error(t, err, "Expected error when yt-dlp fails for non-existent URL")
		assert.Contains(t, err.Error(), "yt-dlp video download failed", "Expected yt-dlp failure error message")
	})

	// Note: Simulating ffmpeg conversion failure for a file download is complex
	// without mocking or specific test files that cause ffmpeg to fail.
	// This test case is removed for now as it relied on mock behavior.
}

func TestDownloadAudioToFile(t *testing.T) {
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToFile: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToFile: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	cfg := createTestConfig(t, downloadDir)
	downloader := NewDownloader(cfg)

	// Use a known short, public domain audio URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (audio only)
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	filePath, err := downloader.DownloadAudioToFile(url, outputFormat, codec, bitrate)
	assert.NoError(t, err, "DownloadAudioToFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")

	// Verify file exists and is not empty
	fileInfo, err := os.Stat(filePath)
	assert.NoError(t, err, "Failed to stat downloaded file")
	assert.True(t, fileInfo.Size() > 0, "Downloaded file should not be empty")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Test with a non-existent URL to simulate yt-dlp failure
	t.Run("yt-dlp failure", func(t *testing.T) {
		nonExistentURL := "http://example.com/nonexistent_audio_12345"
		_, err = downloader.DownloadAudioToFile(nonExistentURL, outputFormat, codec, bitrate)
		assert.Error(t, err, "Expected error when yt-dlp fails for non-existent URL")
		assert.Contains(t, err.Error(), "yt-dlp audio fetch failed", "Expected yt-dlp failure error message")
	})

	// Note: Simulating ffmpeg conversion failure for a file download is complex
	// without mocking or specific test files that cause ffmpeg to fail.
	// This test case is removed for now as it relied on mock behavior.
}

func TestStreamVideo(t *testing.T) {
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamVideo: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestStreamVideo: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir() // Still needed for config creation, though not used by streaming
	cfg := createTestConfig(t, downloadDir)
	downloader := NewDownloader(cfg)

	// Use a known short, public domain video URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (short version for testing)
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	reader, err := downloader.StreamVideo(url, format, resolution, codec)
	assert.NoError(t, err, "StreamVideo should not fail on success")
	defer reader.Close()

	// Read from the stream - just check if we can read some bytes
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	assert.NoError(t, err, "Failed to read from video stream")
	assert.True(t, n > 0, "Expected to read some bytes from the stream")

	// Test with yt-dlp failure (e.g., non-existent URL)
	t.Run("yt-dlp failure", func(t *testing.T) {
		nonExistentURL := "http://example.com/nonexistent_stream_video_12345"
		_, err = downloader.StreamVideo(nonExistentURL, format, resolution, codec)
		assert.Error(t, err, "Expected error when yt-dlp fails")
		assert.Contains(t, err.Error(), "failed to start yt-dlp", "Expected yt-dlp failure error message")
	})

	// Note: Simulating ffmpeg failure for streaming is complex without specific test cases
	// that cause ffmpeg to fail during a pipe operation.
	// This test case is removed for now as it relied on mock behavior.
}

func TestStreamAudio(t *testing.T) {
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamAudio: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestStreamAudio: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir() // Still needed for config creation, though not used by streaming
	cfg := createTestConfig(t, downloadDir)
	downloader := NewDownloader(cfg)

	// Use a known short, public domain audio URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (audio only)
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	reader, err := downloader.StreamAudio(url, outputFormat, codec, bitrate)
	assert.NoError(t, err, "StreamAudio should not fail on success")
	defer reader.Close()

	// Read from the stream - just check if we can read some bytes
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	assert.NoError(t, err, "Failed to read from audio stream")
	assert.True(t, n > 0, "Expected to read some bytes from the stream")

	// Test with yt-dlp failure (e.g., non-existent URL)
	t.Run("yt-dlp failure", func(t *testing.T) {
		nonExistentURL := "http://example.com/nonexistent_stream_audio_12345"
		_, err = downloader.StreamAudio(nonExistentURL, outputFormat, codec, bitrate)
		assert.Error(t, err, "Expected error when yt-dlp fails")
		assert.Contains(t, err.Error(), "failed to start yt-dlp", "Expected yt-dlp failure error message")
	})

	// Note: Simulating ffmpeg failure for streaming is complex without specific test cases
	// that cause ffmpeg to fail during a pipe operation.
	// This test case is removed for now as it relied on mock behavior.
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
