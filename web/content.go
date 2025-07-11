package web

import "embed"

//go:embed *.html
var Content embed.FS // Declare a variable to hold the embedded content
