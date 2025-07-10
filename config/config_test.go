package config

import (
	"os"
	"testing"
)

func TestLocalMode(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalUsername := os.Getenv("AUTH_USERNAME")
	originalPassword := os.Getenv("AUTH_PASSWORD")
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("AUTH_USERNAME", originalUsername)
		os.Setenv("AUTH_PASSWORD", originalPassword)
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
	}()

	// Set auth credentials for non-local mode tests
	os.Setenv("AUTH_USERNAME", "testuser")
	os.Setenv("AUTH_PASSWORD", "testpass")

	// Test when LOCAL_MODE is not set
	os.Unsetenv("LOCAL_MODE")
	cfg, err := New()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if cfg.LocalMode {
		t.Error("LocalMode should be false when LOCAL_MODE env var is not set")
	}

	// Test when LOCAL_MODE is set to true
	os.Setenv("LOCAL_MODE", "true")
	cfg, err = New()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !cfg.LocalMode {
		t.Error("LocalMode should be true when LOCAL_MODE env var is set to 'true'")
	}

	// Test when LOCAL_MODE is set to something else
	os.Setenv("LOCAL_MODE", "yes")
	cfg, err = New()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if cfg.LocalMode {
		t.Error("LocalMode should be false when LOCAL_MODE env var is not 'true'")
	}
}

func TestAuthCredentials(t *testing.T) {
	// Clear environment variables for this test
	t.Setenv("AUTH_USERNAME", "")
	t.Setenv("AUTH_PASSWORD", "")
	t.Setenv("LOCAL_MODE", "")

	// Test missing username in non-local mode
	t.Setenv("AUTH_PASSWORD", "testpass")

	_, err := New()
	if err == nil {
		t.Error("Expected error for missing username in non-local mode")
	}

	// Test missing password in non-local mode
	t.Setenv("AUTH_USERNAME", "testuser")
	t.Setenv("AUTH_PASSWORD", "")

	_, err = New()
	if err == nil {
		t.Error("Expected error for missing password in non-local mode")
	}

	// Test local mode with missing credentials (should not error)
	t.Setenv("LOCAL_MODE", "true")

	cfg, err := New()
	if err != nil {
		t.Errorf("Unexpected error in local mode: %v", err)
	}
	if !cfg.LocalMode {
		t.Error("LocalMode should be true")
	}
}

func TestYTDLPAndFFMPEGPaths(t *testing.T) {
	// Save original env vars to restore later
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalLocalMode := os.Getenv("LOCAL_MODE")

	defer func() {
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("LOCAL_MODE", originalLocalMode)
	}()

	// Set local mode to bypass auth for these tests
	os.Setenv("LOCAL_MODE", "true")

	// Test default values
	t.Run("DefaultPaths", func(t *testing.T) {
		os.Unsetenv("YTDLP_PATH")
		os.Unsetenv("FFMPEG_PATH")

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		if cfg.YTDLPPath != "yt-dlp" {
			t.Errorf("Expected default YTDLPPath to be 'yt-dlp', got '%s'", cfg.YTDLPPath)
		}

		if cfg.FFMPEGPath != "ffmpeg" {
			t.Errorf("Expected default FFMPEGPath to be 'ffmpeg', got '%s'", cfg.FFMPEGPath)
		}
	})

	// Test custom values
	t.Run("CustomPaths", func(t *testing.T) {
		os.Setenv("YTDLP_PATH", "/usr/local/bin/yt-dlp-custom")
		os.Setenv("FFMPEG_PATH", "/opt/ffmpeg/bin/ffmpeg-custom")

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		if cfg.YTDLPPath != "/usr/local/bin/yt-dlp-custom" {
			t.Errorf("Expected YTDLPPath to be '/usr/local/bin/yt-dlp-custom', got '%s'", cfg.YTDLPPath)
		}

		if cfg.FFMPEGPath != "/opt/ffmpeg/bin/ffmpeg-custom" {
			t.Errorf("Expected FFMPEGPath to be '/opt/ffmpeg/bin/ffmpeg-custom', got '%s'", cfg.FFMPEGPath)
		}
	})
}
