package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"time"

	"sort"

	"github.com/google/go-github/github"
)

type debPackageSlice []*debPackage

func (a debPackageSlice) Len() int {
	return len(a)
}

func (a debPackageSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a debPackageSlice) Less(i, j int) bool {
	if a[i].name() < a[j].name() {
		return true
	} else if a[i].name() > a[j].name() {
		return false
	}
	if a[i].version() < a[j].version() {
		return true
	} else if a[i].version() > a[j].version() {
		return false
	}
	if a[i].architecture() < a[j].architecture() {
		return true
	} else if a[i].architecture() > a[j].architecture() {
		return false
	}
	return false
}

type packageRepository struct {
	debs   debPackageSlice
	loaded map[debKey]struct{}
}

func (p *packageRepository) add(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
	deb, err := packages.get(release, asset)
	if err != nil {
		return err
	}

	// don't add the same version, again
	if _, ok := p.loaded[deb.key()]; ok {
		log.Println("ignore", deb.key())
		return nil
	}

	if p.loaded == nil {
		p.loaded = make(map[debKey]struct{})
	}
	p.loaded[deb.key()] = struct{}{}
	p.debs = append(p.debs, deb)
	return nil
}

func (p *packageRepository) sort() {
	fmt.Println("sort me")
	sort.Sort(p.debs)
	for _, xx := range p.debs {
		fmt.Println("XX:", xx.name(), xx.architecture(), xx.version())
	}
}

func (p *packageRepository) write(w io.Writer) {
	for _, deb := range p.debs {
		deb.write(w)
	}
}

func (p *packageRepository) writeGz(w io.Writer) {
	gz := gzip.NewWriter(w)
	defer gz.Close()

	p.write(gz)
}

func (p *packageRepository) newestUpdatedAt() (result time.Time) {
	for _, deb := range p.debs {
		updatedAt := deb.updatedAt()
		if result.Sub(updatedAt) < 0 {
			result = updatedAt
		}
	}
	return
}

func (p *packageRepository) writeRelease(w io.Writer) {
	packagesHash := newMultiHash()
	packagesGzHash := newMultiHash()

	packagesGz := gzip.NewWriter(packagesGzHash)
	defer packagesGz.Close()

	p.write(io.MultiWriter(packagesHash, packagesGz))
	packagesGz.Close()

	fmt.Fprintln(w, "Date:", p.newestUpdatedAt().Format(time.RFC1123))
	for _, name := range supportedHashes {
		fmt.Fprint(w, name, ":\n")
		packagesHash.print(w, name, "Packages")
		packagesGzHash.print(w, name, "Packages.gz")
	}
}
