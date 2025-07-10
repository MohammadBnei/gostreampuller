package config

import (
	"errors"
	"log/slog"
	"os"

	"github.com/BoRuDar/configuration/v5"
)

// Config holds all application configuration.
type Config struct {
	Port         string `env:"PORT" default:"8080"`
	AuthUsername string `env:"AUTH_USERNAME"`
	AuthPassword string `env:"AUTH_PASSWORD"`
	DebugMode    bool   `env:"DEBUG" default:"false"`
	LocalMode    bool   `env:"LOCAL_MODE" default:"false"` // When true, bypasses authentication for local testing
	YTDLPPath    string `env:"YTDLP_PATH" default:"yt-dlp"`
	FFMPEGPath   string `env:"FFMPEG_PATH" default:"ffmpeg"`
}

// New creates a new Config with values from environment variables.
// Returns an error if required authentication credentials are missing.
func New() (*Config, error) {
	cfg, err := configuration.FromEnvAndDefault[Config]()
	if err != nil {
		return nil, err
	}

	if cfg.LocalMode {
		slog.Warn("Running in LOCAL_MODE - authentication is disabled")
	}

	// Only check auth credentials if not in local mode
	if !cfg.LocalMode {
		if cfg.AuthUsername == "" {
			return nil, errors.New("AUTH_USERNAME environment variable not set")
		}

		if cfg.AuthPassword == "" {
			return nil, errors.New("AUTH_PASSWORD environment variable not set")
		}
	}

	// Configure global logger based on debug mode
	logLevel := slog.LevelInfo
	if cfg.DebugMode {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	return cfg, nil
}
