package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	config := GlobalConfig

	groupsMap := make(map[string][]Repository)
	var groupOrder []string

	for _, repo := range config.Repositories {
		if _, ok := groupsMap[repo.Group]; !ok {
			groupOrder = append(groupOrder, repo.Group)
		}
		groupsMap[repo.Group] = append(groupsMap[repo.Group], repo)
	}

	var grouped []GroupedRepositories
	for _, groupName := range groupOrder {
		grouped = append(grouped, GroupedRepositories{
			Name:         groupName,
			Repositories: groupsMap[groupName],
		})
	}

	err := templates.ExecuteTemplate(w, "repositories.html", struct {
		Groups []GroupedRepositories
		Repo   *Repository
	}{
		Groups: grouped,
		Repo:   nil,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func repoHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		if err.Error() == "repository not found" {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 30

	totalCommitsKey := ctx.Repo.Name + ":" + ctx.Hash.String()
	var totalCommits int
	if val, ok := repoMetadataCache.Load(totalCommitsKey); ok {
		totalCommits = val.(RepoMetadata).TotalCommits
	} else {
		cIter, err := ctx.GitRepo.Log(&git.LogOptions{From: ctx.Hash})
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting log: %v", err), http.StatusInternalServerError)
			return
		}
		_ = cIter.ForEach(func(c *object.Commit) error {
			totalCommits++
			return nil
		})
		repoMetadataCache.Store(totalCommitsKey, RepoMetadata{
			TotalCommits: totalCommits,
			Branches:     ctx.Branches,
			Tags:         ctx.Tags,
			ReadmeName:   ctx.ReadmeName,
			LicenseName:  ctx.LicenseName,
			Version:      CurrentMetadataVersion,
		})
		NotifySave()
	}

	totalPages := (totalCommits + pageSize - 1) / pageSize

	cIter, err := ctx.GitRepo.Log(&git.LogOptions{From: ctx.Hash})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting log: %v", err), http.StatusInternalServerError)
		return
	}

	var commits []Commit
	var wg sync.WaitGroup
	count := 0

	// Collect commits for the current page
	var commitsToProcess []*object.Commit
	err = cIter.ForEach(func(c *object.Commit) error {
		if count < (page-1)*pageSize {
			count++
			return nil
		}
		if len(commitsToProcess) >= pageSize {
			return fmt.Errorf("limit reached")
		}
		commitsToProcess = append(commitsToProcess, c)
		count++
		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		http.Error(w, fmt.Sprintf("Error iterating commits: %v", err), http.StatusInternalServerError)
		return
	}

	commits = make([]Commit, len(commitsToProcess))
	for i, c := range commitsToProcess {
		wg.Add(1)
		go func(idx int, commit *object.Commit) {
			defer wg.Done()
			hashStr := commit.Hash.String()

			var adds, dels int
			if val, ok := commitStatsCache.Load(hashStr); ok {
				s := val.(CommitStat)
				adds, dels = s.Additions, s.Deletions
			} else {
				stats, err := commit.Stats()
				if err == nil {
					for _, st := range stats {
						adds += st.Addition
						dels += st.Deletion
					}
					commitStatsCache.Store(hashStr, CommitStat{Additions: adds, Deletions: dels})
					NotifySave()
				}
			}

			commits[idx] = Commit{
				Hash:           hashStr,
				AuthorName:     commit.Author.Name,
				AuthorEmail:    commit.Author.Email,
				AuthorDate:     commit.Author.When,
				CommitterName:  commit.Committer.Name,
				CommitterEmail: commit.Committer.Email,
				CommitterDate:  commit.Committer.When,
				Message:        commit.Message,
				Additions:      adds,
				Deletions:      dels,
			}
		}(i, c)
	}
	wg.Wait()

	// Calculate page range (show up to 8 pages around current page)
	startPage := page - 4
	if startPage < 1 {
		startPage = 1
	}
	endPage := startPage + 7
	if endPage > totalPages {
		endPage = totalPages
		startPage = endPage - 7
		if startPage < 1 {
			startPage = 1
		}
	}

	var pages []int
	for i := startPage; i <= endPage; i++ {
		pages = append(pages, i)
	}

	commit, _ := ctx.GitRepo.CommitObject(ctx.Hash)
	tree, _ := commit.Tree()
	langStats, _ := getLanguageStats(ctx.Repo.Name, ctx.Hash.String(), tree)

	data := struct {
		*RepoContext
		Commits    []Commit
		Languages  []LanguageStat
		View       string
		Page       int
		TotalPages int
		Pages      []int
		PrevPage   int
		NextPage   int
	}{
		RepoContext: ctx,
		Commits:     commits,
		Languages:   langStats,
		View:        "commits",
		Page:        page,
		TotalPages:  totalPages,
		Pages:       pages,
		PrevPage:    page - 1,
	}
	if page < totalPages {
		data.NextPage = page + 1
	}

	err = templates.ExecuteTemplate(w, "repository.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func treeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.PathValue("path")
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

	if path != "" {
		tree, err = tree.Tree(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	var entries []TreeEntry
	for _, entry := range tree.Entries {
		fullPath := entry.Name
		if path != "" {
			fullPath = path + "/" + entry.Name
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
		Path:        path,
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

	path := r.PathValue("path")
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := commit.File(path)
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
		Path:        path,
		Content:     highlight(path, content),
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

	path := r.PathValue("path")
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := commit.File(path)
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path))
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

func commitHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	commitHash := r.PathValue("hash")
	hash := plumbing.NewHash(commitHash)
	commit, err := ctx.GitRepo.CommitObject(hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	var fileDiffs []FileDiff
	maxChanges := 0
	if commit.NumParents() > 0 {
		parent, _ := commit.Parent(0)
		patch, err := parent.Patch(commit)
		if err == nil {
			for _, fp := range patch.FilePatches() {
				from, to := fp.Files()
				name := ""
				mode := ""
				if to != nil {
					name = to.Path()
					mode = formatMode(to.Mode())
				} else if from != nil {
					name = from.Path()
					mode = formatMode(from.Mode())
				}

				fileAdd, fileDel := 0, 0
				isBinary := fp.IsBinary()
				deleted := to == nil
				var oldSize, newSize int64

				if isBinary {
					if from != nil {
						obj, _ := ctx.GitRepo.Object(plumbing.AnyObject, from.Hash())
						if blob, ok := obj.(*object.Blob); ok {
							oldSize = blob.Size
						}
					}
					if to != nil {
						obj, _ := ctx.GitRepo.Object(plumbing.AnyObject, to.Hash())
						if blob, ok := obj.(*object.Blob); ok {
							newSize = blob.Size
						}
					}
				}

				var diffLines []DiffLine
				leftNo, rightNo := 1, 1

				var delLines []string
				var addLines []string

				flush := func() {
					max := len(delLines)
					if len(addLines) > max {
						max = len(addLines)
					}
					for i := 0; i < max; i++ {
						line := DiffLine{}
						if i < len(delLines) && i < len(addLines) {
							line.LeftNo = fmt.Sprintf("%d", leftNo)
							line.Left = delLines[i]
							line.RightNo = fmt.Sprintf("%d", rightNo)
							line.Right = addLines[i]
							line.Type = "mod"
							leftNo++
							rightNo++
							fileAdd++
							fileDel++
						} else if i < len(delLines) {
							line.LeftNo = fmt.Sprintf("%d", leftNo)
							line.Left = delLines[i]
							line.Type = "del"
							leftNo++
							fileDel++
						} else if i < len(addLines) {
							line.RightNo = fmt.Sprintf("%d", rightNo)
							line.Right = addLines[i]
							line.Type = "add"
							rightNo++
							fileAdd++
						}
						diffLines = append(diffLines, line)
					}
					delLines = nil
					addLines = nil
				}

				for _, chunk := range fp.Chunks() {
					lines := strings.Split(strings.TrimSuffix(chunk.Content(), "\n"), "\n")
					switch chunk.Type() {
					case diff.Equal:
						flush()
						for _, line := range lines {
							diffLines = append(diffLines, DiffLine{
								LeftNo:  fmt.Sprintf("%d", leftNo),
								Left:    line,
								RightNo: fmt.Sprintf("%d", rightNo),
								Right:   line,
								Type:    "eq",
							})
							leftNo++
							rightNo++
						}
					case diff.Delete:
						delLines = append(delLines, lines...)
					case diff.Add:
						addLines = append(addLines, lines...)
					}
				}
				flush()

				if fileAdd+fileDel > maxChanges {
					maxChanges = fileAdd + fileDel
				}

				visible := make([]bool, len(diffLines))
				for i, line := range diffLines {
					if line.Type == "add" || line.Type == "del" || line.Type == "mod" {
						for j := i - 3; j <= i+3; j++ {
							if j >= 0 && j < len(diffLines) {
								visible[j] = true
							}
						}
					}
				}

				var filteredLines []DiffLine
				lastWasGap := false
				for i, isVisible := range visible {
					if isVisible {
						filteredLines = append(filteredLines, diffLines[i])
						lastWasGap = false
					} else {
						if !lastWasGap {
							filteredLines = append(filteredLines, DiffLine{Type: "gap"})
							lastWasGap = true
						}
					}
				}

				fileDiffs = append(fileDiffs, FileDiff{
					Name:     name,
					Lines:    filteredLines,
					Addition: fileAdd,
					Deletion: fileDel,
					IsBinary: isBinary,
					Mode:     mode,
					OldSize:  oldSize,
					NewSize:  newSize,
					Deleted:  deleted,
				})
			}
		}
	}

	stats, _ := commit.Stats()
	adds, dels := 0, 0
	for _, s := range stats {
		adds += s.Addition
		dels += s.Deletion
	}

	data := struct {
		*RepoContext
		Commit     Commit
		FileDiffs  []FileDiff
		MaxChanges int
	}{
		RepoContext: ctx,
		Commit: Commit{
			Hash:           commit.Hash.String(),
			AuthorName:     commit.Author.Name,
			AuthorEmail:    commit.Author.Email,
			AuthorDate:     commit.Author.When,
			CommitterName:  commit.Committer.Name,
			CommitterEmail: commit.Committer.Email,
			CommitterDate:  commit.Committer.When,
			Message:        commit.Message,
			Additions:      adds,
			Deletions:      dels,
		},
		FileDiffs:  fileDiffs,
		MaxChanges: maxChanges,
	}

	err = templates.ExecuteTemplate(w, "commit.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func patchHandler(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("name")
	commitHash := r.PathValue("hash")

	config, err := loadConfig(ConfigPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading config: %v", err), http.StatusInternalServerError)
		return
	}

	var repo *Repository
	for _, repoItem := range config.Repositories {
		if repoItem.Name == repoName {
			repo = &repoItem
			break
		}
	}

	if repo == nil {
		http.NotFound(w, r)
		return
	}

	gitRepo, err := git.PlainOpen(repo.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error opening repository: %v", err), http.StatusInternalServerError)
		return
	}

	hash := plumbing.NewHash(commitHash)
	commit, err := gitRepo.CommitObject(hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting commit: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	currentTree, err := commit.Tree()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting tree: %v", err), http.StatusInternalServerError)
		return
	}

	var parentTree *object.Tree
	if commit.NumParents() > 0 {
		parent, _ := commit.Parent(0)
		parentTree, err = parent.Tree()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting parent tree: %v", err), http.StatusInternalServerError)
			return
		}
	}

	patch, err := parentTree.Patch(currentTree)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error generating patch: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, patch.String())
}

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

func repoCommitsRSSHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := getRepoContext(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cIter, err := ctx.GitRepo.Log(&git.LogOptions{From: ctx.Hash})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting log: %v", err), http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	rss := RSS{
		Version: "2.0",
		Channel: Channel{
			Title:       fmt.Sprintf("%s - Commits", ctx.Repo.Name),
			Link:        fmt.Sprintf("%s/r/%s?ref=%s", baseURL, ctx.Repo.Name, ctx.CurrentRef),
			Description: fmt.Sprintf("Commit history for %s (%s)", ctx.Repo.Name, ctx.CurrentRef),
		},
	}

	count := 0
	err = cIter.ForEach(func(c *object.Commit) error {
		if count >= 20 {
			return fmt.Errorf("limit reached")
		}

		hash := c.Hash.String()
		item := RSSItem{
			Title:       strings.Split(c.Message, "\n")[0],
			Link:        fmt.Sprintf("%s/r/%s/c/%s", baseURL, ctx.Repo.Name, hash),
			Description: c.Message,
			PubDate:     c.Author.When.Format(time.RFC1123Z),
			GUID:        fmt.Sprintf("%s/r/%s/c/%s", baseURL, ctx.Repo.Name, hash),
		}
		rss.Channel.Items = append(rss.Channel.Items, item)
		count++
		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		http.Error(w, fmt.Sprintf("Error iterating commits: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(rss); err != nil {
		log.Printf("Error encoding RSS: %v", err)
	}
}

func repoTagsRSSHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	config := GlobalConfig

	var repo *Repository
	for _, repoItem := range config.Repositories {
		if repoItem.Name == name {
			repo = &repoItem
			break
		}
	}

	if repo == nil {
		http.NotFound(w, r)
		return
	}

	gitRepo, err := git.PlainOpen(repo.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error opening repository: %v", err), http.StatusInternalServerError)
		return
	}

	tIter, err := gitRepo.Tags()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting tags: %v", err), http.StatusInternalServerError)
		return
	}

	type tagInfo struct {
		Name string
		Date time.Time
		Hash string
	}
	var tags []tagInfo

	err = tIter.ForEach(func(ref *plumbing.Reference) error {
		obj, err := gitRepo.TagObject(ref.Hash())
		if err != nil {
			// Lightweight tag
			commit, err := gitRepo.CommitObject(ref.Hash())
			if err == nil {
				tags = append(tags, tagInfo{
					Name: ref.Name().Short(),
					Date: commit.Author.When,
					Hash: ref.Hash().String(),
				})
			}
		} else {
			// Annotated tag
			if _, err := obj.Commit(); err == nil {
				tags = append(tags, tagInfo{
					Name: ref.Name().Short(),
					Date: obj.Tagger.When,
					Hash: ref.Hash().String(),
				})
			} else {
				tags = append(tags, tagInfo{
					Name: ref.Name().Short(),
					Date: obj.Tagger.When,
					Hash: obj.Target.String(),
				})
			}
		}
		return nil
	})

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Date.After(tags[j].Date)
	})

	if len(tags) > 20 {
		tags = tags[:20]
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	rss := RSS{
		Version: "2.0",
		Channel: Channel{
			Title:       fmt.Sprintf("%s - Tags", repo.Name),
			Link:        fmt.Sprintf("%s/r/%s", baseURL, repo.Name),
			Description: fmt.Sprintf("Tags for %s", repo.Name),
		},
	}

	for _, t := range tags {
		item := RSSItem{
			Title:       t.Name,
			Link:        fmt.Sprintf("%s/r/%s?ref=%s", baseURL, repo.Name, t.Name),
			Description: fmt.Sprintf("Tag %s at %s", t.Name, t.Hash),
			PubDate:     t.Date.Format(time.RFC1123Z),
			GUID:        fmt.Sprintf("%s/r/%s/tags/%s", baseURL, repo.Name, t.Name),
		}
		rss.Channel.Items = append(rss.Channel.Items, item)
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(rss); err != nil {
		log.Printf("Error encoding RSS: %v", err)
	}
}
