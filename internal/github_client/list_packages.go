package github_client

import (
	"strings"

	"github.com/google/go-github/github"
)

type Package struct {
	Release *github.RepositoryRelease
	Asset   *github.ReleaseAsset
}

func (a *API) ListPackages(owner, repo string) ([]Package, error) {
	releases, _, err := a.ListReleases(owner, repo)
	if err != nil {
		return nil, err
	}

	var packages []Package

	for _, release := range releases {
		if release.Draft != nil && *release.Draft {
			continue
		}

		for _, asset := range release.Assets {
			if !strings.HasSuffix(*asset.Name, ".deb") {
				continue
			}

			asset2 := asset

			packages = append(packages, Package{release, &asset2})
		}
	}

	return packages, nil
}
