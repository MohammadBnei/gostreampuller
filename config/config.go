package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
)

// Config holds all application configuration.
type Config struct {
	Port         string
	AuthUsername string
	AuthPassword string
	DebugMode    bool
	LocalMode    bool // When true, bypasses authentication for local testing
	MaxRetries   int  // Maximum number of retries for search requests
	RetryBackoff int  // Initial backoff in milliseconds (doubles with each retry)
}

// New creates a new Config with values from environment variables.
// Returns an error if required authentication credentials are missing.
func New() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Debug mode defaults to false
	debugMode := os.Getenv("DEBUG") == "true"

	// Local mode defaults to false
	localMode := os.Getenv("LOCAL_MODE") == "true"

	if localMode {
		slog.Warn("Running in LOCAL_MODE - authentication is disabled")
	}

	// Set default retry values
	maxRetries := 3
	retryBackoff := 500 // milliseconds

	// Override with environment variables if provided
	if maxRetriesStr := os.Getenv("MAX_RETRIES"); maxRetriesStr != "" {
		if val, err := strconv.Atoi(maxRetriesStr); err == nil {
			maxRetries = val
		}
	}

	if retryBackoffStr := os.Getenv("RETRY_BACKOFF"); retryBackoffStr != "" {
		if val, err := strconv.Atoi(retryBackoffStr); err == nil && val > 0 {
			retryBackoff = val
		}
	}

	// Only check auth credentials if not in local mode
	username := os.Getenv("AUTH_USERNAME")
	password := os.Getenv("AUTH_PASSWORD")

	if !localMode {
		if username == "" {
			return nil, errors.New("AUTH_USERNAME environment variable not set")
		}

		if password == "" {
			return nil, errors.New("AUTH_PASSWORD environment variable not set")
		}
	}

	// Configure global logger based on debug mode
	logLevel := slog.LevelInfo
	if debugMode {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	return &Config{
		Port:         port,
		AuthUsername: username,
		AuthPassword: password,
		DebugMode:    debugMode,
		LocalMode:    localMode,
		MaxRetries:   maxRetries,
		RetryBackoff: retryBackoff,
	}, nil
}
