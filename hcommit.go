package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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
