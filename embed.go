package main

import "embed"

//go:embed views/*.html
var viewsFS embed.FS

//go:embed static/*
var staticFS embed.FS
