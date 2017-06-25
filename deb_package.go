package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"github.com/stapelberg/godebiancontrol"
)

type debKey struct {
	name         string
	version      string
	architecture string
}

type debPackage struct {
	release    *github.RepositoryRelease
	asset      *github.ReleaseAsset
	paragraphs godebiancontrol.Paragraph

	control string
	md5sum  string

	loadOnce   sync.Once
	loadStatus error
}

func (p *debPackage) key() debKey {
	return debKey{
		name:         p.name(),
		version:      p.version(),
		architecture: p.architecture(),
	}
}

func (p *debPackage) name() string {
	if p.paragraphs == nil {
		return ""
	}
	return p.paragraphs["Package"]
}

func (p *debPackage) architecture() string {
	if p.paragraphs == nil {
		return ""
	}
	return p.paragraphs["Architecture"]
}

func (p *debPackage) version() string {
	if p.paragraphs == nil {
		return ""
	}
	return p.paragraphs["Version"]
}

func (p *debPackage) updatedAt() time.Time {
	return p.asset.UpdatedAt.Time
}

func (p *debPackage) load(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
	control, etag, err := readDebianArchive(*asset.BrowserDownloadURL)
	if err != nil {
		return err
	}

	paragraphs, err := godebiancontrol.Parse(bytes.NewBuffer(control))
	if err != nil {
		return err
	}

	if len(paragraphs) == 0 {
		return errors.New("no paragraphs")
	}

	if len(paragraphs) > 1 {
		return errors.New("too many paragraphs")
	}

	p.control = string(control)
	p.release = release
	p.asset = asset
	p.paragraphs = paragraphs[0]
	p.md5sum = etag
	p.md5sum = strings.Trim(p.md5sum, `W/"`)

	// Validate package
	if p.name() == "" {
		return errors.New("missing Package from control")
	}
	if p.architecture() == "" {
		return errors.New("missing Architecture from control")
	}
	if p.version() == "" {
		return errors.New("missing Version from control")
	}
	if p.md5sum == "" {
		return errors.New("missing md5sum")
	}
	return nil
}

func (p *debPackage) ensure(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
	p.loadOnce.Do(func() {
		p.loadStatus = p.load(release, asset)
	})
	return p.loadStatus
}

func (p *debPackage) write(w io.Writer) {
	fmt.Fprint(w, p.control)
	fmt.Fprint(w, "Filename: ", "./download/", *p.release.TagName, "/", *p.asset.Name, "\n")
	fmt.Fprint(w, "Size: ", *p.asset.Size, "\n")
	fmt.Fprint(w, "MD5sum: ", p.md5sum, "\n")
	fmt.Fprint(w, "\n")
}
