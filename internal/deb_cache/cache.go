package deb_cache

import (
	"errors"
	"sync"

	"github.com/golang/groupcache/lru"

	"github.com/ayufan/debian-repository/internal/deb"
	"github.com/ayufan/debian-repository/internal/github_client"
)

type Cache struct {
	cache *lru.Cache
	lock  sync.Mutex
}

func (d *Cache) find(id int64) *deb.Package {
	d.lock.Lock()
	defer d.lock.Unlock()

	debPackage, found := d.cache.Get(id)
	if !found {
		debPackage = &deb.Package{}
		d.cache.Add(id, debPackage)
	}

	return debPackage.(*deb.Package)
}

func (d *Cache) Get(ghPackage github_client.Package) (*deb.Package, error) {
	if ghPackage.Asset == nil || ghPackage.Asset.ID == nil {
		return nil, errors.New("asset is null")
	}

	deb := d.find(*ghPackage.Asset.ID)
	return deb, deb.Ensure(ghPackage.Release, ghPackage.Asset)
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
