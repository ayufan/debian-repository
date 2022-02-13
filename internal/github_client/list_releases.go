package github_client

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
)

func (a *API) ListReleasesOneRepo(owner, repo string) (releases []*github.RepositoryRelease, resp *github.Response, err error) {
	cached, found := a.requestCache.Get(filepath.Join(owner, repo))
	if found {
		releases = cached.([]*github.RepositoryRelease)
		return
	}

	start := time.Now()
	releases, resp, err = a.client.Repositories.ListReleases(context.TODO(), owner, repo, &listOptions)

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

	a.requestCache.Add(filepath.Join(owner, repo), releases, cache.DefaultExpiration)
	return
}

func (a *API) ListReleasesInOrganization(owner string) (releases []*github.RepositoryRelease, resp *github.Response, err error) {
	repos, resp, err := a.ListProjects(owner)
	if err != nil {
		return nil, resp, err
	}

	var wg sync.WaitGroup
	var lock sync.Mutex

	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			repoReleases, resp2, err2 := a.ListReleasesOneRepo(owner, repo)
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

func (a *API) ListReleases(owner, repo string) (releases []*github.RepositoryRelease, resp *github.Response, err error) {
	if repo != "" {
		return a.ListReleasesOneRepo(owner, repo)
	}
	return a.ListReleasesInOrganization(owner)
}
