package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/blakesmith/ar"
)

func enumeratedebArchive(r io.Reader, fn func(name string, r io.Reader) error) error {
	rd := ar.NewReader(r)

	for {
		header, err := rd.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = fn(header.Name, rd)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func parseDebianBinary(r io.Reader) (string, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	version := string(data)
	version = strings.TrimSpace(version)
	if version != "2.0" {
		return "", fmt.Errorf("only debian version 2.0 is supported, where received: %s", version)
	}
	return version, nil
}

func parseControlTarGz(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	rd := tar.NewReader(gz)
	for {
		header, err := rd.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Name == "control" || header.Name == "./control" {
			return ioutil.ReadAll(rd)
		}
	}
	return nil, errors.New("control not found in control.tar.gz")
}

type debArchive struct {
	Version string
	Control []byte
	Hashes  map[string]string
}

func (d *debArchive) readArchive(r io.Reader) error {
	m := newMultiHash()
	pr, pw := io.Pipe()
	defer pr.Close()

	go func() {
		defer pw.Close()
		io.Copy(io.MultiWriter(pw, m), r)
	}()

	err := enumeratedebArchive(pr, func(name string, r io.Reader) (err error) {
		if name == "debian-binary" || name == "debian-binary/" {
			d.Version, err = parseDebianBinary(r)
		} else if name == "control.tar.gz" || name == "control.tar.gz/" {
			d.Control, err = parseControlTarGz(r)
		}
		return
	})

	if err == nil {
		if d.Version == "" || d.Control == nil {
			err = errors.New("missing debian-binary or control.tar.gz")
		}
	}
	if err == nil {
		d.Hashes = m.packagesHashes()
	}
	return err
}

func (d *debArchive) readFromCache(tag string) error {
	data, err := readFromCache(tag, "json")
	if err != nil {
		return err
	}

	return json.Unmarshal(data, d)
}

func (d *debArchive) writeToCache(tag string) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	return writeToCache(tag, "json", data)
}

func readDebArchive(url string) (deb *debArchive, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("http status code: %d %s", resp.StatusCode, resp.Status)
		return
	}
	defer resp.Body.Close()

	md5sum := resp.Header.Get("Etag")
	md5sum = strings.Trim(md5sum, `W/"`)
	if md5sum == "" {
		err = fmt.Errorf("missing md5sum")
		return
	}

	deb = &debArchive{}
	if deb.readFromCache(md5sum) == nil {
		return
	}

	deb = &debArchive{}
	err = deb.readArchive(resp.Body)
	if err != nil {
		return nil, err
	}

	deb.writeToCache(md5sum)
	return
}
