package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalMode(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalUsername := os.Getenv("AUTH_USERNAME")
	originalPassword := os.Getenv("AUTH_PASSWORD")
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")
	originalAppURL := os.Getenv("APP_URL") // Save AppURL

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("AUTH_USERNAME", originalUsername)
		os.Setenv("AUTH_PASSWORD", originalPassword)
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
		os.Setenv("APP_URL", originalAppURL) // Restore AppURL
	}()

	// Set auth credentials for non-local mode tests
	os.Setenv("AUTH_USERNAME", "testuser")
	os.Setenv("AUTH_PASSWORD", "testpass")
	// Set dummy paths for executables to pass checks
	os.Setenv("YTDLP_PATH", "echo")
	os.Setenv("FFMPEG_PATH", "echo")
	// Set a dummy AppURL
	os.Setenv("APP_URL", "http://test.com")

	// Test when LOCAL_MODE is not set
	os.Unsetenv("LOCAL_MODE")
	cfg, err := New()
	assert.NoError(t, err)
	assert.False(t, cfg.LocalMode, "LocalMode should be false when LOCAL_MODE env var is not set")

	// Test when LOCAL_MODE is set to true
	os.Setenv("LOCAL_MODE", "true")
	cfg, err = New()
	assert.NoError(t, err)
	assert.True(t, cfg.LocalMode, "LocalMode should be true when LOCAL_MODE env var is set to 'true'")

	// Test when LOCAL_MODE is set to something else
	os.Setenv("LOCAL_MODE", "yes")
	cfg, err = New()
	assert.NoError(t, err)
	assert.False(t, cfg.LocalMode, "LocalMode should be false when LOCAL_MODE env var is not 'true'")
}

func TestAuthCredentials(t *testing.T) {
	// Clear environment variables for this test
	t.Setenv("AUTH_USERNAME", "")
	t.Setenv("AUTH_PASSWORD", "")
	t.Setenv("LOCAL_MODE", "")
	// Set dummy paths for executables to pass checks
	t.Setenv("YTDLP_PATH", "echo")
	t.Setenv("FFMPEG_PATH", "echo")
	// Set a dummy AppURL
	t.Setenv("APP_URL", "http://test.com")

	// Test missing username in non-local mode
	t.Setenv("AUTH_PASSWORD", "testpass")

	_, err := New()
	assert.Error(t, err, "Expected error for missing username in non-local mode")
	assert.Contains(t, err.Error(), "AUTH_USERNAME environment variable not set")

	// Test missing password in non-local mode
	t.Setenv("AUTH_USERNAME", "testuser")
	t.Setenv("AUTH_PASSWORD", "")

	_, err = New()
	assert.Error(t, err, "Expected error for missing password in non-local mode")
	assert.Contains(t, err.Error(), "AUTH_PASSWORD environment variable not set")

	// Test local mode with missing credentials (should not error)
	t.Setenv("LOCAL_MODE", "true")

	cfg, err := New()
	assert.NoError(t, err, "Unexpected error in local mode")
	assert.True(t, cfg.LocalMode, "LocalMode should be true")
}

func TestYTDLPAndFFMPEGPaths(t *testing.T) {
	// Save original env vars to restore later
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")
	originalAppURL := os.Getenv("APP_URL") // Save AppURL

	defer func() {
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
		os.Setenv("APP_URL", originalAppURL) // Restore AppURL
	}()

	// Set local mode to bypass auth for these tests
	os.Setenv("LOCAL_MODE", "true")
	// Set a temporary download directory for these tests
	tempDir := t.TempDir()
	os.Setenv("DOWNLOAD_DIR", tempDir)
	// Set a dummy AppURL
	os.Setenv("APP_URL", "http://test.com")

	// Test default values
	t.Run("DefaultPaths", func(t *testing.T) {
		os.Unsetenv("YTDLP_PATH")
		os.Unsetenv("FFMPEG_PATH")

		cfg, err := New()
		assert.NoError(t, err, "Failed to create config with default paths")

		assert.Equal(t, "yt-dlp", cfg.YTDLPPath, "Expected default YTDLPPath to be 'yt-dlp'")
		assert.Equal(t, "ffmpeg", cfg.FFMPEGPath, "Expected default FFMPEGPath to be 'ffmpeg'")
	})

	// Test custom values
	t.Run("CustomPaths", func(t *testing.T) {
		os.Setenv("YTDLP_PATH", "/usr/local/bin/yt-dlp-custom")
		os.Setenv("FFMPEG_PATH", "/opt/ffmpeg/bin/ffmpeg-custom")

		_, err := New()
		assert.Error(t, err, "Failed to create config with custom paths")
	})
}

func TestDownloadDir(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")
	originalAppURL := os.Getenv("APP_URL") // Save AppURL

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
		os.Setenv("APP_URL", originalAppURL) // Restore AppURL
	}()

	// Set local mode to bypass auth for these tests
	os.Setenv("LOCAL_MODE", "true")
	// Set dummy paths for executables to pass checks
	os.Setenv("YTDLP_PATH", "echo")
	os.Setenv("FFMPEG_PATH", "echo")
	// Set a dummy AppURL
	os.Setenv("APP_URL", "http://test.com")

	t.Run("DefaultDownloadDir", func(t *testing.T) {
		os.Unsetenv("DOWNLOAD_DIR") // Ensure default is used
		cfg, err := New()
		assert.NoError(t, err, "Failed to create config with default download dir")

		expectedDir, _ := filepath.Abs("./data") // Updated default
		assert.Equal(t, expectedDir, cfg.DownloadDir, "Expected default DownloadDir to be './data'")

		// Verify it's writable
		testFile := filepath.Join(cfg.DownloadDir, ".test_write_default")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		assert.NoError(t, err, "Default download directory should be writable")
		os.Remove(testFile)
	})

	t.Run("CustomDownloadDir", func(t *testing.T) {
		tempDir := t.TempDir() // Create a temporary directory for the test
		os.Setenv("DOWNLOAD_DIR", tempDir)

		cfg, err := New()
		assert.NoError(t, err, "Failed to create config with custom download dir")

		expectedDir, _ := filepath.Abs(tempDir)
		assert.Equal(t, expectedDir, cfg.DownloadDir, "Expected DownloadDir to be custom temp directory")

		// Verify the directory was created and is writable
		info, err := os.Stat(cfg.DownloadDir)
		assert.NoError(t, err, "Custom download directory should exist")
		assert.True(t, info.IsDir(), "Custom download path should be a directory")

		testFile := filepath.Join(cfg.DownloadDir, ".test_write_custom")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		assert.NoError(t, err, "Custom download directory should be writable")
		os.Remove(testFile)
	})

	t.Run("InvalidDownloadDir", func(t *testing.T) {
		// Set a path that is likely not writable or creatable
		os.Setenv("DOWNLOAD_DIR", "/root/nonexistent/path/that/should/fail") // On most systems, /root is not writable by non-root
		_, err := New()
		assert.Error(t, err, "Expected an error for an invalid/unwritable download directory")
		assert.Contains(t, err.Error(), "failed to create download directory", "Error message should indicate directory creation/write issue")
	})
}

func TestAppURL(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")
	originalAppURL := os.Getenv("APP_URL")

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
		os.Setenv("APP_URL", originalAppURL)
	}()

	// Set local mode to bypass auth for these tests
	os.Setenv("LOCAL_MODE", "true")
	// Set dummy paths for executables to pass checks
	os.Setenv("YTDLP_PATH", "echo")
	os.Setenv("FFMPEG_PATH", "echo")
	// Set a temporary download directory for these tests
	tempDir := t.TempDir()
	os.Setenv("DOWNLOAD_DIR", tempDir)

	t.Run("DefaultAppURL", func(t *testing.T) {
		os.Unsetenv("APP_URL") // Ensure default is used
		cfg, err := New()
		assert.NoError(t, err, "Failed to create config with default AppURL")
		assert.Equal(t, "http://localhost:8080", cfg.AppURL, "Expected default AppURL to be 'http://localhost:8080'")
	})

	t.Run("CustomAppURL", func(t *testing.T) {
		customURL := "https://my.custom.domain/gsp"
		os.Setenv("APP_URL", customURL)
		cfg, err := New()
		assert.NoError(t, err, "Failed to create config with custom AppURL")
		assert.Equal(t, customURL, cfg.AppURL, "Expected AppURL to be the custom value")
	})

	t.Run("EmptyAppURL", func(t *testing.T) {
		os.Setenv("APP_URL", "") // Test empty string, should fall back to default
		cfg, err := New()
		assert.NoError(t, err, "Failed to create config with empty AppURL")
		assert.Equal(t, "http://localhost:8080", cfg.AppURL, "Expected AppURL to fall back to default when empty")
	})
}
