package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/github"

	"github.com/ayufan/debian-repository/internal/multi_hash"
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
	debs             debPackageSlice
	loaded           map[debKey]struct{}
	owner, repo      string
	organizationWide bool
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
	sort.Sort(p.debs)
}

func (p *packageRepository) write(w io.Writer) {
	for _, deb := range p.debs {
		deb.write(w, p.organizationWide)
	}
}

func (p *packageRepository) writeGz(w io.Writer) {
	gz := gzip.NewWriter(w)
	defer gz.Close()

	p.write(gz)
}

func (p *packageRepository) newestUpdatedAt() (result time.Time) {
	for _, deb := range p.debs {
		if result.Sub(deb.updatedAt) < 0 {
			result = deb.updatedAt
		}
	}
	return
}

func (p *packageRepository) getOrigin() string {
	components := []string{
		"GITHUB", "AYUFAN", "DEB",
	}
	if p.owner != "" {
		components = append(components, p.owner)
	}
	if p.repo != "" {
		components = append(components, p.repo)
	}
	return strings.Join(components, "-")
}

func (p *packageRepository) getDescription() string {
	components := []string{
		"https://github.com",
	}
	if p.owner != "" {
		components = append(components, p.owner)
	}
	if p.repo != "" {
		components = append(components, p.repo)
	}
	return strings.Join(components, "/")
}

func (p *packageRepository) writeRelease(w io.Writer) {
	packagesHash := multi_hash.New()
	packagesGzHash := multi_hash.New()

	packagesGz := gzip.NewWriter(packagesGzHash)
	defer packagesGz.Close()

	p.write(io.MultiWriter(packagesHash, packagesGz))
	packagesGz.Close()

	fmt.Fprintln(w, "Origin:", p.getOrigin())
	fmt.Fprintln(w, "Description:", p.getDescription())
	fmt.Fprintln(w, "Date:", p.newestUpdatedAt().Format(time.RFC1123))
	for _, hashOpt := range multi_hash.Hashes {
		fmt.Fprint(w, hashOpt.Name, ":\n")
		packagesHash.WriteReleaseHash(w, hashOpt.Name, "Packages")
		packagesGzHash.WriteReleaseHash(w, hashOpt.Name, "Packages.gz")
	}
}
