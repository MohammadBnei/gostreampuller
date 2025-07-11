package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"gostreampuller/service"
)

// StreamAudioHandler handles requests to stream audio.
type StreamAudioHandler struct {
	downloader *service.Downloader
}

// NewStreamAudioHandler creates a new StreamAudioHandler.
func NewStreamAudioHandler(downloader *service.Downloader) *StreamAudioHandler {
	return &StreamAudioHandler{
		downloader: downloader,
	}
}

// StreamAudioRequest represents the request body for audio streaming.
type StreamAudioRequest struct {
	URL          string `json:"url"`
	OutputFormat string `json:"outputFormat"`
	Codec        string `json:"codec"`
	Bitrate      string `json:"bitrate"`
}

// Handle handles the audio streaming request.
//	@Summary		Stream an audio file
//	@Description	Streams an audio file directly from the source URL.
//	@Tags			stream
//	@Accept			json
//	@Produce		audio/mpeg
//	@Param			request	body		StreamAudioRequest	true	"Audio stream request"
//	@Success		200		{file}		file				"Successfully streamed audio"
//	@Failure		400		{object}	ErrorResponse		"Invalid request payload or missing URL"
//	@Failure		500		{object}	ErrorResponse		"Internal server error during audio streaming"
//	@Router			/stream/audio [post]
func (h *StreamAudioHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req StreamAudioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode request body", "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Invalid request payload: %v", err)).ToJson(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		slog.Error("Missing URL in stream audio request")
		http.Error(w, NewErrorResponse("URL is required").ToJson(), http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to stream audio", "url", req.URL, "outputFormat", req.OutputFormat, "codec", req.Codec, "bitrate", req.Bitrate)

	// Pass an empty string for progressID as this API endpoint doesn't have an SSE client
	readCloser, err := h.downloader.StreamAudio(r.Context(), req.URL, req.OutputFormat, req.Codec, req.Bitrate, "")
	if err != nil {
		slog.Error("Failed to stream audio", "error", err, "url", req.URL)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to stream audio: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}
	defer readCloser.Close()

	// Set appropriate headers for audio streaming
	w.Header().Set("Content-Type", "audio/mpeg") // Assuming mp3 for now, can be dynamic
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")

	slog.Info("Starting audio stream", "url", req.URL)
	if _, err := io.Copy(w, readCloser); err != nil {
		slog.Error("Error while streaming audio", "error", err, "url", req.URL)
		// Note: Cannot send HTTP error after headers have been written and body started.
		// The client might just see a broken stream.
	}
	slog.Info("Audio stream finished", "url", req.URL)
}
