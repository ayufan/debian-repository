package github_client

import (
	"fmt"
	"strings"

	"github.com/google/go-github/github"
)

type Package struct {
	Release *github.RepositoryRelease
	Asset   *github.ReleaseAsset
}

func (a *API) ListPackages(owner, repo, distribution string) ([]Package, error) {
	releases, _, err := a.ListReleases(owner, repo)
	if err != nil {
		return nil, err
	}

	var packages []Package

	for _, release := range releases {
		if release.Draft != nil && *release.Draft {
			continue
		}

		switch distribution {
		case "releases":
			if release.Prerelease != nil && *release.Prerelease {
				continue
			}
		case "pre-releases":
		default:
			return nil, fmt.Errorf("%q is unknown distribution", distribution)
		}

		for _, asset := range release.Assets {
			if !strings.HasSuffix(*asset.Name, ".deb") {
				continue
			}

			packages = append(packages, Package{&release, &asset})
		}
	}

	return packages, nil
}
