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

// DownloadVideoHandler handles requests to download videos.
type DownloadVideoHandler struct {
	downloader *service.Downloader
}

// NewDownloadVideoHandler creates a new DownloadVideoHandler.
func NewDownloadVideoHandler(downloader *service.Downloader) *DownloadVideoHandler {
	return &DownloadVideoHandler{
		downloader: downloader,
	}
}

// DownloadVideoRequest represents the request body for video download.
type DownloadVideoRequest struct {
	URL        string `json:"url"`
	Format     string `json:"format"`
	Resolution string `json:"resolution"`
	Codec      string `json:"codec"`
}

// DownloadVideoResponse represents the response body for video download.
type DownloadVideoResponse struct {
	FilePath  string            `json:"filePath"`
	VideoInfo *service.VideoInfo `json:"videoInfo"`
	Message   string            `json:"message"`
}

// Handle handles the video download request.
// @Summary Download a video
// @Description Downloads a video from a given URL to the server's download directory.
// @Tags download
// @Accept json
// @Produce json
// @Param request body DownloadVideoRequest true "Video download request"
// @Success 200 {object} DownloadVideoResponse "Video downloaded successfully"
// @Failure 400 {object} ErrorResponse "Invalid request payload or missing URL"
// @Failure 500 {object} ErrorResponse "Internal server error during video download"
// @Router /download/video [post]
func (h *DownloadVideoHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req DownloadVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode request body", "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Invalid request payload: %v", err)).ToJson(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		slog.Error("Missing URL in download video request")
		http.Error(w, NewErrorResponse("URL is required").ToJson(), http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to download video", "url", req.URL, "format", req.Format, "resolution", req.Resolution, "codec", req.Codec)

	filePath, videoInfo, err := h.downloader.DownloadVideoToFile(r.Context(), req.URL, req.Format, req.Resolution, req.Codec)
	if err != nil {
		slog.Error("Failed to download video", "error", err, "url", req.URL)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to download video: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	resp := DownloadVideoResponse{
		FilePath:  filePath,
		VideoInfo: videoInfo,
		Message:   "Video downloaded successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	slog.Info("Video downloaded successfully", "filePath", filePath)
}

// ServeDownloadedVideo serves a previously downloaded video file.
// @Summary Serve a downloaded video file
// @Description Serves a video file from the server's download directory given its filename.
// @Tags download
// @Produce video/mp4
// @Param filename path string true "Filename of the video to serve"
// @Success 200 {file} file "Successfully served video file"
// @Failure 400 {object} ErrorResponse "Missing filename"
// @Failure 404 {object} ErrorResponse "File not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /download/video/{filename} [get]
func (h *DownloadVideoHandler) ServeDownloadedVideo(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		slog.Error("Missing filename for serving downloaded video")
		http.Error(w, NewErrorResponse("Filename is required").ToJson(), http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.downloader.GetDownloadDir(), filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		slog.Warn("Downloaded video file not found", "filePath", filePath)
		http.Error(w, NewErrorResponse("File not found").ToJson(), http.StatusNotFound)
		return
	} else if err != nil {
		slog.Error("Error checking file existence", "filePath", filePath, "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Error accessing file: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	slog.Info("Serving downloaded video file", "filePath", filePath)
	http.ServeFile(w, r, filePath)
}

// GetVideoInfoRequest represents the request body for getting video info.
type GetVideoInfoRequest struct {
	URL string `json:"url"`
}

// GetVideoInfoResponse represents the response body for getting video info.
type GetVideoInfoResponse struct {
	VideoInfo *service.VideoInfo `json:"videoInfo"`
	Message   string            `json:"message"`
}

// GetVideoInfo handles requests to get video information without downloading.
// @Summary Get video information
// @Description Retrieves metadata for a video from a given URL without downloading the file.
// @Tags download
// @Accept json
// @Produce json
// @Param request body GetVideoInfoRequest true "Video info request"
// @Success 200 {object} GetVideoInfoResponse "Video information retrieved successfully"
// @Failure 400 {object} ErrorResponse "Invalid request payload or missing URL"
// @Failure 500 {object} ErrorResponse "Internal server error during video info retrieval"
// @Router /download/video/info [post]
func (h *DownloadVideoHandler) GetVideoInfo(w http.ResponseWriter, r *http.Request) {
	var req GetVideoInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode request body for video info", "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Invalid request payload: %v", err)).ToJson(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		slog.Error("Missing URL in get video info request")
		http.Error(w, NewErrorResponse("URL is required").ToJson(), http.StatusBadRequest)
		return
	}

	slog.Info("Attempting to get video info", "url", req.URL)

	videoInfo, err := h.downloader.GetVideoInfo(r.Context(), req.URL)
	if err != nil {
		slog.Error("Failed to get video info", "error", err, "url", req.URL)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to get video info: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	resp := GetVideoInfoResponse{
		VideoInfo: videoInfo,
		Message:   "Video information retrieved successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	slog.Info("Video information retrieved successfully", "videoID", videoInfo.ID)
}

// DeleteDownloadedFile deletes a previously downloaded file.
// @Summary Delete a downloaded file
// @Description Deletes a file from the server's download directory given its filename.
// @Tags download
// @Produce json
// @Param filename path string true "Filename of the file to delete"
// @Success 200 {object} SuccessResponse "File deleted successfully"
// @Failure 400 {object} ErrorResponse "Missing filename"
// @Failure 404 {object} ErrorResponse "File not found"
// @Failure 500 {object} ErrorResponse "Internal server error during file deletion"
// @Router /download/delete/{filename} [delete]
func (h *DownloadVideoHandler) DeleteDownloadedFile(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		slog.Error("Missing filename for deleting downloaded file")
		http.Error(w, NewErrorResponse("Filename is required").ToJson(), http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.downloader.GetDownloadDir(), filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		slog.Warn("File not found for deletion", "filePath", filePath)
		http.Error(w, NewErrorResponse("File not found").ToJson(), http.StatusNotFound)
		return
	} else if err != nil {
		slog.Error("Error checking file existence for deletion", "filePath", filePath, "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Error accessing file: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	if err := os.Remove(filePath); err != nil {
		slog.Error("Failed to delete file", "filePath", filePath, "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to delete file: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	slog.Info("File deleted successfully", "filePath", filePath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NewSuccessResponse("File deleted successfully"))
}

// ListDownloadedFilesResponse represents the response body for listing downloaded files.
type ListDownloadedFilesResponse struct {
	Files   []FileInfo `json:"files"`
	Message string     `json:"message"`
}

// FileInfo represents metadata for a downloaded file.
type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// ListDownloadedFiles lists all files in the download directory.
// @Summary List downloaded files
// @Description Lists all files present in the server's configured download directory.
// @Tags download
// @Produce json
// @Success 200 {object} ListDownloadedFilesResponse "Successfully listed downloaded files"
// @Failure 500 {object} ErrorResponse "Internal server error during file listing"
// @Router /download/list [get]
func (h *DownloadVideoHandler) ListDownloadedFiles(w http.ResponseWriter, r *http.Request) {
	downloadDir := h.downloader.GetDownloadDir()
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		slog.Error("Failed to read download directory", "directory", downloadDir, "error", err)
		http.Error(w, NewErrorResponse(fmt.Sprintf("Failed to list files: %v", err)).ToJson(), http.StatusInternalServerError)
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, err := file.Info()
		if err != nil {
			slog.Warn("Could not get file info", "filename", file.Name(), "error", err)
			continue
		}
		fileInfos = append(fileInfos, FileInfo{
			Name:    info.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(http.TimeFormat),
		})
	}

	resp := ListDownloadedFilesResponse{
		Files:   fileInfos,
		Message: "Successfully listed downloaded files",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	slog.Info("Successfully listed downloaded files", "count", len(fileInfos))
}

// ErrorResponse represents a generic error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// NewErrorResponse creates a new ErrorResponse.
func NewErrorResponse(message string) *ErrorResponse {
	return &ErrorResponse{
		Error:   "Bad Request", // Default error type, can be more specific
		Message: message,
	}
}

// ToJson converts the ErrorResponse to a JSON string.
func (e *ErrorResponse) ToJson() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// SuccessResponse represents a generic success response.
type SuccessResponse struct {
	Message string `json:"message"`
}

// NewSuccessResponse creates a new SuccessResponse.
func NewSuccessResponse(message string) *SuccessResponse {
	return &SuccessResponse{
		Message: message,
	}
}
