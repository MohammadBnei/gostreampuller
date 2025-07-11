package handler

import (
	// Import the embed package
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gostreampuller/service"
	"gostreampuller/web"
)

// WebStreamHandler handles web-based video streaming requests.
type WebStreamHandler struct {
	downloader      *service.Downloader
	indexTemplate   *template.Template // New template for the initial page
	streamTemplate  *template.Template // Existing template for the streaming page
	progressManager *service.ProgressManager
}

// NewWebStreamHandler creates a new WebStreamHandler.
func NewWebStreamHandler(downloader *service.Downloader, pm *service.ProgressManager) *WebStreamHandler {
	// Use template.ParseFS to parse templates from the embedded file system
	indexTmpl, err := template.ParseFS(web.Content, "index.html")
	if err != nil {
		slog.Error("Failed to parse web index template", "error", err)
		panic(err)
	}
	streamTmpl, err := template.ParseFS(web.Content, "stream.html")
	if err != nil {
		slog.Error("Failed to parse web stream template", "error", err)
		panic(err)
	}
	return &WebStreamHandler{
		downloader:      downloader,
		indexTemplate:   indexTmpl,
		streamTemplate:  streamTmpl,
		progressManager: pm,
	}
}

// ServeMainPage serves the initial HTML page with just the URL input.
//
//	@Summary		Serve main web interface page
//	@Description	Serves the initial HTML page for GoStreamPuller web interface.
//	@Tags			web
//	@Produce		html
//	@Success		200	{string}	html	"HTML page for URL input"
//	@Router			/ [get]
func (h *WebStreamHandler) ServeMainPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Error string
	}{
		Error: r.URL.Query().Get("error"), // Check for error message in query params
	}
	err := h.indexTemplate.Execute(w, data)
	if err != nil {
		slog.Error("Failed to execute web index template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ServeStreamPage serves the HTML page with the video streaming form and info.
// This is now the "second screen" that receives data via query parameters.
//
//	@Summary		Serve web streaming page with video info
//	@Description	Serves an HTML page that displays video information and allows streaming/downloading.
//	@Tags			web
//	@Produce		html
//	@Param			url			query		string	true	"Video URL"
//	@Param			progressID	query		string	true	"Unique ID for the operation to track"
//	@Param			videoInfo	query		string	true	"JSON string of VideoInfo"
//	@Success		200			{string}	html	"HTML page for video streaming"
//	@Failure		400			{string}	string	"Bad Request"
//	@Router			/web [get]
func (h *WebStreamHandler) ServeStreamPage(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	progressID := r.URL.Query().Get("progressID")
	videoInfoJSONStr := r.URL.Query().Get("videoInfo")

	if videoURL == "" || progressID == "" || videoInfoJSONStr == "" {
		slog.Error("Missing required query parameters for stream page", "url", videoURL, "progressID", progressID, "videoInfo", videoInfoJSONStr)
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	var videoInfo service.VideoInfo
	if err := json.Unmarshal([]byte(videoInfoJSONStr), &videoInfo); err != nil {
		slog.Error("Failed to unmarshal video info from query param", "error", err, "json", videoInfoJSONStr)
		http.Error(w, "Invalid video info format", http.StatusBadRequest)
		return
	}

	data := struct {
		URL           string
		VideoInfoJSON template.HTML
		VideoInfo     *service.VideoInfo
		ProgressID    string
	}{
		URL:           videoURL,
		VideoInfoJSON: template.HTML(videoInfoJSONStr),
		VideoInfo:     &videoInfo,
		ProgressID:    progressID,
	}
	err := h.streamTemplate.Execute(w, data)
	if err != nil {
		slog.Error("Failed to execute web stream template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleLoadInfo handles the initial URL submission, fetches video info, and redirects.
//
//	@Summary		Load video information and redirect to stream page
//	@Description	Receives a video URL, fetches its metadata, and redirects the user to the main streaming/downloading page with the info pre-populated.
//	@Tags			web
//	@Accept			x-www-form-urlencoded
//	@Produce		html
//	@Param			url	formData	string	true	"Video URL"
//	@Success		302	{string}	string	"Redirect to /web with video info"
//	@Failure		400	{string}	string	"Bad Request"
//	@Failure		500	{string}	string	"Internal Server Error"
//	@Router			/load-info [post]
func (h *WebStreamHandler) HandleLoadInfo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form data", "error", err)
		http.Redirect(w, r, "/?error="+url.QueryEscape("Bad Request: Could not parse form"), http.StatusFound)
		return
	}

	videoURL := r.FormValue("url")
	if videoURL == "" {
		slog.Error("Missing URL in load info request")
		http.Redirect(w, r, "/?error="+url.QueryEscape("URL is required"), http.StatusFound)
		return
	}

	// Generate a unique progress ID for this operation
	progressID := fmt.Sprintf("info-%d", time.Now().UnixNano())

	slog.Info("Attempting to get video info for web interface", "url", videoURL, "progressID", progressID)
	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), videoURL, progressID)
	if err != nil {
		slog.Error("Failed to get video info for web interface", "error", err, "url", videoURL)
		// Error event already sent by downloader.GetVideoInfo
		http.Redirect(w, r, "/?error="+url.QueryEscape(fmt.Sprintf("Failed to get video information: %v", err)), http.StatusFound)
		return
	}

	// Prepare data for redirection to /web
	videoInfoJSON, err := json.Marshal(videoInfo)
	if err != nil {
		slog.Error("Failed to marshal video info to JSON for redirect", "error", err)
		http.Redirect(w, r, "/?error="+url.QueryEscape("Internal server error: Failed to process video info"), http.StatusFound)
		return
	}

	// Construct the redirect URL with all necessary parameters
	redirectURL := fmt.Sprintf("/web?url=%s&progressID=%s&videoInfo=%s",
		url.QueryEscape(videoURL),
		url.QueryEscape(progressID),
		url.QueryEscape(string(videoInfoJSON)),
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
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
		slog.Error("Streaming unsupported: http.ResponseWriter does not implement http.Flusher")
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
	progressID := r.URL.Query().Get("progressID")     // Get progress ID

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
