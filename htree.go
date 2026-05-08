package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func treeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pathValue := r.PathValue("path")
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	tree, err := commit.Tree()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting tree: %v", err), http.StatusInternalServerError)
		return
	}

	if pathValue != "" {
		tree, err = tree.Tree(pathValue)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	var entries []TreeEntry
	for _, entry := range tree.Entries {
		fullPath := entry.Name
		if pathValue != "" {
			fullPath = pathValue + "/" + entry.Name
		}

		isDir := entry.Mode.IsFile() == false

		var size int64
		if !isDir {
			obj, _ := ctx.GitRepo.Object(plumbing.AnyObject, entry.Hash)
			if blob, ok := obj.(*object.Blob); ok {
				size = blob.Size
			}
		}

		entries = append(entries, TreeEntry{
			Name:  entry.Name,
			Path:  fullPath,
			IsDir: isDir,
			Size:  size,
			Mode:  entry.Mode.String(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	data := struct {
		*RepoContext
		Entries []TreeEntry
		Path    string
		View    string
	}{
		RepoContext: ctx,
		Entries:     entries,
		Path:        pathValue,
		View:        "tree",
	}

	err = templates.ExecuteTemplate(w, "tree.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func blobHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pathValue := r.PathValue("path")
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := commit.File(pathValue)
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
		Path    string
		Content template.HTML
	}{
		RepoContext: ctx,
		Path:        pathValue,
		Content:     highlight(pathValue, content),
	}

	err = templates.ExecuteTemplate(w, "blob.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func rawHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pathValue := r.PathValue("path")
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := commit.File(pathValue)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	reader, err := file.Reader()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", pathValue))
	io.Copy(w, reader)
}

func archiveHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pathValue := r.PathValue("path")
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	tree, err := commit.Tree()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting tree: %v", err), http.StatusInternalServerError)
		return
	}

	if pathValue != "" {
		tree, err = tree.Tree(pathValue)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	filename := ctx.Repo.Name
	if pathValue != "" {
		filename = path.Base(pathValue)
	}
	filename = fmt.Sprintf("%s-%s.tar.gz", filename, ctx.CurrentRef)

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = tree.Files().ForEach(func(f *object.File) error {
		hdr := &tar.Header{
			Name: strings.TrimPrefix(strings.TrimPrefix(f.Name, pathValue), "/"),
			Mode: int64(f.Mode),
			Size: f.Size,
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		reader, err := f.Reader()
		if err != nil {
			return err
		}
		defer reader.Close()

		_, err = io.Copy(tw, reader)
		return err
	})

	if err != nil {
		log.Printf("Error creating archive: %v", err)
	}
}
