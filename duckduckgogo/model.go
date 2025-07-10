package duckduckgogo

type Result struct {
	HTMLFormattedURL string `json:"htmlFormattedUrl,omitempty"`

	HTMLTitle string `json:"htmlTitle,omitempty"`

	HTMLSnippet string `json:"htmlSnippet,omitempty"`

	FormattedURL string `json:"formattedUrl,omitempty"`

	Title string `json:"title,omitempty"`

	Snippet string `json:"snippet,omitempty"`

	Icon Icon `json:"icon,omitempty"`
}

type Icon struct {
	Src    string `json:"src,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}
