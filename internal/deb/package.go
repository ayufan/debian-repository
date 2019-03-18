package deb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"strings"

	"path/filepath"

	"github.com/google/go-github/github"
	"github.com/stapelberg/godebiancontrol"
)

var Suites = []string{"bionic", "xenial"}
var Architectures = []string{"arm64", "armhf", "amd64"}

type Key struct {
	Name         string
	Version      string
	Architecture string
}

type Package struct {
	*Archive

	paragraphs godebiancontrol.Paragraph

	RepoName    string
	TagName     string
	Suite       string
	Component   string
	FileName    string
	DownloadURL string
	FileSize    int
	UpdatedAt   time.Time

	loadOnce   sync.Once
	loadStatus error
}

func (p *Package) Key() Key {
	return Key{
		Name:         p.Name(),
		Version:      p.Version(),
		Architecture: p.Architecture(),
	}
}

func (p *Package) Name() string {
	if p.paragraphs == nil {
		return ""
	}
	return p.paragraphs["Package"]
}

func (p *Package) Architecture() string {
	if p.paragraphs == nil {
		return ""
	}
	return p.paragraphs["Architecture"]
}

func (p *Package) Version() string {
	if p.paragraphs == nil {
		return ""
	}
	return p.paragraphs["Version"]
}

func (p *Package) MatchingSuite(suite string) bool {
	if suite != "" {
		return p.Suite == "all" || p.Suite == suite
	}

	return p.Suite == "all"
}

func (p *Package) MatchingArchitecture(architecture string) bool {
	if architecture != "" {
		return architecture == "all" || p.Architecture() == architecture
	}

	return true
}

func (p *Package) MatchingComponents(component string) bool {
	return p.Component == component
}

func (p *Package) Load(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
	archive, err := ReadFromURL(*asset.BrowserDownloadURL, "cache-asset-"+strconv.Itoa(*asset.ID))
	if err != nil {
		return err
	}

	paragraphs, err := godebiancontrol.Parse(bytes.NewBuffer(archive.Control))
	if err != nil {
		return err
	}

	if len(paragraphs) == 0 {
		return errors.New("no paragraphs")
	}

	if len(paragraphs) > 1 {
		return errors.New("too many paragraphs")
	}

	downloadURL := strings.Split(*asset.BrowserDownloadURL, "/")

	p.Archive = archive
	p.RepoName = downloadURL[4]
	p.TagName = downloadURL[7]
	p.FileName = downloadURL[8]
	p.DownloadURL = *asset.BrowserDownloadURL
	p.FileSize = *asset.Size
	p.UpdatedAt = asset.UpdatedAt.Time
	p.Component = "releases"
	if release.Prerelease != nil && *release.Prerelease {
		p.Component = "pre-releases"
	}
	p.paragraphs = paragraphs[0]

	for _, suite := range Suites {
		if strings.Contains(p.Version(), suite) || strings.Contains(p.FileName, suite) {
			p.Suite = suite
			break
		}
	}

	if p.Suite == "" {
		p.Suite = "all"
	}

	// Validate package
	if p.Name() == "" {
		return errors.New("missing Package from control")
	}
	if p.Architecture() == "" {
		return errors.New("missing Architecture from control")
	}
	if p.Version() == "" {
		return errors.New("missing Version from control")
	}
	return nil
}

func (p *Package) scheduleRestart() {
	if p.loadStatus == nil {
		return
	}

	if strings.Contains(p.loadStatus.Error(), "http") {
		time.AfterFunc(30*time.Second, func() {
			p.loadOnce = sync.Once{}
		})
	}
}

func (p *Package) Ensure(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
	p.loadOnce.Do(func() {
		p.loadStatus = p.Load(release, asset)
		p.scheduleRestart()
	})
	return p.loadStatus
}

func (p *Package) Write(w io.Writer, organizationWide bool) {
	w.Write(p.Control)
	if organizationWide {
		fmt.Fprintln(w, "Filename:", filepath.Join("pool", p.RepoName, p.TagName, p.FileName))
	} else {
		fmt.Fprintln(w, "Filename:", filepath.Join("pool", p.TagName, p.FileName))
	}
	fmt.Fprintln(w, "Size:", p.FileSize)
	fmt.Fprintln(w)
}
