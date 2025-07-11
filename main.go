package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gostreampuller/config"
	_ "gostreampuller/docs" // This line is necessary for Swagger to find the docs
	"gostreampuller/router"
)

//	@title			GoStreamPuller API
//	@version		1.0
//	@description	A lightweight, containerized REST API service that provides video and audio download and streaming functionalities using yt-dlp and ffmpeg.
//	@contact.name	API Support
//	@contact.url	http://www.example.com/support
//	@contact.email	support@example.com
//	@BasePath		/
//	@schemes		http
func main() {
	// Set up structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load configuration
	cfg, err := config.New()
	if err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	// Setup router
	r := router.New(cfg)

	// Configure server
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r.Handler(),
		ReadHeaderTimeout: 10 * time.Second, // Fix for G112: Potential Slowloris Attack
	}

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start server
	go func() {
		slog.Info(fmt.Sprintf("Server starting on port %s...", cfg.Port))
		if cfg.LocalMode {
			slog.Warn("LOCAL_MODE enabled: Authentication is bypassed")
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to listen", "error", err)
			os.Exit(1) // Exit if server fails to start
		}
	}()

	// Wait for shutdown signal
	<-stop

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("Shutting down server...")
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Server stopped")
}
