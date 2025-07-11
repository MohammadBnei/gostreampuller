package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"gostreampuller/service"
)

// StreamVideoHandler handles requests to stream videos.
type StreamVideoHandler struct {
	downloader *service.Downloader
}

// NewStreamVideoHandler creates a new StreamVideoHandler.
func NewStreamVideoHandler(downloader *service.Downloader) *StreamVideoHandler {
	return &StreamVideoHandler{
		downloader: downloader,
	}
}

// StreamVideoRequest represents the request body for video streaming.
type StreamVideoRequest struct {
	URL        string `json:"url"`
	Format     string `json:"format"`
	Resolution string `json:"resolution"`
	Codec      string `json:"codec"`
}

// Handle handles the video streaming request.
//	@Summary		Stream a video
//	@Description	Streams a video directly from the source URL.
//	@Tags			stream
//	@Accept			json
//	@Produce		video/mp4
//	@Param			request	body		StreamVideoRequest	true	"Video stream request"
//	@Success		200		{file}		file				"Successfully streamed video"
//	@Failure		400		{object}	ErrorResponse		"Invalid request payload or missing URL"
//	@Failure		500		{object}	ErrorResponse		"Internal server error during video streaming"
//	@Router			/stream/video [post]
func (h *StreamVideoHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req StreamVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode request body", "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Invalid request payload: %v", err)).ToJson(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		slog.Error("Missing URL in stream video request")
		http.Error(w, NewErrorResponse("URL is required").ToJson(), http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to stream video", "url", req.URL, "format", req.Format, "resolution", req.Resolution, "codec", req.Codec)

	// Pass an empty string for progressID as this API endpoint doesn't have an SSE client
	readCloser, err := h.downloader.StreamVideo(r.Context(), req.URL, req.Format, req.Resolution, req.Codec, "")
	if err != nil {
		slog.Error("Failed to stream video", "error", err, "url", req.URL)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to stream video: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}
	defer readCloser.Close()

	// Set appropriate headers for video streaming
	w.Header().Set("Content-Type", "video/mp4") // Assuming mp4 for now, can be dynamic
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")

	slog.Info("Starting video stream", "url", req.URL)
	if _, err := io.Copy(w, readCloser); err != nil {
		slog.Error("Error while streaming video", "error", err, "url", req.URL)
		// Note: Cannot send HTTP error after headers have been written and body started.
		// The client might just see a broken stream.
	}
	slog.Info("Video stream finished", "url", req.URL)
}
