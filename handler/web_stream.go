package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url" // Import net/url for URL parsing

	"gostreampuller/service"
)

// WebStreamHandler handles web-based video streaming requests.
type WebStreamHandler struct {
	downloader *service.Downloader
	template   *template.Template
}

// NewWebStreamHandler creates a new WebStreamHandler.
func NewWebStreamHandler(downloader *service.Downloader) *WebStreamHandler {
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

	if videoURL == "" {
		slog.Error("Missing URL in web stream request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to get video info for web stream", "url", videoURL)

	// Get video info
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), videoURL)
	if err != nil {
		slog.Error("Failed to get video info for web stream", "error", err, "url", videoURL)
		http.Error(w, fmt.Sprintf("Failed to get video information: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare data for the template
	// We'll pass the video info and a URL for the actual stream endpoint
	// The stream endpoint will be a GET request with URL, resolution, and codec as query parameters
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

	slog.Info("Attempting to stream video for web player", "url", videoURL, "resolution", resolution, "codec", codec)

	readCloser, err := h.downloader.StreamVideo(r.Context(), videoURL, "mp4", resolution, codec)
	if err != nil {
		slog.Error("Failed to stream video for web player", "error", err, "url", videoURL)
		http.Error(w, fmt.Sprintf("Failed to stream video: %v", err), http.StatusInternalServerError)
		return
	}
	defer readCloser.Close()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")

	slog.Info("Starting web video stream", "url", videoURL)
	if _, err := io.Copy(w, readCloser); err != nil {
		slog.Error("Error while streaming web video", "error", err, "url", videoURL)
		// Client might see a broken stream.
	}
	slog.Info("Web video stream finished", "url", videoURL)
}
