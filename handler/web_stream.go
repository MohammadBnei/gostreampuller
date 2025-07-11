package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url" // Import net/url for URL parsing
	"strings"

	"gostreampuller/service"
)

// WebStreamHandler handles web-based video streaming requests.
type WebStreamHandler struct {
	downloader *service.Downloader // Still needed for GetVideoInfo
	streamer   *service.Streamer   // New field for streaming/proxying
	template   *template.Template
}

// NewWebStreamHandler creates a new WebStreamHandler.
func NewWebStreamHandler(downloader *service.Downloader, streamer *service.Streamer) *WebStreamHandler {
	// Parse the HTML template once during initialization
	tmpl, err := template.ParseFiles("web/stream.html")
	if err != nil {
		slog.Error("Failed to parse web stream template", "error", err)
		// In a real application, you might want to panic or handle this more gracefully
		// to prevent the server from starting with a broken template.
		panic(err)
	}
	return &WebStreamHandler{
		downloader: downloader,
		streamer:   streamer,
		template:   tmpl,
	}
}

// ServeStreamPage serves the HTML page with the video streaming form.
//
//	@Summary		Serve web streaming page
//	@Description	Serves an HTML page that allows users to input a URL and stream video.
//	@Tags			web
//	@Produce		html
//	@Success		200	{string}	html	"HTML page for video streaming"
//	@Router			/web [get]
func (h *WebStreamHandler) ServeStreamPage(w http.ResponseWriter, r *http.Request) {
	err := h.template.Execute(w, nil)
	if err != nil {
		slog.Error("Failed to execute web stream template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleWebStream handles the form submission for web-based video streaming.
// This function will fetch video info and then redirect to a streaming endpoint.
//
//	@Summary		Handle web stream request
//	@Description	Processes the form submission for web-based video streaming, fetches video info, and redirects to the stream.
//	@Tags			web
//	@Accept			x-www-form-urlencoded
//	@Produce		html
//	@Param			url			formData	string	true	"Video URL"
//	@Param			resolution	formData	string	false	"Video Resolution (e.g., 720, 1080)"
//	@Param			codec		formData	string	false	"Video Codec (e.g., avc1, vp9)"
//	@Param			action		formData	string	true	"Action to perform (stream, download_video, download_audio)"
//	@Success		200			{string}	html	"HTML page with streamed video and info"
//	@Failure		400			{string}	string	"Bad Request"
//	@Failure		500			{string}	string	"Internal Server Error"
//	@Router			/web [post]
func (h *WebStreamHandler) HandleWebStream(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form data", "error", err)
		http.Error(w, "Bad Request: Could not parse form", http.StatusBadRequest)
		return
	}

	videoURL := r.FormValue("url")
	resolution := r.FormValue("resolution")
	codec := r.FormValue("codec")
	action := r.FormValue("action") // Get the action from the clicked button

	if videoURL == "" {
		slog.Error("Missing URL in web stream request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// If the action is a direct download, redirect immediately
	// The JavaScript in web/stream.html handles this client-side for now,
	// but this server-side check is a good fallback/alternative.
	if action == "download_video" {
		http.Redirect(w, r, fmt.Sprintf("/web/download/video?url=%s&resolution=%s&codec=%s",
			url.QueryEscape(videoURL),
			url.QueryEscape(resolution),
			url.QueryEscape(codec)), http.StatusFound)
		return
	}
	if action == "download_audio" {
		http.Redirect(w, r, fmt.Sprintf("/web/download/audio?url=%s", url.QueryEscape(videoURL)), http.StatusFound)
		return
	}

	// If action is "stream" or not specified, proceed with streaming logic
	slog.Info("Attempting to get video info for web stream", "url", videoURL)

	// Get video info (using GetVideoInfo for general metadata)
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), videoURL)
	if err != nil {
		slog.Error("Failed to get video info for web stream", "error", err, "url", videoURL)
		http.Error(w, fmt.Sprintf("Failed to get video information: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare data for the template
	streamURL := fmt.Sprintf("/web/play?url=%s&resolution=%s&codec=%s",
		url.QueryEscape(videoURL),
		url.QueryEscape(resolution),
		url.QueryEscape(codec),
	)

	// Marshal videoInfo to pretty JSON for display
	videoInfoJSON, err := json.MarshalIndent(videoInfo, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal video info to JSON", "error", err)
		videoInfoJSON = []byte(fmt.Sprintf(`{"error": "Failed to format video info: %v"}`, err))
	}

	data := struct {
		StreamURL     string
		VideoInfoJSON template.HTML // Use template.HTML to prevent escaping
		VideoInfo     *service.VideoInfo
	}{
		StreamURL:     streamURL,
		VideoInfoJSON: template.HTML(videoInfoJSON),
		VideoInfo:     videoInfo,
	}

	// Re-execute the template with the stream URL and video info
	// This will render the same page but with the video player and info populated
	err = h.template.Execute(w, data)
	if err != nil {
		slog.Error("Failed to execute web stream template with data", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// PlayWebStream handles the actual video streaming for the web player.
// This is a GET endpoint that receives parameters from the HandleWebStream POST.
//
//	@Summary		Play web stream
//	@Description	Streams the video content directly to the browser based on query parameters.
//	@Tags			web
//	@Produce		video/mp4
//	@Param			url			query		string	true	"Video URL"
//	@Param			resolution	query		string	false	"Video Resolution (e.g., 720, 1080)"
//	@Param			codec		query		string	false	"Video Codec (e.g., avc1, vp9)"
//	@Success		200			{file}		file	"Successfully streamed video"
//	@Failure		400			{string}	string	"Bad Request"
//	@Failure		500			{string}	string	"Internal Server Error"
//	@Router			/web/play [get]
func (h *WebStreamHandler) PlayWebStream(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	resolution := r.URL.Query().Get("resolution")
	codec := r.URL.Query().Get("codec")

	if videoURL == "" {
		slog.Error("Missing URL in web stream play request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to proxy video for web player", "url", videoURL, "resolution", resolution, "codec", codec)

	// Use the Streamer service to proxy the video
	err := h.streamer.ProxyVideo(r.Context(), w, r, videoURL, resolution, codec)
	if err != nil {
		slog.Error("Failed to proxy video for web player", "error", err, "url", videoURL)
		http.Error(w, fmt.Sprintf("Failed to stream video: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Info("Web video stream finished", "url", videoURL)
}

// DownloadVideoToBrowser streams video directly to the browser for download.
//
//	@Summary		Download video to browser
//	@Description	Streams video content directly to the browser, triggering a download.
//	@Tags			web
//	@Produce		video/mp4
//	@Param			url			query		string	true	"Video URL"
//	@Param			resolution	query		string	false	"Video Resolution (e.g., 720, 1080)"
//	@Param			codec		query		string	false	"Video Codec (e.g., avc1, vp9)"
//	@Success		200			{file}		file	"Successfully streamed video for download"
//	@Failure		400			{string}	string	"Bad Request"
//	@Failure		500			{string}	string	"Internal Server Error"
//	@Router			/web/download/video [get]
func (h *WebStreamHandler) DownloadVideoToBrowser(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	resolution := r.URL.Query().Get("resolution")
	codec := r.URL.Query().Get("codec")

	if videoURL == "" {
		slog.Error("Missing URL in video download request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to proxy video for direct download", "url", videoURL, "resolution", resolution, "codec", codec)

	// Get video info to suggest a filename
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), videoURL)
	if err != nil {
		slog.Warn("Could not get video info for filename suggestion, proceeding without it", "error", err)
		videoInfo = &service.VideoInfo{Title: "video", Ext: "mp4"} // Fallback
	}

	// Set headers for download
	filename := fmt.Sprintf("%s.%s", sanitizeFilename(videoInfo.Title), "mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "video/mp4")
	// Transfer-Encoding and Cache-Control will be handled by the proxy

	// Use the Streamer service to proxy the video
	err = h.streamer.ProxyVideo(r.Context(), w, r, videoURL, resolution, codec)
	if err != nil {
		slog.Error("Failed to proxy video for direct download", "error", err, "url", videoURL)
		http.Error(w, fmt.Sprintf("Failed to download video: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Info("Direct video download stream finished", "url", videoURL)
}

// DownloadAudioToBrowser streams audio directly to the browser for download.
//
//	@Summary		Download audio to browser
//	@Description	Streams audio content directly to the browser, triggering a download.
//	@Tags			web
//	@Produce		audio/mpeg
//	@Param			url				query		string	true	"Audio URL"
//	@Param			outputFormat	query		string	false	"Output format (e.g., mp3, aac)"
//	@Param			codec			query		string	false	"Audio Codec (e.g., libmp3lame)"
//	@Param			bitrate			query		string	false	"Audio Bitrate (e.g., 128k)"
//	@Success		200				{file}		file	"Successfully streamed audio for download"
//	@Failure		400				{string}	string	"Bad Request"
//	@Failure		500				{string}	string	"Internal Server Error"
//	@Router			/web/download/audio [get]
func (h *WebStreamHandler) DownloadAudioToBrowser(w http.ResponseWriter, r *http.Request) {
	audioURL := r.URL.Query().Get("url")
	outputFormat := r.URL.Query().Get("outputFormat") // This is not used by ProxyAudio, but kept for Swagger
	// codec := r.URL.Query().Get("codec")               // This is not used by ProxyAudio, but kept for Swagger
	// bitrate := r.URL.Query().Get("bitrate")           // This is not used by ProxyAudio, but kept for Swagger

	if audioURL == "" {
		slog.Error("Missing URL in audio download request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to proxy audio for direct download", "url", audioURL, "outputFormat", outputFormat)

	// Get video info to suggest a filename
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), audioURL)
	if err != nil {
		slog.Warn("Could not get video info for filename suggestion, proceeding without it", "error", err)
		videoInfo = &service.VideoInfo{Title: "audio", Ext: "mp3"} // Fallback
	}

	// Set headers for download
	if outputFormat == "" {
		outputFormat = "mp3" // Default for content-type
	}
	filename := fmt.Sprintf("%s.%s", sanitizeFilename(videoInfo.Title), outputFormat)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", fmt.Sprintf("audio/%s", outputFormat)) // e.g., audio/mp3
	// Transfer-Encoding and Cache-Control will be handled by the proxy

	// Use the Streamer service to proxy the audio
	err = h.streamer.ProxyAudio(r.Context(), w, r, audioURL)
	if err != nil {
		slog.Error("Failed to proxy audio for direct download", "error", err, "url", audioURL)
		http.Error(w, fmt.Sprintf("Failed to download audio: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Info("Direct audio download stream finished", "url", audioURL)
}

// sanitizeFilename removes characters that are not allowed in filenames.
func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "\"", "_")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, "|", "_")
	s = strings.ReplaceAll(s, " ", "_")  // Replace spaces with underscores
	s = strings.ReplaceAll(s, "__", "_") // Replace double underscores
	s = strings.Trim(s, "_")             // Trim leading/trailing underscores
	if len(s) > 200 {                    // Limit filename length
		s = s[:200]
	}
	return s
}
