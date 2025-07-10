package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalMode(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalUsername := os.Getenv("AUTH_USERNAME")
	originalPassword := os.Getenv("AUTH_PASSWORD")
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("AUTH_USERNAME", originalUsername)
		os.Setenv("AUTH_PASSWORD", originalPassword)
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
	}()

	// Set auth credentials for non-local mode tests
	os.Setenv("AUTH_USERNAME", "testuser")
	os.Setenv("AUTH_PASSWORD", "testpass")
	// Set dummy paths for executables to pass checks
	os.Setenv("YTDLP_PATH", "echo")
	os.Setenv("FFMPEG_PATH", "echo")

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
	// Set dummy paths for executables to pass checks
	t.Setenv("YTDLP_PATH", "echo")
	t.Setenv("FFMPEG_PATH", "echo")

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
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")

	defer func() {
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
	}()

	// Set local mode to bypass auth for these tests
	os.Setenv("LOCAL_MODE", "true")
	// Set a temporary download directory for these tests
	tempDir := t.TempDir()
	os.Setenv("DOWNLOAD_DIR", tempDir)

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

func TestDownloadDir(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalYTDLPPath := os.Getenv("YTDLP_PATH")
	originalFFMPEGPath := os.Getenv("FFMPEG_PATH")
	originalDownloadDir := os.Getenv("DOWNLOAD_DIR")

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("YTDLP_PATH", originalYTDLPPath)
		os.Setenv("FFMPEG_PATH", originalFFMPEGPath)
		os.Setenv("DOWNLOAD_DIR", originalDownloadDir)
	}()

	// Set local mode to bypass auth for these tests
	os.Setenv("LOCAL_MODE", "true")
	// Set dummy paths for executables to pass checks
	os.Setenv("YTDLP_PATH", "echo")
	os.Setenv("FFMPEG_PATH", "echo")

	t.Run("DefaultDownloadDir", func(t *testing.T) {
		os.Unsetenv("DOWNLOAD_DIR") // Ensure default is used
		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config with default download dir: %v", err)
		}
		expectedDir, _ := filepath.Abs(".")
		if cfg.DownloadDir != expectedDir {
			t.Errorf("Expected default DownloadDir to be '%s', got '%s'", expectedDir, cfg.DownloadDir)
		}
		// Verify it's writable
		testFile := filepath.Join(cfg.DownloadDir, ".test_write_default")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Errorf("Default download directory '%s' is not writable: %v", cfg.DownloadDir, err)
		}
		os.Remove(testFile)
	})

	t.Run("CustomDownloadDir", func(t *testing.T) {
		tempDir := t.TempDir() // Create a temporary directory for the test
		os.Setenv("DOWNLOAD_DIR", tempDir)

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config with custom download dir: %v", err)
		}

		expectedDir, _ := filepath.Abs(tempDir)
		if cfg.DownloadDir != expectedDir {
			t.Errorf("Expected DownloadDir to be '%s', got '%s'", expectedDir, cfg.DownloadDir)
		}

		// Verify the directory was created and is writable
		info, err := os.Stat(cfg.DownloadDir)
		if err != nil {
			t.Fatalf("Custom download directory '%s' does not exist: %v", cfg.DownloadDir, err)
		}
		if !info.IsDir() {
			t.Errorf("Custom download path '%s' is not a directory", cfg.DownloadDir)
		}
		testFile := filepath.Join(cfg.DownloadDir, ".test_write_custom")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Errorf("Custom download directory '%s' is not writable: %v", cfg.DownloadDir, err)
		}
		os.Remove(testFile)
	})

	t.Run("InvalidDownloadDir", func(t *testing.T) {
		// Set a path that is likely not writable or creatable
		os.Setenv("DOWNLOAD_DIR", "/root/nonexistent/path/that/should/fail") // On most systems, /root is not writable by non-root
		_, err := New()
		if err == nil {
			t.Error("Expected an error for an invalid/unwritable download directory, but got none")
		}
	})
}
