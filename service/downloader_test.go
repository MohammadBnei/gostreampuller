package service

import (
	"context" // Import context package
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gostreampuller/config"
)

// createTestConfig creates a config.Config for testing.
// It ensures that real yt-dlp and ffmpeg executables are used if available in PATH,
// or relies on the system's default behavior for finding them.
func createTestConfig(downloadDir string) *config.Config {
	return &config.Config{
		DownloadDir: downloadDir,
		LocalMode:   true,
		DebugMode:   true,
		YTDLPPath:   "yt-dlp",
		FFMPEGPath:  "ffmpeg",
	}
}

// createTestDownloader creates a Downloader instance for testing, with a dummy ProgressManager.
func createTestDownloader(t *testing.T, downloadDir string) *Downloader {
	cfg := createTestConfig(downloadDir)
	// Create a dummy ProgressManager for tests
	pm := NewProgressManager()
	return NewDownloader(cfg, pm)
}

func TestDownloadVideoToFile_Success(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToFile_Success: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToFile_Success: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	// Use a known short, public domain video URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (short version for testing)
	format := "mp4"
	resolution := "360" // Use a lower resolution for faster downloads
	codec := "avc1"     // Common video codec

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filePath, videoInfo, err := downloader.DownloadVideoToFile(ctx, url, format, resolution, codec, "") // Pass empty progressID
	assert.NoError(t, err, "DownloadVideoToFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")
	assert.NotNil(t, videoInfo, "Returned VideoInfo should not be nil")

	// Verify VideoInfo content (these values are specific to the test URL)
	assert.Equal(t, "dQw4w9WgXcQ", videoInfo.ID)
	assert.Equal(t, "Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)", videoInfo.Title)
	assert.Contains(t, videoInfo.OriginalURL, "youtube.com/watch?v=dQw4w9WgXcQ")
	assert.True(t, videoInfo.Duration > 0, "Duration should be greater than 0")
	assert.NotEmpty(t, videoInfo.Uploader, "Uploader should not be empty")
	assert.NotEmpty(t, videoInfo.UploadDate, "UploadDate should not be empty")
	assert.NotEmpty(t, videoInfo.Thumbnail, "Thumbnail should not be empty")

	// Verify file exists and is not empty
	fileInfo, err := os.Stat(filePath)
	assert.NoError(t, err, "Failed to stat downloaded file")
	assert.True(t, fileInfo.Size() > 0, "Downloaded file should not be empty")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Verify filename format (timestamp-id.ext)
	filename := filepath.Base(filePath)
	parts := strings.Split(filename, "-")
	assert.Len(t, parts, 2, "Filename should have two parts separated by '-'")
	_, err = time.ParseDuration(parts[0] + "ns") // Check if first part is a valid timestamp
	assert.NoError(t, err, "First part of filename should be a timestamp")
	assert.True(t, strings.HasSuffix(parts[1], "."+format), "Filename should end with requested format extension")
}

func TestDownloadVideoToFile_YTDLPFailure(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToFile_YTDLPFailure: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_video_12345"
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, err := downloader.DownloadVideoToFile(ctx, nonExistentURL, format, resolution, codec, "") // Pass empty progressID
	assert.Error(t, err, "Expected error when yt-dlp fails for non-existent URL")
	assert.Contains(t, err.Error(), "yt-dlp info dump failed", "Expected yt-dlp info dump failure error message")
}

func TestDownloadVideoToFile_ContextCancellation(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToFile_ContextCancellation: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctxCancel, cancelCancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancelCancel()

	_, _, err := downloader.DownloadVideoToFile(ctxCancel, url, format, resolution, codec, "") // Pass empty progressID
	assert.Error(t, err, "Expected error due to context cancellation")
	assert.Contains(t, err.Error(), "context canceled", "Expected context canceled error message")
}

func TestDownloadAudioToFile_Success(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToFile_Success: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToFile_Success: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	// Use a known short, public domain audio URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (audio only)
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filePath, videoInfo, err := downloader.DownloadAudioToFile(ctx, url, outputFormat, codec, bitrate, "") // Pass empty progressID
	assert.NoError(t, err, "DownloadAudioToFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")
	assert.NotNil(t, videoInfo, "Returned VideoInfo should not be nil")

	// Verify VideoInfo content (these values are specific to the test URL)
	assert.Equal(t, "dQw4w9WgXcQ", videoInfo.ID)
	assert.Equal(t, "Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)", videoInfo.Title)
	assert.Contains(t, videoInfo.OriginalURL, "youtube.com/watch?v=dQw4w9WgXcQ")
	assert.True(t, videoInfo.Duration > 0, "Duration should be greater than 0")
	assert.NotEmpty(t, videoInfo.Uploader, "Uploader should not be empty")
	assert.NotEmpty(t, videoInfo.UploadDate, "UploadDate should not be empty")
	assert.NotEmpty(t, videoInfo.Thumbnail, "Thumbnail should not be empty")

	// Verify file exists and is not empty
	fileInfo, err := os.Stat(filePath)
	assert.NoError(t, err, "Failed to stat downloaded file")
	assert.True(t, fileInfo.Size() > 0, "Downloaded file should not be empty")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Verify filename format (timestamp-id.ext)
	filename := filepath.Base(filePath)
	parts := strings.Split(filename, "-")
	assert.Len(t, parts, 2, "Filename should have two parts separated by '-'")
	_, err = time.ParseDuration(parts[0] + "ns") // Check if first part is a valid timestamp
	assert.NoError(t, err, "First part of filename should be a timestamp")
	assert.True(t, strings.HasSuffix(parts[1], "."+outputFormat), "Filename should end with requested format extension")
}

func TestDownloadAudioToFile_YTDLPFailure(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToFile_YTDLPFailure: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_audio_12345"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, err := downloader.DownloadAudioToFile(ctx, nonExistentURL, outputFormat, codec, bitrate, "") // Pass empty progressID
	assert.Error(t, err, "Expected error when yt-dlp fails for non-existent URL")
	assert.Contains(t, err.Error(), "yt-dlp info dump failed", "Expected yt-dlp info dump failure error message")
}

func TestDownloadAudioToFile_ContextCancellation(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToFile_ContextCancellation: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctxCancel, cancelCancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancelCancel()

	_, _, err := downloader.DownloadAudioToFile(ctxCancel, url, outputFormat, codec, bitrate, "") // Pass empty progressID
	assert.Error(t, err, "Expected error due to context cancellation")
	assert.Contains(t, err.Error(), "context canceled", "Expected context canceled error message")
}

func TestStreamVideo_Success(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamVideo_Success: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestStreamVideo_Success: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir() // Still needed for config creation, though not used by streaming
	downloader := createTestDownloader(t, downloadDir)

	// Use a known short, public domain video URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (short version for testing)
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := downloader.StreamVideo(ctx, url, format, resolution, codec, "") // Pass empty progressID
	assert.NoError(t, err, "StreamVideo should not fail on success")
	defer reader.Close()

	// Read from the stream - just check if we can read some bytes
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	assert.NoError(t, err, "Failed to read from video stream")
	assert.True(t, n > 0, "Expected to read some bytes from the stream")
}

func TestStreamVideo_YTDLPFailure(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamVideo_YTDLPFailure: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_stream_video_12345"
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := downloader.StreamVideo(ctx, nonExistentURL, format, resolution, codec, "") // Pass empty progressID
	// StreamVideo itself might not return an error immediately if yt-dlp starts but then fails.
	// The error will be propagated when reading from or closing the stream.
	assert.NoError(t, err, "StreamVideo should not return an error on initial call for runtime failure")

	// Attempt to read from the stream to trigger the underlying process's execution and potential failure
	buf := make([]byte, 1024)
	_, readErr := reader.Read(buf)
	assert.Error(t, readErr, "Expected error when reading from stream due to yt-dlp failure")

	// Close the reader to ensure the command's exit status is checked
	closeErr := reader.Close()
	assert.Error(t, closeErr, "Expected error when closing stream due to yt-dlp failure")
	assert.Contains(t, closeErr.Error(), "command exited with error", "Expected command exited with error message")
}

func TestStreamVideo_ContextCancellation(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamVideo_ContextCancellation: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctxCancel, cancelCancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancelCancel()

	reader, err := downloader.StreamVideo(ctxCancel, url, format, resolution, codec, "") // Pass empty progressID
	// StreamVideo itself might return an error immediately if context is cancelled before start
	// or it might return a reader that will error on read/close.
	assert.Error(t, err, "Expected error due to context cancellation on initial call")
	assert.Contains(t, err.Error(), "context canceled", "Expected context canceled error message")

	// If reader was returned, ensure it's closed to avoid resource leaks
	if reader != nil {
		closeErr := reader.Close()
		assert.Error(t, closeErr, "Expected error when closing stream due to context cancellation")
		assert.Contains(t, closeErr.Error(), "context canceled", "Expected context canceled error message")
	}
}

func TestStreamAudio_Success(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	// Skip this test if yt-dlp or ffmpeg are not found in PATH
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamAudio_Success: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestStreamAudio_Success: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir() // Still needed for config creation, though not used by streaming
	downloader := createTestDownloader(t, downloadDir)

	// Use a known short, public domain audio URL for testing
	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Rick Astley - Never Gonna Give You Up (audio only)
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := downloader.StreamAudio(ctx, url, outputFormat, codec, bitrate, "") // Pass empty progressID
	assert.NoError(t, err, "StreamAudio should not fail on success")
	defer reader.Close()

	// Read from the stream - just check if we can read some bytes
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	assert.NoError(t, err, "Failed to read from audio stream")
	assert.True(t, n > 0, "Expected to read some bytes from the stream")
}

func TestStreamAudio_YTDLPFailure(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamAudio_YTDLPFailure: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_stream_audio_12345"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := downloader.StreamAudio(ctx, nonExistentURL, outputFormat, codec, bitrate, "") // Pass empty progressID
	// StreamAudio itself might not return an error immediately if yt-dlp starts but then fails.
	// The error will be propagated when reading from or closing the stream.
	assert.NoError(t, err, "StreamAudio should not return an error on initial call for runtime failure")

	// Attempt to read from the stream to trigger the underlying process's execution and potential failure
	buf := make([]byte, 1024)
	_, readErr := reader.Read(buf)
	assert.Error(t, readErr, "Expected error when reading from stream due to yt-dlp failure")

	// Close the reader to ensure the command's exit status is checked
	closeErr := reader.Close()
	assert.Error(t, closeErr, "Expected error when closing stream due to yt-dlp failure")
	assert.Contains(t, closeErr.Error(), "command exited with error", "Expected command exited with error message")
}

func TestStreamAudio_ContextCancellation(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestStreamAudio_ContextCancellation: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctxCancel, cancelCancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancelCancel()

	reader, err := downloader.StreamAudio(ctxCancel, url, outputFormat, codec, bitrate, "") // Pass empty progressID
	// StreamAudio itself might return an error immediately if context is cancelled before start
	// or it might return a reader that will error on read/close.
	assert.Error(t, err, "Expected error due to context cancellation on initial call")
	assert.Contains(t, err.Error(), "context canceled", "Expected context canceled error message")

	// If reader was returned, ensure it's closed to avoid resource leaks
	if reader != nil {
		closeErr := reader.Close()
		assert.Error(t, closeErr, "Expected error when closing stream due to context cancellation")
		assert.Contains(t, closeErr.Error(), "context canceled", "Expected context canceled error message")
	}
}

func TestDownloadVideoToTempFile_Success(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToTempFile_Success: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToTempFile_Success: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filePath, err := downloader.DownloadVideoToTempFile(ctx, url, format, resolution, codec, "") // Pass empty progressID
	assert.NoError(t, err, "DownloadVideoToTempFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")

	// Verify file exists and is not empty
	fileInfo, err := os.Stat(filePath)
	assert.NoError(t, err, "Failed to stat downloaded file")
	assert.True(t, fileInfo.Size() > 0, "Downloaded file should not be empty")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Clean up the file after test
	defer os.Remove(filePath)
}

func TestDownloadVideoToTempFile_YTDLPFailure(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadVideoToTempFile_YTDLPFailure: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_video_temp_12345"
	format := "mp4"
	resolution := "360"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filePath, err := downloader.DownloadVideoToTempFile(ctx, nonExistentURL, format, resolution, codec, "") // Pass empty progressID
	assert.Error(t, err, "Expected error when yt-dlp fails for non-existent URL")
	assert.Contains(t, err.Error(), "yt-dlp temp video download failed", "Expected yt-dlp temp video download failure error message")
	assert.Empty(t, filePath, "File path should be empty on failure")
}

func TestDownloadAudioToTempFile_Success(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToTempFile_Success: yt-dlp not found in PATH (%v)", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToTempFile_Success: ffmpeg not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filePath, err := downloader.DownloadAudioToTempFile(ctx, url, outputFormat, codec, bitrate, "") // Pass empty progressID
	assert.NoError(t, err, "DownloadAudioToTempFile should not fail on success")
	assert.NotEmpty(t, filePath, "Returned file path should not be empty")

	// Verify file exists and is not empty
	fileInfo, err := os.Stat(filePath)
	assert.NoError(t, err, "Failed to stat downloaded file")
	assert.True(t, fileInfo.Size() > 0, "Downloaded file should not be empty")

	// Verify file is in the correct directory
	assert.Equal(t, downloadDir, filepath.Dir(filePath), "File downloaded to wrong directory")

	// Clean up the file after test
	defer os.Remove(filePath)
}

func TestDownloadAudioToTempFile_YTDLPFailure(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestDownloadAudioToTempFile_YTDLPFailure: yt-dlp not found in PATH (%v)", err)
	}
	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_audio_temp_12345"
	outputFormat := "mp3"
	codec := "libmp3lame"
	bitrate := "128k"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filePath, err := downloader.DownloadAudioToTempFile(ctx, nonExistentURL, outputFormat, codec, bitrate, "") // Pass empty progressID
	assert.Error(t, err, "Expected error when yt-dlp fails for non-existent URL")
	assert.Contains(t, err.Error(), "yt-dlp temp audio download failed", "Expected yt-dlp temp audio download failure error message")
	assert.Empty(t, filePath, "File path should be empty on failure")
}

func TestGetVideoInfo_Success(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestGetVideoInfo_Success: yt-dlp not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	videoInfo, err := downloader.GetVideoInfo(ctx, url, "") // Pass empty progressID
	assert.NoError(t, err, "GetVideoInfo should not fail on success")
	assert.NotNil(t, videoInfo, "VideoInfo should not be nil")
	assert.Equal(t, "dQw4w9WgXcQ", videoInfo.ID)
	assert.Contains(t, videoInfo.Title, "Rick Astley")
}

func TestGetVideoInfo_Failure(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestGetVideoInfo_Failure: yt-dlp not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_info_12345"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	videoInfo, err := downloader.GetVideoInfo(ctx, nonExistentURL, "") // Pass empty progressID
	assert.Error(t, err, "Expected error for non-existent URL")
	assert.Nil(t, videoInfo, "VideoInfo should be nil on failure")
	assert.Contains(t, err.Error(), "yt-dlp info dump failed")
}

func TestGetStreamInfo_Success(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestGetStreamInfo_Success: yt-dlp not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	url := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	resolution := "720"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamInfo, err := downloader.GetStreamInfo(ctx, url, resolution, codec, "") // Pass empty progressID
	assert.NoError(t, err, "GetStreamInfo should not fail on success")
	assert.NotNil(t, streamInfo, "StreamInfo should not be nil")
	assert.NotEmpty(t, streamInfo.DirectStreamURL, "DirectStreamURL should not be empty")
	assert.Contains(t, streamInfo.Title, "Rick Astley")
}

func TestGetStreamInfo_Failure(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skipf("Skipping TestGetStreamInfo_Failure: yt-dlp not found in PATH (%v)", err)
	}

	downloadDir := t.TempDir()
	downloader := createTestDownloader(t, downloadDir)

	nonExistentURL := "http://example.com/nonexistent_stream_info_12345"
	resolution := "720"
	codec := "avc1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamInfo, err := downloader.GetStreamInfo(ctx, nonExistentURL, resolution, codec, "") // Pass empty progressID
	assert.Error(t, err, "Expected error for non-existent URL")
	assert.Nil(t, streamInfo, "StreamInfo should be nil on failure")
	assert.Contains(t, err.Error(), "yt-dlp stream info dump failed")
}

func TestCommandReadCloserClose(t *testing.T) {
	t.Parallel() // Enable parallel execution for this test
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
