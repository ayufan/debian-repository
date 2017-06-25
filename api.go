package main

import (
	"path/filepath"

	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
)

var requestCache *cache.Cache

func listReleasesOneRepo(owner, repo string) (releases []github.RepositoryRelease, resp *github.Response, err error) {
	cached, found := requestCache.Get(filepath.Join(owner, repo))
	if found {
		releases = cached.([]github.RepositoryRelease)
		return
	}

	releases, resp, err = client.Repositories.ListReleases(owner, repo, nil)
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

	repos, resp, err = client.Repositories.List(owner, nil)
	if err != nil {
		return
	}

	requestCache.Add(owner, repos, cache.DefaultExpiration)
	return
}

func listReleases(owner, repo string) (releases []github.RepositoryRelease, resp *github.Response, err error) {
	if repo == "" {
		repos, resp, err := listProjects(owner)
		if err != nil {
			return nil, resp, err
		}

		for _, repo := range repos {
			repoReleases, _, err := listReleasesOneRepo(owner, *repo.Name)
			if err != nil {
				continue
			}
			releases = append(releases, repoReleases...)
		}
	} else {
		releases, resp, err = listReleasesOneRepo(owner, repo)
	}
	return
}
