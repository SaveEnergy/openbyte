package web

import "embed"

//go:embed *.html *.css *.js *.svg fonts/*
var Assets embed.FS
