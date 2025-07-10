package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"home-go-api-template/config"
	"home-go-api-template/handler"
	"home-go-api-template/service"
)

// Mock search service for testing.
type mockSearchService struct{}

// We need to import the service package to use SearchResult.
func (m *mockSearchService) Search(_ string, limit int) ([]service.SearchResult, error) {
	return []service.SearchResult{
		{
			Title:   "Test Result",
			URL:     "https://example.com",
			Snippet: "This is a test result",
		},
	}, nil
}

func TestSearchHandlerAuth(t *testing.T) {
	tests := []struct {
		name           string
		localMode      bool
		withAuth       bool
		expectedStatus int
	}{
		{
			name:           "Local mode bypasses auth",
			localMode:      true,
			withAuth:       false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Normal mode requires auth - missing auth",
			localMode:      false,
			withAuth:       false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Normal mode with auth",
			localMode:      false,
			withAuth:       true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create config with test settings
			cfg := &config.Config{
				AuthUsername: "test-user",
				AuthPassword: "test-pass",
				LocalMode:    tc.localMode,
			}

			// Create handler with mock service
			h := handler.NewSearchHandler(cfg, &mockSearchService{})

			// Create test request
			req, err := http.NewRequest(http.MethodGet, "/search?q=test", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Add auth if needed
			if tc.withAuth {
				req.SetBasicAuth("test-user", "test-pass")
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			h.Handle(rr, req)

			// Check status code
			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tc.expectedStatus)
			}
		})
	}
}
