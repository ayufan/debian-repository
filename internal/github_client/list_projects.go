package github_client

import (
	"log"
	"time"

	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
)

func (a *API) ListProjects(owner string) (repos []github.Repository, resp *github.Response, err error) {
	cached, found := a.requestCache.Get(owner)
	if found {
		repos = cached.([]github.Repository)
		return
	}

	start := time.Now()
	repos, resp, err = a.client.Repositories.List(owner, &github.RepositoryListOptions{ListOptions: listOptions})

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

	a.requestCache.Add(owner, repos, cache.DefaultExpiration)
	return
}
