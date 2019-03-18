package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ayufan/debian-repository/internal/deb"
	"github.com/ayufan/debian-repository/internal/deb_cache"
	"github.com/ayufan/debian-repository/internal/github_client"
)

var allowedOwners []string
var githubAPI *github_client.API
var packagesCache *deb_cache.Cache

func isOwnerAllowed(owner string) bool {
	for _, allowedOwner := range allowedOwners {
		if allowedOwner == owner {
			return true
		}
	}
	return false
}

func enumeratePackages(w http.ResponseWriter, r *http.Request, fn func(ghPackage github_client.Package) error) error {
	vars := mux.Vars(r)

	if !isOwnerAllowed(vars["owner"]) {
		return fmt.Errorf("%q is not allowed. Please add it to ALLOWED_ORGS", vars["owner"])
	}

	packages, err := githubAPI.ListPackages(vars["owner"], vars["repo"])
	if err != nil {
		return err
	}

	ch := make(chan github_client.Package)

	// load at most 4 files at single time
	for i := 0; i < 4; i++ {
		go func() {
			for ghPackage := range ch {
				packagesCache.Get(ghPackage)
			}
		}()
	}

	// trigger loading of all packages
	for _, ghPackage := range packages {
		ch <- ghPackage
	}

	// do actual iteration of packages
	for _, ghPackage := range packages {
		err := fn(ghPackage)
		if err != nil {
			return err
		}
	}

	return nil
}

func getRepository(w http.ResponseWriter, r *http.Request) (*deb.Repository, error) {
	vars := mux.Vars(r)

	repository := deb.NewRepository(vars["owner"], vars["repo"],
		vars["suite"], vars["component"],
		signingKey)

	err := enumeratePackages(w, r, func(ghPackage github_client.Package) error {
		deb, err := packagesCache.Get(ghPackage)
		if err == nil {
			repository.Add(deb)
		}
		return nil
	})

	repository.Sort()

	return repository, err
}
