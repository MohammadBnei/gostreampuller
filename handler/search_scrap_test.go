package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gostreampuller/config"
	"gostreampuller/handler"
	"gostreampuller/service"
)

// MockSearchServiceWithScraping is a mock implementation of SearchService that supports scraping.
type MockSearchServiceWithScraping struct {
	results []handler.SearchResultResponse
	err     error
}

func (m *MockSearchServiceWithScraping) Search(query string, limit int) ([]service.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Convert handler.SearchResultResponse to service.SearchResult
	serviceResults := make([]service.SearchResult, len(m.results))
	for i, r := range m.results {
		serviceResults[i] = service.SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Snippet,
		}
	}
	return serviceResults, nil
}

func TestSearchHandler_Scraping(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name            string
		scrapParam      string
		mockResults     []handler.SearchResultResponse
		mockError       error
		expectedContent []string
		serverContent   []string // Content to be served by the mock server for each URL
	}{
		{
			name:       "Scraping enabled, scrap query param present",
			scrapParam: "true",
			mockResults: []handler.SearchResultResponse{
				{Title: "Result 1", URL: "/1", Snippet: "Snippet 1"},
				{Title: "Result 2", URL: "/2", Snippet: "Snippet 2"},
			},
			expectedContent: []string{"Example Domain 1", "Example Domain 2"}, // Expecting markdown content
			serverContent:   []string{"<html><body><h1>Example Domain 1</h1></body></html>", "<html><body><h1>Example Domain 2</h1></body></html>"},
		},
		{
			name:       "Scraping enabled, scrap query param missing",
			scrapParam: "",
			mockResults: []handler.SearchResultResponse{
				{Title: "Result 1", URL: "/1", Snippet: "Snippet 1"},
				{Title: "Result 2", URL: "/2", Snippet: "Snippet 2"},
			},
			expectedContent: []string{"", ""}, // Expecting empty content
			serverContent:   []string{"<html><body><h1>Example Domain 1</h1></body></html>", "<html><body><h1>Example Domain 2</h1></body></html>"},
		},
		{
			name:       "Scraping fails for a URL",
			scrapParam: "true",
			mockResults: []handler.SearchResultResponse{
				{Title: "Result 1", URL: "/1", Snippet: "Snippet 1"},
				{Title: "Result 2", URL: "invalid-url", Snippet: "Snippet 2"}, // Invalid URL
			},
			expectedContent: []string{"Example Domain 1", ""},                                    // Expecting empty content for invalid URL
			serverContent:   []string{"<html><body><h1>Example Domain 1</h1></body></html>", ""}, // No content for invalid URL
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/1":
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, tc.serverContent[0])
				case "/2":
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, tc.serverContent[1])
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			// Adjust URLs in mockResults to use the mock server's URL
			for i := range tc.mockResults {
				tc.mockResults[i].URL = server.URL + tc.mockResults[i].URL
			}

			// Create a mock config
			cfg := &config.Config{
				LocalMode: true,
			}

			// Create a mock search service
			mockSvc := &MockSearchServiceWithScraping{
				results: tc.mockResults,
				err:     tc.mockError,
			}

			// Create a search searchHandler with the mock config and service
			searchHandler := handler.NewSearchHandler(cfg, mockSvc)

			// Create a request with the scrap query parameter
			req, err := http.NewRequest(http.MethodGet, "/search?q=test&scrap="+tc.scrapParam, nil)
			assert.NoError(t, err, "Failed to create request")

			// Create a recorder to capture the response
			recorder := httptest.NewRecorder()

			// Serve the request
			searchHandler.Handle(recorder, req)

			// Check the response status code
			assert.Equal(t, http.StatusOK, recorder.Code, "Expected status code %d, got %d", http.StatusOK, recorder.Code)

			// Decode the response body
			var response []handler.SearchResultResponse
			err = json.NewDecoder(recorder.Body).Decode(&response)
			assert.NoError(t, err, "Failed to decode response body")

			// Check the number of results
			assert.Equal(t, len(tc.mockResults), len(response), "Expected %d results, got %d", len(tc.mockResults), len(response))

			// Check the content of each result
			for i, result := range response {
				assert.Contains(t, result.Content, tc.expectedContent[i], "Expected content to contain %q, got %q", tc.expectedContent[i], result.Content)
			}
		})
	}
}
