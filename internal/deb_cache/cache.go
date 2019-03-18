package deb_cache

import (
	"errors"
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/google/go-github/github"

	"github.com/ayufan/debian-repository/internal/deb"
)

type Cache struct {
	cache *lru.Cache
	lock  sync.Mutex
}

func (d *Cache) find(id int) *deb.Package {
	d.lock.Lock()
	defer d.lock.Unlock()

	debPackage, found := d.cache.Get(id)
	if !found {
		debPackage = &deb.Package{}
		d.cache.Add(id, debPackage)
	}

	return debPackage.(*deb.Package)
}

func (d *Cache) Get(release *github.RepositoryRelease, asset *github.ReleaseAsset) (*deb.Package, error) {
	if asset == nil || asset.ID == nil {
		return nil, errors.New("asset is null")
	}

	deb := d.find(*asset.ID)
	return deb, deb.Ensure(release, asset)
}

func (d *Cache) Clear() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.cache.Clear()
}

func New(itemCount int) *Cache {
	return &Cache{
		cache: lru.New(itemCount),
	}
}
