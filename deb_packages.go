package main

import (
	"errors"
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/google/go-github/github"

	"github.com/ayufan/debian-repository/internal/deb"
)

type debPackages struct {
	cache *lru.Cache
	lock  sync.Mutex
}

var packages *debPackages

func (d *debPackages) find(id int) *deb.Package {
	d.lock.Lock()
	defer d.lock.Unlock()

	debPackage, found := d.cache.Get(id)
	if !found {
		debPackage = &deb.Package{}
		d.cache.Add(id, debPackage)
	}

	return debPackage.(*deb.Package)
}

func (d *debPackages) get(release *github.RepositoryRelease, asset *github.ReleaseAsset) (*deb.Package, error) {
	if asset == nil || asset.ID == nil {
		return nil, errors.New("asset is null")
	}

	deb := d.find(*asset.ID)
	return deb, deb.Ensure(release, asset)
}

func (d *debPackages) clear() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.cache.Clear()
}
