package deb

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/ayufan/debian-repository/internal/multi_hash"
)

type Repository struct {
	debs             PackageSlice
	loaded           map[Key]struct{}
	owner, repo      string
	organizationWide bool
}

func (p *Repository) Add(debPackage *Package) error {
	// don't add the same version, again
	if _, ok := p.loaded[debPackage.Key()]; ok {
		log.Println("ignore", debPackage.Key())
		return nil
	}

	if p.loaded == nil {
		p.loaded = make(map[Key]struct{})
	}
	p.loaded[debPackage.Key()] = struct{}{}
	p.debs = append(p.debs, debPackage)
	return nil
}

func (p *Repository) Sort() {
	sort.Sort(p.debs)
}

func (p *Repository) Write(w io.Writer) {
	for _, deb := range p.debs {
		deb.Write(w, p.organizationWide)
	}
}

func (p *Repository) newestUpdatedAt() (result time.Time) {
	for _, deb := range p.debs {
		if result.Sub(deb.UpdatedAt) < 0 {
			result = deb.UpdatedAt
		}
	}
	return
}

func (p *Repository) getOrigin() string {
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

func (p *Repository) getDescription() string {
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

func (p *Repository) WriteRelease(w io.Writer) {
	packagesHash := multi_hash.New()
	packagesGzHash := multi_hash.New()

	packagesGz := gzip.NewWriter(packagesGzHash)
	defer packagesGz.Close()

	p.Write(io.MultiWriter(packagesHash, packagesGz))
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

func NewRepository(owner, repo string) *Repository {
	return &Repository{
		owner:            owner,
		repo:             repo,
		organizationWide: repo == "",
	}
}
