package duckduckgogo

import (
	"encoding/json"
	"testing"
)

func TestResultSerialization(t *testing.T) {
	// Create a test result
	result := Result{
		HTMLFormattedURL: "<b>example.com</b>",
		HTMLTitle:        "<b>Test</b> Title",
		HTMLSnippet:      "This is a <b>test</b> snippet",
		FormattedURL:     "example.com",
		Title:            "Test Title",
		Snippet:          "This is a test snippet",
		Icon: Icon{
			Src:    "icon.png",
			Width:  16,
			Height: 16,
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal Result to JSON: %v", err)
	}

	// Deserialize back to Result
	var deserializedResult Result
	err = json.Unmarshal(jsonData, &deserializedResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON to Result: %v", err)
	}

	// Verify fields match
	if deserializedResult.Title != result.Title {
		t.Errorf("Title mismatch: expected %q, got %q", result.Title, deserializedResult.Title)
	}
	if deserializedResult.Snippet != result.Snippet {
		t.Errorf("Snippet mismatch: expected %q, got %q", result.Snippet, deserializedResult.Snippet)
	}
	if deserializedResult.FormattedURL != result.FormattedURL {
		t.Errorf("FormattedUrl mismatch: expected %q, got %q", result.FormattedURL, deserializedResult.FormattedURL)
	}
	if deserializedResult.Icon.Src != result.Icon.Src {
		t.Errorf("Icon.Src mismatch: expected %q, got %q", result.Icon.Src, deserializedResult.Icon.Src)
	}
	if deserializedResult.Icon.Width != result.Icon.Width {
		t.Errorf("Icon.Width mismatch: expected %d, got %d", result.Icon.Width, deserializedResult.Icon.Width)
	}
	if deserializedResult.Icon.Height != result.Icon.Height {
		t.Errorf("Icon.Height mismatch: expected %d, got %d", result.Icon.Height, deserializedResult.Icon.Height)
	}
}

func TestIconSerialization(t *testing.T) {
	// Create a test icon
	icon := Icon{
		Src:    "icon.png",
		Width:  16,
		Height: 16,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(icon)
	if err != nil {
		t.Fatalf("Failed to marshal Icon to JSON: %v", err)
	}

	// Deserialize back to Icon
	var deserializedIcon Icon
	err = json.Unmarshal(jsonData, &deserializedIcon)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON to Icon: %v", err)
	}

	// Verify fields match
	if deserializedIcon.Src != icon.Src {
		t.Errorf("Src mismatch: expected %q, got %q", icon.Src, deserializedIcon.Src)
	}
	if deserializedIcon.Width != icon.Width {
		t.Errorf("Width mismatch: expected %d, got %d", icon.Width, deserializedIcon.Width)
	}
	if deserializedIcon.Height != icon.Height {
		t.Errorf("Height mismatch: expected %d, got %d", icon.Height, deserializedIcon.Height)
	}
}
