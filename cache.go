package main

import (
	"encoding/gob"
	"log"
	"os"
	"sync"
	"time"
)

type CommitStat struct {
	Additions int
	Deletions int
}

type RepoMetadata struct {
	Branches     []string
	Tags         []string
	ReadmeName   string
	LicenseName  string
	TotalCommits int
	Version      int
}

const CurrentMetadataVersion = 1

type LangCacheKey struct {
	RepoName   string
	CommitHash string
}

var (
	commitStatsCache  sync.Map // string -> CommitStat
	repoMetadataCache sync.Map // string -> RepoMetadata
	langCache         sync.Map // LangCacheKey -> []LanguageStat
	saveChan          = make(chan struct{}, 1)
)

const cacheFile = "bbgit.cache"

func NotifySave() {
	select {
	case saveChan <- struct{}{}:
	default:
	}
}

func StartSaver() {
	go func() {
		for range saveChan {
			// Throttle saves to once per second
			time.Sleep(1 * time.Second)
			// Drain any additional signals during sleep
			for len(saveChan) > 0 {
				<-saveChan
			}
			if err := SaveCache(); err != nil {
				log.Printf("Error saving cache: %v", err)
			}
		}
	}()
}

type cacheContainer struct {
	CommitStats  map[string]CommitStat
	RepoMetadata map[string]RepoMetadata
	LangStats    map[LangCacheKey][]LanguageStat
}

func SaveCache() error {
	container := cacheContainer{
		CommitStats:  make(map[string]CommitStat),
		RepoMetadata: make(map[string]RepoMetadata),
		LangStats:    make(map[LangCacheKey][]LanguageStat),
	}

	commitStatsCache.Range(func(k, v interface{}) bool {
		container.CommitStats[k.(string)] = v.(CommitStat)
		return true
	})

	repoMetadataCache.Range(func(k, v interface{}) bool {
		container.RepoMetadata[k.(string)] = v.(RepoMetadata)
		return true
	})

	langCache.Range(func(k, v interface{}) bool {
		container.LangStats[k.(LangCacheKey)] = v.([]LanguageStat)
		return true
	})

	f, err := os.Create(cacheFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := gob.NewEncoder(f).Encode(container); err != nil {
		return err
	}

	log.Printf("OPS Cache saved to %s", cacheFile)
	return nil
}

func LoadCache() {
	f, err := os.Open(cacheFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error opening cache file: %v", err)
		}
		return
	}
	defer f.Close()

	var container cacheContainer
	if err := gob.NewDecoder(f).Decode(&container); err != nil {
		log.Printf("Error decoding cache file: %v", err)
		return
	}

	for k, v := range container.CommitStats {
		commitStatsCache.Store(k, v)
	}
	for k, v := range container.RepoMetadata {
		repoMetadataCache.Store(k, v)
	}
	for k, v := range container.LangStats {
		langCache.Store(k, v)
	}

	log.Printf("Loaded cache from %s", cacheFile)
}
