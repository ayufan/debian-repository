package main

import (
	"errors"
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/google/go-github/github"
)

type debPackages struct {
	cache *lru.Cache
	lock  sync.Mutex
}

var packages debPackages = debPackages{
	cache: lru.New(10000),
}

func (d *debPackages) find(id int) *debPackage {
	d.lock.Lock()
	defer d.lock.Unlock()

	deb, found := d.cache.Get(id)
	if !found {
		deb = &debPackage{}
		d.cache.Add(id, deb)
	}

	return deb.(*debPackage)
}

func (d *debPackages) get(release *github.RepositoryRelease, asset *github.ReleaseAsset) (*debPackage, error) {
	if asset == nil || asset.ID == nil {
		return nil, errors.New("asset is null")
	}

	deb := d.find(*asset.ID)
	return deb, deb.ensure(release, asset)
}

func (d *debPackages) clear() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.cache.Clear()
}
