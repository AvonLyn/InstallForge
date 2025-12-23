package webembed

import "embed"

// Content holds the compiled frontend assets.
//go:embed web/*
var Content embed.FS
