package main

import (
	"fmt"
	"html/template"
	"net/http"
)

func readmeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ctx.ReadmeName == "" {
		http.NotFound(w, r)
		return
	}

	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := commit.File(ctx.ReadmeName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	content, err := file.Contents()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		*RepoContext
		Content template.HTML
	}{
		RepoContext: ctx,
		Content:     renderMarkdown(content),
	}

	err = templates.ExecuteTemplate(w, "readme.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func licenseHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ctx.LicenseName == "" {
		http.NotFound(w, r)
		return
	}

	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := commit.File(ctx.LicenseName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	content, err := file.Contents()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		*RepoContext
		Content template.HTML
	}{
		RepoContext: ctx,
		Content:     renderMarkdown(content),
	}

	err = templates.ExecuteTemplate(w, "license.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func markersHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	markers, err := scanMarkers(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error scanning markers: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		*RepoContext
		Markers []Marker
	}{
		RepoContext: ctx,
		Markers:     markers,
	}

	err = templates.ExecuteTemplate(w, "markers.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
