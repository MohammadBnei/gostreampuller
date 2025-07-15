package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/num30/config" // Updated import
)

// Config holds all application configuration.
type Config struct {
	Port         string `env:"PORT" default:"8080"`
	AuthUsername string `default:""` // No longer using -1 as default for optional fields
	AuthPassword string `default:""` // No longer using -1 as default for optional fields
	DebugMode    bool   `env:"DEBUG" default:"false"`
	LocalMode    bool   `env:"LOCAL_MODE" default:"false"` // When true, bypasses authentication for local testing
	YTDLPPath    string `env:"YTDLP_PATH" default:"yt-dlp"`
	FFMPEGPath   string `env:"FFMPEG_PATH" default:"ffmpeg"`
	DownloadDir  string `env:"DOWNLOAD_DIR" default:"./data"` // Directory to store downloaded files
	AppURL       string `default:"http://localhost:8080"`     // Base URL of the application for redirects and external links
}

// New creates a new Config with values from environment variables.
// Returns an error if required authentication credentials are missing.
func New() (*Config, error) {
	var cfg Config
	// Use num30/config's NewConfReader and Read methods
	// No need for `env` tags if field names match env var names (case-insensitive, underscores for camelCase)
	// or if `envvar` tag is used. Default behavior is usually sufficient.
	err := config.NewConfReader("gostreampuller").Read(&cfg) // "gostreampuller" can be used as config file name prefix
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration: %w", err)
	}

	if cfg.LocalMode {
		slog.Warn("Running in LOCAL_MODE - authentication is disabled")
	}

	// Only check auth credentials if not in local mode
	if !cfg.LocalMode {
		if cfg.AuthUsername == "" { // Check for empty string now
			return nil, errors.New("AUTH_USERNAME environment variable not set")
		}

		if cfg.AuthPassword == "" { // Check for empty string now
			return nil, errors.New("AUTH_PASSWORD environment variable not set")
		}
	}

	// Verify yt-dlp and ffmpeg executables
	if err := checkExecutable(cfg.YTDLPPath, "yt-dlp", "--version"); err != nil {
		return nil, err
	}
	if err := checkExecutable(cfg.FFMPEGPath, "ffmpeg", "-version"); err != nil {
		return nil, err
	}

	// Verify and prepare download directory
	absDownloadDir, err := filepath.Abs(cfg.DownloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for download directory '%s': %w", cfg.DownloadDir, err)
	}
	cfg.DownloadDir = absDownloadDir // Update config with absolute path

	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory '%s': %w", cfg.DownloadDir, err)
	}

	// Check if directory is writable
	testFile := filepath.Join(cfg.DownloadDir, ".test_write")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return nil, fmt.Errorf("download directory '%s' is not writable: %w", cfg.DownloadDir, err)
	}
	os.Remove(testFile) // Clean up test file

	slog.Info(fmt.Sprintf("Download directory set to: %s", cfg.DownloadDir))

	// Configure global logger based on debug mode
	logLevel := slog.LevelInfo
	if cfg.DebugMode {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	return &cfg, nil
}

// checkExecutable verifies if an executable exists and is runnable.
func checkExecutable(path, name, versionCmd string) error {
	cmd := exec.Command(path, versionCmd) // Use --version to check if it's runnable
	if err := cmd.Run(); err != nil {
		// If the command fails, try to find it in PATH
		if _, err := exec.LookPath(path); err != nil {
			return fmt.Errorf("executable '%s' not found or not runnable at '%s': %w", name, path, err)
		}
		// If found in PATH but still not runnable with --version, it's a deeper issue
		return fmt.Errorf("executable '%s' found at '%s' but not runnable: %w", name, path, err)
	}
	slog.Info(fmt.Sprintf("Found %s executable at %s", name, path))
	return nil
}
