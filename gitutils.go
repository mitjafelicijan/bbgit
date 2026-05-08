package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

func highlight(filename, content string) template.HTML {
	lexer := lexers.Get(filename)
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("vs")
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(html.WithLineNumbers(true), html.WithLinkableLineNumbers(true, "L"))

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(content))
	}

	var sb strings.Builder
	err = formatter.Format(&sb, style, iterator)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(content))
	}

	return template.HTML(sb.String())
}

func scanMarkers(ctx *RepoContext) ([]Marker, error) {
	commit, err := ctx.GitRepo.CommitObject(ctx.Hash)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var markers []Marker
	err = tree.Files().ForEach(func(f *object.File) error {
		// Skip vendor directory and binary files
		if strings.HasPrefix(f.Name, "vendor/") || strings.Contains(f.Name, "/vendor/") {
			return nil
		}
		if f.Mode.IsFile() && f.Size > 1024*1024 { // Skip files > 1MB
			return nil
		}

		isSource := false
		exts := []string{".go", ".js", ".ts", ".py", ".c", ".h", ".cpp", ".hpp", ".css", ".html", ".md", ".txt", ".yaml", ".yml", ".sh"}
		for _, ext := range exts {
			if strings.HasSuffix(strings.ToLower(f.Name), ext) {
				isSource = true
				break
			}
		}

		if !isSource {
			return nil
		}

		content, err := f.Contents()
		if err != nil {
			return nil
		}

		lines := strings.Split(content, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			markerType := ""
			if strings.Contains(trimmed, "TODO:") {
				markerType = "TODO"
			} else if strings.Contains(trimmed, "FIXME:") {
				markerType = "FIXME"
			}

			if markerType != "" {
				// Extract content after the marker
				idx := strings.Index(trimmed, markerType+":")
				content := strings.TrimSpace(trimmed[idx+len(markerType)+1:])
				markers = append(markers, Marker{
					Type:     markerType,
					FilePath: f.Name,
					Line:     i + 1,
					Content:  content,
				})
			}
		}
		return nil
	})

	return markers, err
}

func getRepoContext(w http.ResponseWriter, r *http.Request) (*RepoContext, error) {
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
		return nil, fmt.Errorf("repository not found")
	}

	gitRepo, err := git.PlainOpen(repo.Path)
	if err != nil {
		return nil, fmt.Errorf("error opening repository: %w", err)
	}

	refName := r.URL.Query().Get("ref")
	var hash plumbing.Hash

	if refName == "" {
		head, err := gitRepo.Head()
		if err != nil {
			return nil, fmt.Errorf("error getting HEAD: %w", err)
		}
		hash = head.Hash()
		refName = head.Name().Short()
	} else {
		h, err := gitRepo.ResolveRevision(plumbing.Revision(refName))
		if err != nil {
			return nil, fmt.Errorf("error resolving revision: %w", err)
		}
		hash = *h
	}

	metadataKey := name + ":" + hash.String()

	if val, ok := repoMetadataCache.Load(metadataKey); ok {
		meta := val.(RepoMetadata)
		if meta.Version >= CurrentMetadataVersion {
			return &RepoContext{
				Repo:        repo,
				GitRepo:     gitRepo,
				Config:      config,
				CurrentRef:  refName,
				Hash:        hash,
				Branches:    meta.Branches,
				Tags:        meta.Tags,
				ReadmeName:  meta.ReadmeName,
				LicenseName: meta.LicenseName,
			}, nil
		}
	}

	// Get all branch-like references (local and remote)
	var branches []string
	rIter, err := gitRepo.References()
	seenBranches := make(map[string]bool)
	if err == nil {
		rIter.ForEach(func(r *plumbing.Reference) error {
			if r.Name().IsBranch() || r.Name().IsRemote() {
				name := r.Name().Short()
				if name == "origin/HEAD" || seenBranches[name] {
					return nil
				}
				branches = append(branches, name)
				seenBranches[name] = true
			}
			return nil
		})
	}

	// Get tags
	var tags []string
	tIter, err := gitRepo.Tags()
	if err == nil {
		tIter.ForEach(func(r *plumbing.Reference) error {
			tags = append(tags, r.Name().Short())
			return nil
		})
	}

	// Detect README and LICENSE
	readmeName := ""
	licenseName := ""
	commit, err := gitRepo.CommitObject(hash)
	if err == nil {
		tree, err := commit.Tree()
		if err == nil {
			for _, entry := range tree.Entries {
				nameLower := strings.ToLower(entry.Name)
				if readmeName == "" && (nameLower == "readme.md" || nameLower == "readme.markdown" || nameLower == "readme") {
					readmeName = entry.Name
				}
				if licenseName == "" && (nameLower == "license" || nameLower == "license.md" || nameLower == "license.txt" || nameLower == "licence" || nameLower == "licence.md" || nameLower == "licence.txt" || nameLower == "copying" || nameLower == "copying.md" || nameLower == "copying.txt") {
					licenseName = entry.Name
				}
				if readmeName != "" && licenseName != "" {
					break
				}
			}
		}
	}

	// Note: totalCommits will be updated in repoHandler if needed,
	// for now we store what we have.
	repoMetadataCache.Store(metadataKey, RepoMetadata{
		Branches:    branches,
		Tags:        tags,
		ReadmeName:  readmeName,
		LicenseName: licenseName,
		Version:     CurrentMetadataVersion,
	})
	NotifySave()

	return &RepoContext{
		Repo:        repo,
		GitRepo:     gitRepo,
		Config:      config,
		CurrentRef:  refName,
		Hash:        hash,
		Branches:    branches,
		Tags:        tags,
		ReadmeName:  readmeName,
		LicenseName: licenseName,
	}, nil
}

func formatMode(m filemode.FileMode) string {
	if m == filemode.Empty {
		return "----------"
	}
	s := m.String()
	switch m {
	case filemode.Regular:
		return "-rw-r--r--"
	case filemode.Executable:
		return "-rwxr-xr-x"
	case filemode.Dir:
		return "drwxr-xr-x"
	case filemode.Symlink:
		return "lrwxrwxrwx"
	default:
		return s
	}
}

func renderMarkdown(content string) template.HTML {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
			extension.Table,
			AlertExtension,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return template.HTML(content)
	}
	return template.HTML(buf.String())
}
