package main

import (
	"sync"
	"errors"

	"github.com/google/go-github/github"
)

type debPackages struct {
	debs map[int]*debPackage
	lock sync.Mutex
}

var packages debPackages = debPackages{
	debs: make(map[int]*debPackage),
}

func (d *debPackages) find(id int) *debPackage {
	d.lock.Lock()
	defer d.lock.Unlock()

	deb := d.debs[id]
	if deb == nil {
		deb = &debPackage{}
		d.debs[id] = deb
	}
	return deb
}

func (d *debPackages) get(release *github.RepositoryRelease, asset *github.ReleaseAsset) (*debPackage, error) {
	if asset == nil || asset.ID == nil {
		return nil, errors.New("asset is null")
	}

	deb := d.find(*asset.ID)
	return deb, deb.ensure(release, asset)
}
