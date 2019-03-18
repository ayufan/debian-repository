package deb

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/ayufan/debian-repository/internal/deb_key"
	"github.com/ayufan/debian-repository/internal/helpers"
	"github.com/ayufan/debian-repository/internal/multi_hash"
)

type Repository struct {
	debs             PackageSlice
	loaded           map[Key]struct{}
	owner, repo      string
	suite, component string
	organizationWide bool
	signingKey       *deb_key.Key
}

type RepositoryFile struct {
	Writer func(io.Writer) error

	hash *multi_hash.MultiHash
}

func (p *Repository) Architectures() map[string]struct{} {
	archs := make(map[string]struct{})
	for _, deb := range p.debs {
		archs[deb.Architecture()] = struct{}{}
	}
	return archs
}

func (p *Repository) Add(debPackage *Package) error {
	if !debPackage.MatchingSuite(p.suite) {
		return nil
	}

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

func (p *Repository) WritePackages(w io.Writer, component, architecture string) error {
	for _, deb := range p.debs {
		if !deb.MatchingArchitecture(architecture) {
			continue
		}
		if !deb.MatchingComponents(component) {
			continue
		}

		deb.Write(w, p.organizationWide)
	}
	return nil
}

func (p *Repository) Files() map[string]*RepositoryFile {
	files := make(map[string]*RepositoryFile)

	if p.suite != "" {
		for arch := range p.Architectures() {
			if arch == "" {
				continue
			}

			arch_ := arch

			files["releases/binary-"+arch+"/Packages"] = &RepositoryFile{
				Writer: func(w io.Writer) error {
					return p.WritePackages(w, "releases", arch_)
				},
			}
			files["pre-releases/binary-"+arch+"/Packages"] = &RepositoryFile{
				Writer: func(w io.Writer) error {
					return p.WritePackages(w, "pre-releases", arch_)
				},
			}
		}
	} else {
		files["Packages"] = &RepositoryFile{
			Writer: func(w io.Writer) error {
				return p.WritePackages(w, p.component, "")
			},
		}
	}

	// compress all files
	for fileName, fileOpt := range files {
		if strings.HasSuffix(fileName, ".gz") {
			continue
		}

		files[fileName+".gz"] = &RepositoryFile{
			Writer: helpers.GzWriter(fileOpt.Writer),
		}
	}

	return files
}

func (p *Repository) AllFiles() map[string]*RepositoryFile {
	files := p.Files()
	files["Release"] = &RepositoryFile{
		Writer: p.WriteRelease,
	}
	files["Release.gpg"] = &RepositoryFile{
		Writer: func(w io.Writer) error {
			return p.signingKey.EncodeWithArmor(w, p.WriteRelease)
		},
	}
	files["InRelease"] = &RepositoryFile{
		Writer: func(w io.Writer) error {
			return p.signingKey.Encode(w, p.WriteRelease)
		},
	}
	return files
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

func (p *Repository) WriteRelease(w io.Writer) error {
	files := p.Files()

	for _, fileOpt := range files {
		hash, err := multi_hash.HashMe(fileOpt.Writer)
		if err != nil {
			return err
		}
		fileOpt.hash = hash
	}

	fmt.Fprintln(w, "Origin:", p.getOrigin())
	fmt.Fprintln(w, "Description:", p.getDescription())
	fmt.Fprintln(w, "Date:", p.newestUpdatedAt().Format(time.RFC1123))
	if p.suite != "" {
		fmt.Fprintln(w, "Codename:", p.suite)
		fmt.Fprintln(w, "Components:", "releases pre-releases")
	}
	for _, hashOpt := range multi_hash.Hashes {
		fmt.Fprint(w, hashOpt.Name, ":\n")
		for fileName, fileOpt := range files {
			fileOpt.hash.WriteReleaseHash(w, hashOpt.Name, fileName)
		}
	}

	return nil
}

func NewRepository(owner, repo, suite, component string, signingKey *deb_key.Key) *Repository {
	return &Repository{
		owner:            owner,
		repo:             repo,
		suite:            suite,
		component:        component,
		organizationWide: repo == "",
		signingKey:       signingKey,
	}
}
