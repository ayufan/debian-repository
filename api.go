package main

import (
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
)

var requestCache *cache.Cache

var listOptions = github.ListOptions{PerPage: 10000}

func listReleasesOneRepo(owner, repo string) (releases []github.RepositoryRelease, resp *github.Response, err error) {
	cached, found := requestCache.Get(filepath.Join(owner, repo))
	if found {
		releases = cached.([]github.RepositoryRelease)
		return
	}

	start := time.Now()
	releases, resp, err = client.Repositories.ListReleases(owner, repo, &listOptions)

	var rate github.Rate
	if resp != nil {
		rate = resp.Rate
	}
	log.Println("listReleases:",
		"owner:", owner,
		"repo:", repo,
		"releases:", len(releases),
		"error:", err,
		"limits:", rate,
		"duration:", time.Since(start))

	if err != nil {
		return
	}

	requestCache.Add(filepath.Join(owner, repo), releases, cache.DefaultExpiration)
	return
}

func listProjects(owner string) (repos []github.Repository, resp *github.Response, err error) {
	cached, found := requestCache.Get(owner)
	if found {
		repos = cached.([]github.Repository)
		return
	}

	start := time.Now()
	repos, resp, err = client.Repositories.List(owner, &github.RepositoryListOptions{ListOptions: listOptions})

	var rate github.Rate
	if resp != nil {
		rate = resp.Rate
	}
	log.Println("listProjects:",
		"owner:", owner,
		"repos:", len(repos),
		"error:", err,
		"limits:", rate,
		"duration:", time.Since(start))

	if err != nil {
		return
	}

	requestCache.Add(owner, repos, cache.DefaultExpiration)
	return
}

func listReleasesInOrganization(owner string) (releases []github.RepositoryRelease, resp *github.Response, err error) {
	repos, resp, err := listProjects(owner)
	if err != nil {
		return nil, resp, err
	}

	var wg sync.WaitGroup
	var lock sync.Mutex

	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			repoReleases, resp2, err2 := listReleasesOneRepo(owner, repo)
			if resp2 != nil {
				resp = resp2
			}
			if err2 != nil {
				err = err2
				return
			}

			lock.Lock()
			defer lock.Unlock()
			releases = append(releases, repoReleases...)
		}(*repo.Name)
	}
	wg.Wait()
	return
}

func listReleases(owner, repo string) (releases []github.RepositoryRelease, resp *github.Response, err error) {
	if repo != "" {
		return listReleasesOneRepo(owner, repo)
	}
	return listReleasesInOrganization(owner)
}
