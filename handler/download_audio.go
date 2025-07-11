package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath" // Import filepath

	"gostreampuller/service"
)

// DownloadAudioHandler handles requests to download audio.
type DownloadAudioHandler struct {
	downloader *service.Downloader
}

// NewDownloadAudioHandler creates a new DownloadAudioHandler.
func NewDownloadAudioHandler(downloader *service.Downloader) *DownloadAudioHandler {
	return &DownloadAudioHandler{
		downloader: downloader,
	}
}

// DownloadAudioRequest represents the request body for audio download.
type DownloadAudioRequest struct {
	URL          string `json:"url"`
	OutputFormat string `json:"outputFormat"`
	Codec        string `json:"codec"`
	Bitrate      string `json:"bitrate"`
}

// DownloadAudioResponse represents the response body for audio download.
type DownloadAudioResponse struct {
	FilePath  string             `json:"filePath"`
	VideoInfo *service.VideoInfo `json:"videoInfo"` // Re-use VideoInfo for audio metadata
	Message   string             `json:"message"`
}

// Handle handles the audio download request.
//
//	@Summary		Download an audio file
//	@Description	Downloads an audio file from a given URL to the server's download directory.
//	@Tags			download
//	@Accept			json
//	@Produce		json
//	@Param			request	body		DownloadAudioRequest	true	"Audio download request"
//	@Success		200		{object}	DownloadAudioResponse	"Audio downloaded successfully"
//	@Failure		400		{object}	ErrorResponse			"Invalid request payload or missing URL"
//	@Failure		500		{object}	ErrorResponse			"Internal server error during audio download"
//	@Router			/download/audio [post]
func (h *DownloadAudioHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req DownloadAudioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode request body", "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Invalid request payload: %v", err)).ToJson(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		slog.Error("Missing URL in download audio request")
		http.Error(w, NewErrorResponse("URL is required").ToJson(), http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to download audio", "url", req.URL, "outputFormat", req.OutputFormat, "codec", req.Codec, "bitrate", req.Bitrate)

	filePath, videoInfo, err := h.downloader.DownloadAudioToFile(r.Context(), req.URL, req.OutputFormat, req.Codec, req.Bitrate)
	if err != nil {
		slog.Error("Failed to download audio", "error", err, "url", req.URL)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to download audio: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	resp := DownloadAudioResponse{
		FilePath:  filePath,
		VideoInfo: videoInfo,
		Message:   "Audio downloaded successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	slog.Info("Audio downloaded successfully", "filePath", filePath)
}

// ServeDownloadedAudio serves a previously downloaded audio file.
//
//	@Summary		Serve a downloaded audio file
//	@Description	Serves an audio file from the server's download directory given its filename.
//	@Tags			download
//	@Produce		audio/mpeg
//	@Param			filename	path		string			true	"Filename of the audio to serve"
//	@Success		200			{file}		file			"Successfully served audio file"
//	@Failure		400			{object}	ErrorResponse	"Missing filename"
//	@Failure		404			{object}	ErrorResponse	"File not found"
//	@Failure		500			{object}	ErrorResponse	"Internal server error"
//	@Router			/download/audio/{filename} [get]
func (h *DownloadAudioHandler) ServeDownloadedAudio(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		slog.Error("Missing filename for serving downloaded audio")
		http.Error(w, NewErrorResponse("Filename is required").ToJson(), http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.downloader.GetDownloadDir(), filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		slog.Warn("Downloaded audio file not found", "filePath", filePath)
		http.Error(w, NewErrorResponse("File not found").ToJson(), http.StatusNotFound)
		return
	} else if err != nil {
		slog.Error("Error checking file existence", "filePath", filePath, "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Error accessing file: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	slog.Info("Serving downloaded audio file", "filePath", filePath)
	http.ServeFile(w, r, filePath)
}
