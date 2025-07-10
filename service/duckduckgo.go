package service

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"home-go-api-template/duckduckgogo"
)

const (
	RATE_LIMIT_DELAY  = time.Second
	RATE_LIMIT_EVENTS = 10
)

// SearchService defines the interface for search operations.
type SearchService interface {
	Search(query string, limit int) ([]SearchResult, error)
}

// SearchResult represents a search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// DuckDuckGoService implements SearchService using DuckDuckGo.
type DuckDuckGoService struct {
	client      duckduckgogo.SearchClient
	rateLimiter *rate.Limiter
}

// NewDuckDuckGoService creates a new DuckDuckGo search service.
func NewDuckDuckGoService() *DuckDuckGoService {
	return &DuckDuckGoService{
		client:      duckduckgogo.NewDuckDuckGoSearchClient(),
		rateLimiter: rate.NewLimiter(rate.Every(RATE_LIMIT_DELAY), RATE_LIMIT_EVENTS), // 10 requests per second
	}
}

// WithRetryConfig configures the retry behavior of the service.
func (s *DuckDuckGoService) WithRetryConfig(maxRetries, retryBackoff int) *DuckDuckGoService {
	if client, ok := s.client.(*duckduckgogo.DuckDuckGoSearchClient); ok {
		s.client = client.WithRetryConfig(maxRetries, retryBackoff)
	} else {
		// Handle the case where the client is a mock for testing
		if mockClient, ok := s.client.(interface {
			WithRetryConfig(int, int) duckduckgogo.SearchClient
		}); ok {
			s.client = mockClient.WithRetryConfig(maxRetries, retryBackoff)
		}
	}
	return s
}

// Search performs a search with the given query and limit.
func (s *DuckDuckGoService) Search(query string, limit int) ([]SearchResult, error) {
	// Wait for rate limiter
	ctx := context.Background()
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	results, err := s.client.SearchLimited(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			Title:   r.Title,
			URL:     r.FormattedURL,
			Snippet: r.Snippet,
		}
	}

	return searchResults, nil
}
