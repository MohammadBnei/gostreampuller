package duckduckgogo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// MockSearchClient implements the SearchClient interface for testing.
type MockSearchClient struct {
	results   []Result
	err       error
	callCount int // Track number of calls to simulate retries
}

func (m *MockSearchClient) Search(ctx context.Context, query string) ([]Result, error) {
	return m.SearchLimited(ctx, query, 0)
}

func (m *MockSearchClient) SearchLimited(ctx context.Context, query string, limit int) ([]Result, error) {
	m.callCount++
	if limit <= 0 || limit > len(m.results) {
		return m.results, m.err
	}
	return m.results[:limit], m.err
}

func TestDuckDuckGoSearchClient_SearchLimited(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		if !strings.Contains(r.URL.String(), "?q=test") {
			t.Errorf("Expected query parameter 'q=test', got %s", r.URL.String())
		}

		// Check user agent is set
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header not set")
		}

		// Return a simple HTML response with search results
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<div class="results">
				<div class="web-result">
					<a class="result__a">Test Title</a>
					<div class="result__snippet">Test Snippet</div>
					<a class="result__url">https://example.com</a>
					<img class="result__icon__img" src="icon.png" width="16" height="16" />
				</div>
			</div>
		`))
	}))
	defer server.Close()

	// Create client with mock server URL and retry config
	client := &DuckDuckGoSearchClient{
		baseUrl:      server.URL + "/",
		maxRetries:   2,
		retryBackoff: 10,
		httpClient:   http.DefaultClient,
	}

	// Test search
	results, err := client.SearchLimited(t.Context(), "test", 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify results
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Check result fields
	result := results[0]
	if result.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", result.Title)
	}
	if result.Snippet != "Test Snippet" {
		t.Errorf("Expected snippet 'Test Snippet', got '%s'", result.Snippet)
	}
	if result.FormattedURL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", result.FormattedURL)
	}
}

func TestCleanFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  test  ", "test"},
		{"test\ntest", "testtest"},
		{"\n test \n", "test"},
		{"", ""},
	}

	for _, test := range tests {
		result := clean(test.input)
		if result != test.expected {
			t.Errorf("clean(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestToIntFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"-10", -10},
		{"abc", 0}, // Invalid number should return 0
		{"", 0},    // Empty string should return 0
	}

	for _, test := range tests {
		result := toInt(test.input)
		if result != test.expected {
			t.Errorf("toInt(%q) = %d, expected %d", test.input, result, test.expected)
		}
	}
}
func TestRetryLogic(t *testing.T) {
	// Test case 1: Success after first retry
	t.Run("SuccessAfterRetry", func(t *testing.T) {
		attempts := 0

		// Create a custom HTTP client for testing
		customClient := &http.Client{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					attempts++
					if attempts == 1 {
						// First attempt fails
						return nil, errors.New("simulated network error")
					}
					// Second attempt succeeds with a minimal valid response
					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`
							<div class="results">
								<div class="web-result">
									<a class="result__a">Test Title</a>
									<div class="result__snippet">Test Snippet</div>
									<a class="result__url">https://example.com</a>
								</div>
							</div>
						`)),
					}, nil
				},
			},
		}

		// Create a client with retry configuration and custom HTTP client
		client := &DuckDuckGoSearchClient{
			baseUrl:      "https://example.com/",
			maxRetries:   2,
			retryBackoff: 10, // Small backoff for faster tests
			httpClient:   customClient,
		}

		// Reset attempts counter
		attempts = 0

		// This should succeed after one retry
		results, err := client.Search(t.Context(), "test query")

		if err != nil {
			t.Errorf("Expected success after retry, got error: %v", err)
		}

		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})

	// Test case 2: All retries fail
	t.Run("AllRetriesFail", func(t *testing.T) {
		attempts := 0

		// Create a custom HTTP client for testing
		customClient := &http.Client{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					attempts++
					return nil, errors.New("simulated persistent network error")
				},
			},
		}

		// Create a client with retry configuration and custom HTTP client
		client := &DuckDuckGoSearchClient{
			baseUrl:      "https://example.com/",
			maxRetries:   2,
			retryBackoff: 10, // Small backoff for faster tests
			httpClient:   customClient,
		}

		// This should fail after all retries
		_, err := client.Search(t.Context(), "test query")

		if err == nil {
			t.Error("Expected error after all retries, but got nil")
		}

		// Initial attempt + 2 retries = 3 attempts
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	// Test case 3: Context cancellation
	t.Run("ContextCancellation", func(t *testing.T) {
		attempts := 0

		// Create a custom HTTP client for testing
		customClient := &http.Client{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					attempts++
					// Simulate a slow response to allow context to be canceled
					time.Sleep(5 * time.Millisecond)
					return nil, errors.New("simulated network error")
				},
			},
		}

		// Create a client with retry configuration and custom HTTP client
		client := &DuckDuckGoSearchClient{
			baseUrl:      "https://example.com/",
			maxRetries:   2,
			retryBackoff: 10, // Small backoff for faster tests
			httpClient:   customClient,
		}

		// Create a context that will be canceled
		ctx, cancel := context.WithCancel(t.Context())

		// Cancel the context after a short delay
		go func() {
			time.Sleep(15 * time.Millisecond) // Should be enough time for first attempt and during first backoff
			cancel()
		}()

		// This should be interrupted by context cancellation
		_, err := client.Search(ctx, "test query")

		if err == nil {
			t.Error("Expected error due to context cancellation, but got nil")
		}

		// We expect at least 1 attempt before cancellation
		if attempts < 1 {
			t.Errorf("Expected at least 1 attempt, got %d", attempts)
		}
	})
}

// mockTransport implements the http.RoundTripper interface for testing.
type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestWithRetryConfig(t *testing.T) {
	client := NewDuckDuckGoSearchClient()

	// Default values
	if client.maxRetries != 3 {
		t.Errorf("Expected default maxRetries to be 3, got %d", client.maxRetries)
	}

	if client.retryBackoff != 500 {
		t.Errorf("Expected default retryBackoff to be 500, got %d", client.retryBackoff)
	}

	// Custom values
	client = client.WithRetryConfig(5, 200)

	if client.maxRetries != 5 {
		t.Errorf("Expected maxRetries to be 5, got %d", client.maxRetries)
	}

	if client.retryBackoff != 200 {
		t.Errorf("Expected retryBackoff to be 200, got %d", client.retryBackoff)
	}
}
