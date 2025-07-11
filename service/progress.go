package service

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// ProgressEvent represents a single update in the download/stream process.
type ProgressEvent struct {
	ID        string    `json:"id"`        // Unique ID for this operation
	Status    string    `json:"status"`    // e.g., "fetching_info", "downloading", "encoding", "complete", "error"
	Message   string    `json:"message"`   // Human-readable message
	Percentage float64   `json:"percentage"` // 0.0 to 100.0, if applicable
	VideoInfo *VideoInfo `json:"videoInfo,omitempty"` // Optional: full video info
	Error     string    `json:"error,omitempty"`     // Error message if status is "error"
}

// ProgressManager manages and broadcasts progress updates to subscribed clients.
type ProgressManager struct {
	clients map[string]chan []byte // Map of progressID to a channel of JSON-encoded events
	mu      sync.RWMutex
}

// NewProgressManager creates and returns a new ProgressManager.
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		clients: make(map[string]chan []byte),
	}
}

// RegisterClient registers a new client for a given progressID.
// It returns a channel where events for this progressID will be sent.
func (pm *ProgressManager) RegisterClient(progressID string) chan []byte {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.clients[progressID]; ok {
		// If a client is already registered for this ID, close the old channel
		// and create a new one. This handles cases where a user refreshes the page.
		close(pm.clients[progressID])
	}

	clientChan := make(chan []byte)
	pm.clients[progressID] = clientChan
	slog.Debug("Registered new progress client", "progressID", progressID)
	return clientChan
}

// UnregisterClient unregisters a client for a given progressID.
func (pm *ProgressManager) UnregisterClient(progressID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if clientChan, ok := pm.clients[progressID]; ok {
		close(clientChan)
		delete(pm.clients, progressID)
		slog.Debug("Unregistered progress client", "progressID", progressID)
	}
}

// SendEvent sends a progress event to the specified client.
func (pm *ProgressManager) SendEvent(event ProgressEvent) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if clientChan, ok := pm.clients[event.ID]; ok {
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			slog.Error("Failed to marshal progress event", "error", err, "event", event)
			return
		}
		select {
		case clientChan <- jsonEvent:
			// Event sent successfully
		default:
			slog.Warn("Failed to send progress event, client channel full or closed", "progressID", event.ID)
			// Optionally, unregister client if channel is consistently full/closed
		}
	} else {
		slog.Debug("No client registered for progress ID", "progressID", event.ID)
	}
}

// SendError sends an error event to the specified client and unregisters it.
func (pm *ProgressManager) SendError(progressID, message string, err error) {
	event := ProgressEvent{
		ID:      progressID,
		Status:  "error",
		Message: message,
		Error:   err.Error(),
	}
	pm.SendEvent(event)
	pm.UnregisterClient(progressID) // Unregister on error
}

// SendComplete sends a complete event to the specified client and unregisters it.
func (pm *ProgressManager) SendComplete(progressID, message string, videoInfo *VideoInfo) {
	event := ProgressEvent{
		ID:        progressID,
		Status:    "complete",
		Message:   message,
		Percentage: 100.0,
		VideoInfo: videoInfo,
	}
	pm.SendEvent(event)
	pm.UnregisterClient(progressID) // Unregister on completion
}
