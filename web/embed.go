package web

import "embed"

// EmbeddedFiles contains the web UI static assets bundled into the binary.
//
//go:embed ui/* assets/*
var EmbeddedFiles embed.FS
