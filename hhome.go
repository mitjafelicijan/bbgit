package main

import (
	"net/http"
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
