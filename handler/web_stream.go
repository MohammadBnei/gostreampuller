package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url" // Import net/url for URL parsing
	"os"
	"strings"
	"time" // For generating unique IDs

	"gostreampuller/service"
)

// WebStreamHandler handles web-based video streaming requests.
type WebStreamHandler struct {
	downloader      *service.Downloader
	template        *template.Template
	progressManager *service.ProgressManager // Added ProgressManager
}

// NewWebStreamHandler creates a new WebStreamHandler.
func NewWebStreamHandler(downloader *service.Downloader, pm *service.ProgressManager) *WebStreamHandler {
	// Parse the HTML template once during initialization
	tmpl, err := template.ParseFiles("web/stream.html")
	if err != nil {
		slog.Error("Failed to parse web stream template", "error", err)
		// In a real application, you might want to panic or handle this more gracefully
		// to prevent the server from starting with a broken template.
		panic(err)
	}
	return &WebStreamHandler{
		downloader:      downloader,
		template:        tmpl,
		progressManager: pm,
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
//	@Param			resolution	formData	string	false	"Video Resolution (e.g., 480, 720, 1080)"
//	@Param			codec		formData	string	false	"Video Codec (e.g., avc1, vp9)"
//	@Param			audioQuality formData string false "Audio Quality (e.g., 128k, 192k)"
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
	audioQuality := r.FormValue("audioQuality") // Get audio quality
	action := r.FormValue("action")             // Get the action from the clicked button

	if videoURL == "" {
		slog.Error("Missing URL in web stream request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Generate a unique progress ID for this operation
	progressID := fmt.Sprintf("op-%d", time.Now().UnixNano())

	// Get video info (using GetVideoInfo for general metadata)
	// This will send the initial "fetching_info" event and the "info_fetched" event
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), videoURL, progressID)
	if err != nil {
		slog.Error("Failed to get video info for web stream", "error", err, "url", videoURL)
		// Error event already sent by downloader.GetVideoInfo
		http.Error(w, fmt.Sprintf("Failed to get video information: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare data for the template
	streamURL := fmt.Sprintf("/web/play?url=%s&resolution=%s&codec=%s&progressID=%s",
		url.QueryEscape(videoURL),
		url.QueryEscape(resolution),
		url.QueryEscape(codec),
		url.QueryEscape(progressID),
	)

	downloadVideoURL := fmt.Sprintf("/web/download/video?url=%s&resolution=%s&codec=%s&progressID=%s",
		url.QueryEscape(videoURL),
		url.QueryEscape(resolution),
		url.QueryEscape(codec),
		url.QueryEscape(progressID),
	)
	downloadAudioURL := fmt.Sprintf("/web/download/audio?url=%s&bitrate=%s&progressID=%s",
		url.QueryEscape(videoURL),
		url.QueryEscape(audioQuality),
		url.QueryEscape(progressID),
	)

	// Marshal videoInfo to pretty JSON for display
	videoInfoJSON, err := json.MarshalIndent(videoInfo, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal video info to JSON", "error", err)
		videoInfoJSON = []byte(fmt.Sprintf(`{"error": "Failed to format video info: %v"}`, err))
	}

	data := struct {
		StreamURL        string
		DownloadVideoURL string
		DownloadAudioURL string
		VideoInfoJSON    template.HTML // Use template.HTML to prevent escaping
		VideoInfo        *service.VideoInfo
		ProgressID       string // Pass progress ID to the template
		Action           string // Pass the requested action to the template
	}{
		StreamURL:        streamURL,
		DownloadVideoURL: downloadVideoURL,
		DownloadAudioURL: downloadAudioURL,
		VideoInfoJSON:    template.HTML(videoInfoJSON),
		VideoInfo:        videoInfo,
		ProgressID:       progressID,
		Action:           action, // So frontend knows which operation to start
	}

	// Re-execute the template with the stream URL and video info
	// This will render the same page but with the video player and info populated
	err = h.template.Execute(w, data)
	if err != nil {
		slog.Error("Failed to execute web stream template with data", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ServeProgress handles Server-Sent Events (SSE) for progress updates.
//
//	@Summary		Get progress updates via SSE
//	@Description	Establishes an SSE connection to stream real-time progress updates for download/stream operations.
//	@Tags			web
//	@Produce		text/event-stream
//	@Param			progressID	query		string	true	"Unique ID for the operation to track"
//	@Success		200			{string}	string	"Event stream of progress updates"
//	@Failure		400			{string}	string	"Missing progressID"
//	@Router			/web/progress [get]
func (h *WebStreamHandler) ServeProgress(w http.ResponseWriter, r *http.Request) {
	progressID := r.URL.Query().Get("progressID")
	if progressID == "" {
		http.Error(w, "progressID is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow CORS for SSE

	clientChan := h.progressManager.RegisterClient(progressID)
	defer h.progressManager.UnregisterClient(progressID)

	slog.Info("SSE client connected", "progressID", progressID)

	// Send a "connected" event immediately
	connectedEvent, _ := json.Marshal(service.ProgressEvent{
		ID:      progressID,
		Status:  "connected",
		Message: "Connected to progress stream.",
	})
	fmt.Fprintf(w, "data: %s\n\n", connectedEvent)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			slog.Info("SSE client disconnected", "progressID", progressID, "reason", r.Context().Err())
			return
		case eventBytes := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", eventBytes)
			flusher.Flush()
		}
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
//	@Param			progressID	query		string	true	"Unique ID for progress tracking"
//	@Success		200			{file}		file	"Successfully streamed video"
//	@Failure		400			{string}	string	"Bad Request"
//	@Failure		500			{string}	string	"Internal Server Error"
//	@Router			/web/play [get]
func (h *WebStreamHandler) PlayWebStream(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	resolution := r.URL.Query().Get("resolution")
	codec := r.URL.Query().Get("codec")
	progressID := r.URL.Query().Get("progressID") // Get progress ID

	if videoURL == "" {
		slog.Error("Missing URL in web stream play request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to stream video for web player", "url", videoURL, "resolution", resolution, "codec", codec, "progressID", progressID)

	// Use the downloader's StreamVideo method (direct piping)
	readCloser, err := h.downloader.StreamVideo(r.Context(), videoURL, "mp4", resolution, codec, progressID)
	if err != nil {
		slog.Error("Failed to stream video for web player", "error", err, "url", videoURL)
		h.progressManager.SendError(progressID, fmt.Sprintf("Failed to stream video: %v", err), err)
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
		// Note: Cannot send HTTP error after headers have been written and body started.
		// The client might just see a broken stream.
		h.progressManager.SendError(progressID, fmt.Sprintf("Error during video stream: %v", err), err)
	} else {
		h.progressManager.SendComplete(progressID, "Video stream finished.", nil) // No video info needed for stream completion
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
//	@Param			progressID	query		string	true	"Unique ID for progress tracking"
//	@Success		200			{file}		file	"Successfully streamed video for download"
//	@Failure		400			{string}	string	"Bad Request"
//	@Failure		500			{string}	string	"Internal Server Error"
//	@Router			/web/download/video [get]
func (h *WebStreamHandler) DownloadVideoToBrowser(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	resolution := r.URL.Query().Get("resolution")
	codec := r.URL.Query().Get("codec")
	progressID := r.URL.Query().Get("progressID") // Get progress ID

	if videoURL == "" {
		slog.Error("Missing URL in video download request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to download video to temp file for direct download", "url", videoURL, "resolution", resolution, "codec", codec, "progressID", progressID)

	// Get video info to suggest a filename
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), videoURL, progressID) // Pass progressID
	if err != nil {
		slog.Warn("Could not get video info for filename suggestion, proceeding without it", "error", err)
		videoInfo = &service.VideoInfo{Title: "video", Ext: "mp4"} // Fallback
		// Error event already sent by downloader.GetVideoInfo
		http.Error(w, fmt.Sprintf("Failed to get video information: %v", err), http.StatusInternalServerError)
		return
	}

	// Download video to a temporary file
	tempFilePath, err := h.downloader.DownloadVideoToTempFile(r.Context(), videoURL, "mp4", resolution, codec, progressID) // Pass progressID
	if err != nil {
		slog.Error("Failed to download video to temporary file", "error", err, "url", videoURL)
		// Error event already sent by downloader.DownloadVideoToTempFile
		http.Error(w, fmt.Sprintf("Failed to download video: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := os.Remove(tempFilePath); err != nil {
			slog.Error("Failed to remove temporary video file", "filePath", tempFilePath, "error", err)
		}
	}()

	// Set headers for download
	filename := fmt.Sprintf("%s.%s", sanitizeFilename(videoInfo.Title), "mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "video/mp4")
	// http.ServeFile will handle Content-Length and other headers

	slog.Info("Serving temporary video file for direct download", "filePath", tempFilePath, "filename", filename)
	http.ServeFile(w, r, tempFilePath)
	h.progressManager.SendComplete(progressID, "Video download complete.", videoInfo) // Send complete event
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
//	@Param			progressID		query		string	true	"Unique ID for progress tracking"
//	@Success		200				{file}		file	"Successfully streamed audio for download"
//	@Failure		400				{string}	string	"Bad Request"
//	@Failure		500				{string}	string	"Internal Server Error"
//	@Router			/web/download/audio [get]
func (h *WebStreamHandler) DownloadAudioToBrowser(w http.ResponseWriter, r *http.Request) {
	audioURL := r.URL.Query().Get("url")
	outputFormat := r.URL.Query().Get("outputFormat") // This is not used by ProxyAudio, but kept for Swagger
	codec := r.URL.Query().Get("codec")               // This is not used by ProxyAudio, but kept for Swagger
	bitrate := r.URL.Query().Get("bitrate")           // Get bitrate from query parameter
	progressID := r.URL.Query().Get("progressID")    // Get progress ID

	if audioURL == "" {
		slog.Error("Missing URL in audio download request")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to download audio to temp file for direct download", "url", audioURL, "outputFormat", outputFormat, "bitrate", bitrate, "progressID", progressID)

	// Get video info to suggest a filename
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), audioURL, progressID) // Pass progressID
	if err != nil {
		slog.Warn("Could not get video info for filename suggestion, proceeding without it", "error", err)
		videoInfo = &service.VideoInfo{Title: "audio", Ext: "mp3"} // Fallback
		// Error event already sent by downloader.GetVideoInfo
		http.Error(w, fmt.Sprintf("Failed to get video information: %v", err), http.StatusInternalServerError)
		return
	}

	// Download audio to a temporary file
	tempFilePath, err := h.downloader.DownloadAudioToTempFile(r.Context(), audioURL, outputFormat, codec, bitrate, progressID) // Pass progressID
	if err != nil {
		slog.Error("Failed to download audio to temporary file", "error", err, "url", audioURL)
		// Error event already sent by downloader.DownloadAudioToTempFile
		http.Error(w, fmt.Sprintf("Failed to download audio: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := os.Remove(tempFilePath); err != nil {
			slog.Error("Failed to remove temporary audio file", "filePath", tempFilePath, "error", err)
		}
	}()

	// Set headers for download
	if outputFormat == "" {
		outputFormat = "mp3" // Default for content-type
	}
	filename := fmt.Sprintf("%s.%s", sanitizeFilename(videoInfo.Title), outputFormat)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", fmt.Sprintf("audio/%s", outputFormat)) // e.g., audio/mp3
	// http.ServeFile will handle Content-Length and other headers

	slog.Info("Serving temporary audio file for direct download", "filePath", tempFilePath, "filename", filename)
	http.ServeFile(w, r, tempFilePath)
	h.progressManager.SendComplete(progressID, "Audio download complete.", videoInfo) // Send complete event
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
