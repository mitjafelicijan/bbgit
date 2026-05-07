package main

import (
	"time"
	"encoding/xml"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Repository struct {
	Name        string `yaml:"name" json:"name"`
	Path        string `yaml:"path" json:"path"`
	Description string `yaml:"description" json:"description"`
	Group       string `yaml:"group" json:"group"`
}

type Commit struct {
	Hash           string
	AuthorName     string
	AuthorEmail    string
	AuthorDate     time.Time
	CommitterName  string
	CommitterEmail string
	CommitterDate  time.Time
	Message        string
	Additions      int
	Deletions      int
}

type DiffLine struct {
	LeftNo  string
	Left    string
	RightNo string
	Right   string
	Type    string // "add", "del", "mod", "eq", "gap"
}

type FileDiff struct {
	Name     string
	Lines    []DiffLine
	Addition int
	Deletion int
	IsBinary bool
	Mode     string
	OldSize  int64
	NewSize  int64
	Deleted  bool
}

type GroupedRepositories struct {
	Name         string
	Repositories []Repository
}

type TreeEntry struct {
	Name  string
	Path  string
	IsDir bool
	Size  int64
	Mode  string
}

type RepoContext struct {
	Repo        *Repository
	GitRepo     *git.Repository
	Config      *Config
	CurrentRef  string
	Hash        plumbing.Hash
	Branches    []string
	Tags        []string
	ReadmeName  string
	LicenseName string
}

type Marker struct {
	Type     string // "TODO" or "FIXME"
	FilePath string
	Line     int
	Content  string
}

type LanguageStat struct {
	Name       string
	Percentage float64
	Color      string
	Size       int64
	Offset     float64
}

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate,omitempty"`
	Items       []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}
