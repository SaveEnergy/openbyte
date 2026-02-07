package web

import "embed"

//go:embed *.html *.css *.js fonts/*
var Assets embed.FS
