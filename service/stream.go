package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"gostreampuller/config"
)

// Streamer provides functionality to proxy video and audio streams.
type Streamer struct {
	cfg        *config.Config
	downloader *Downloader // To get stream info
}

// NewStreamer creates a new Streamer instance.
func NewStreamer(cfg *config.Config, downloader *Downloader) *Streamer {
	return &Streamer{
		cfg:        cfg,
		downloader: downloader,
	}
}

// ProxyVideo proxies a video stream from its direct URL to the http.ResponseWriter.
// It handles Range requests for seeking.
func (s *Streamer) ProxyVideo(ctx context.Context, w http.ResponseWriter, r *http.Request, videoURL string, resolution string, codec string) error {
	slog.Info("Attempting to proxy video stream", "url", videoURL, "resolution", resolution, "codec", codec)

	// Get detailed stream info to find the best direct URL
	streamInfo, err := s.downloader.GetStreamInfo(ctx, videoURL, resolution, codec)
	if err != nil {
		return fmt.Errorf("failed to get stream info for proxy: %w", err)
	}

	if streamInfo.DirectStreamURL == "" {
		return fmt.Errorf("no direct stream URL found for video: %s", videoURL)
	}

	targetURL, err := url.Parse(streamInfo.DirectStreamURL)
	if err != nil {
		return fmt.Errorf("invalid direct stream URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom director to modify the request before sending it to the target
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req) // Call the original director first

		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = targetURL.Path
		req.URL.RawQuery = targetURL.RawQuery
		req.Host = targetURL.Host // Important for some CDNs

		// Copy Range header from client request to proxy request
		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
			slog.Debug("Proxying with Range header", "range", rangeHeader)
		}

		// Remove headers that might cause issues or are not needed
		req.Header.Del("If-Modified-Since")
		req.Header.Del("If-None-Match")
		req.Header.Del("Accept-Encoding") // Prevent double compression
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36") // Mimic a browser
	}

	// Custom transport to modify the response before sending it to the client
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// Add other transport settings if needed, e.g., TLSClientConfig
	}

	// Serve the proxy request
	proxy.ServeHTTP(w, r)

	slog.Info("Successfully proxied video stream", "originalURL", videoURL, "directURL", streamInfo.DirectStreamURL)
	return nil
}

// ProxyAudio proxies an audio stream from its direct URL to the http.ResponseWriter.
// It handles Range requests for seeking.
func (s *Streamer) ProxyAudio(ctx context.Context, w http.ResponseWriter, r *http.Request, audioURL string) error {
	slog.Info("Attempting to proxy audio stream", "url", audioURL)

	// Get detailed stream info to find the best direct URL for audio
	// For audio, we might not need resolution/codec, but GetStreamInfo can still help find the best audio-only format.
	// We'll call GetStreamInfo and then iterate through formats to find an audio-only one.
	streamInfo, err := s.downloader.GetStreamInfo(ctx, audioURL, "", "") // Pass empty resolution/codec for audio
	if err != nil {
		return fmt.Errorf("failed to get stream info for audio proxy: %w", err)
	}

	var bestAudioFormat *VideoInfo
	// Find the best audio-only format
	for _, f := range streamInfo.Formats {
		if f.DirectStreamURL != "" && f.ACodec != "none" && f.VCodec == "none" { // Audio only
			if bestAudioFormat == nil || f.FileSize > bestAudioFormat.FileSize { // Simple heuristic: largest file size
				bestAudioFormat = &f
			}
		}
	}

	if bestAudioFormat == nil || bestAudioFormat.DirectStreamURL == "" {
		return fmt.Errorf("no suitable direct stream URL found for audio: %s", audioURL)
	}

	targetURL, err := url.Parse(bestAudioFormat.DirectStreamURL)
	if err != nil {
		return fmt.Errorf("invalid direct stream URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = targetURL.Path
		req.URL.RawQuery = targetURL.RawQuery
		req.Host = targetURL.Host

		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
			slog.Debug("Proxying audio with Range header", "range", rangeHeader)
		}

		req.Header.Del("If-Modified-Since")
		req.Header.Del("If-None-Match")
		req.Header.Del("Accept-Encoding")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36")
	}

	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	proxy.ServeHTTP(w, r)

	slog.Info("Successfully proxied audio stream", "originalURL", audioURL, "directURL", bestAudioFormat.DirectStreamURL)
	return nil
}
