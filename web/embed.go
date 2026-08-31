package web

import "embed"

//go:embed upload.html gallery.html static/*
var FS embed.FS
