package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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
