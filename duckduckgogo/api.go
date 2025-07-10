package duckduckgogo

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"gostreampuller/util"
)

type SearchClient interface {
	Search(ctx context.Context, query string) ([]Result, error)
	SearchLimited(ctx context.Context, query string, limit int) ([]Result, error)
}

type DuckDuckGoSearchClient struct {
	baseUrl      string
	maxRetries   int
	retryBackoff int // in milliseconds
	httpClient   *http.Client
}

func NewDuckDuckGoSearchClient() *DuckDuckGoSearchClient {
	return &DuckDuckGoSearchClient{
		baseUrl:      "https://duckduckgo.com/html/",
		maxRetries:   3,
		retryBackoff: 500,
		httpClient:   http.DefaultClient,
	}
}

// WithRetryConfig configures the retry behavior of the client.
func (c *DuckDuckGoSearchClient) WithRetryConfig(maxRetries, retryBackoff int) *DuckDuckGoSearchClient {
	c.maxRetries = maxRetries
	c.retryBackoff = retryBackoff
	return c
}
func (c *DuckDuckGoSearchClient) Search(ctx context.Context, query string) ([]Result, error) {
	return c.SearchLimited(ctx, query, 0)
}

func (c *DuckDuckGoSearchClient) SearchLimited(ctx context.Context, query string, limit int) ([]Result, error) {
	queryURLStr := c.baseUrl + "?q=" + url.QueryEscape(query)
	queryURL, err := url.Parse(queryURLStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", queryURLStr, err)
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	req.Header.Add("User-Agent", util.GetRandomUserAgent())

	var resp *http.Response
	var lastErr error

	// Implement retry with exponential backoff
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff duration with exponential increase
			backoff := time.Duration(c.retryBackoff*(1<<(attempt-1))) * time.Millisecond

			// Log retry attempt
			slog.Info("Retrying search request",
				"attempt", attempt,
				"max_retries", c.maxRetries,
				"backoff_ms", backoff.Milliseconds(),
				"query", query)

			// Wait before retrying
			select {
			case <-time.After(backoff):
				// Continue with retry
			case <-ctx.Done():
				// Context was canceled during backoff
				return nil, ctx.Err()
			}

			// Use a new user agent for each retry
			req.Header.Set("User-Agent", util.GetRandomUserAgent())
		}

		resp, err = c.httpClient.Do(req)
		if err == nil {
			break // Success, exit retry loop
		}

		lastErr = err
		slog.Error("Search request failed", "error", err, "attempt", attempt+1, "max_retries", c.maxRetries)

		// If this was the last attempt, we'll exit the loop with err still set
		if attempt == c.maxRetries {
			return nil, fmt.Errorf("all %d search attempts failed: %w", c.maxRetries+1, lastErr)
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("return status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0)
	doc.Find(".results .web-result").Each(func(i int, s *goquery.Selection) {
		if i > limit-1 && limit > 0 {
			return
		}
		results = append(results, c.collectResult(s))
	})
	return results, nil
}

func (c *DuckDuckGoSearchClient) collectResult(s *goquery.Selection) Result {
	resURLHTML := html(s.Find(".result__url").Html())
	resURL := clean(s.Find(".result__url").Text())
	titleHTML := html(s.Find(".result__a").Html())
	title := clean(s.Find(".result__a").Text())
	snippetHTML := html(s.Find(".result__snippet").Html())
	snippet := clean(s.Find(".result__snippet").Text())
	icon := s.Find(".result__icon__img")
	src, _ := icon.Attr("src")
	width, _ := icon.Attr("width")
	height, _ := icon.Attr("height")
	return Result{
		HTMLFormattedURL: resURLHTML,
		HTMLTitle:        titleHTML,
		HTMLSnippet:      snippetHTML,
		FormattedURL:     resURL,
		Title:            title,
		Snippet:          snippet,
		Icon: Icon{
			Src:    src,
			Width:  toInt(width),
			Height: toInt(height),
		},
	}
}

func html(html string, err error) string {
	if err != nil {
		return ""
	}
	return clean(html)
}

func clean(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "\n", ""))
}

func toInt(n string) int {
	res, err := strconv.Atoi(n)
	if err != nil {
		return 0
	}
	return res
}
