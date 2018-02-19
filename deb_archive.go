package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/blakesmith/ar"
	"github.com/ulikunitz/xz"
)

func enumerateDebArchive(r io.Reader, fn func(name string, r io.Reader) error) error {
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

func readDebianBinary(r io.Reader) (string, error) {
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

func readControlTar(r io.Reader) ([]byte, error) {
	rd := tar.NewReader(r)
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

func readControlTarGz(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return readControlTar(gz)
}

func readControlTarXz(r io.Reader) ([]byte, error) {
	xz, err := xz.NewReader(r)
	if err != nil {
		return nil, err
	}

	return readControlTar(xz)
}

type debArchive struct {
	Control []byte
}

func (d *debArchive) parseArchive(r io.Reader) error {
	m := newMultiHash()
	pr, pw := io.Pipe()
	defer pr.Close()

	go func() {
		defer pw.Close()
		io.Copy(io.MultiWriter(pw, m), r)
	}()

	var debianVersion string

	err := enumerateDebArchive(pr, func(name string, r io.Reader) (err error) {
		if name == "debian-binary" || name == "debian-binary/" {
			debianVersion, err = readDebianBinary(r)
		} else if name == "control.tar.gz" || name == "control.tar.gz/" {
			d.Control, err = readControlTarGz(r)
		} else if name == "control.tar.xz" || name == "control.tar.xz/" {
			d.Control, err = readControlTarXz(r)
		}
		return
	})

	if err == nil {
		if debianVersion == "" || d.Control == nil {
			err = errors.New("missing debian-binary or control.tar.gz/xz")
		}
	}
	if err == nil {
		buffer := bytes.NewBuffer(d.Control)
		m.writePackageHashes(buffer)
		d.Control = buffer.Bytes()
	}
	return err
}

func (d *debArchive) readFromCache(tag string) error {
	data, err := readFromCache(tag, "control")
	if err != nil {
		return err
	}

	d.Control = data
	return nil
}

func (d *debArchive) writeToCache(tag string) error {
	return writeToCache(tag, "control", d.Control)
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

	err = deb.parseArchive(resp.Body)
	if err != nil {
		return nil, err
	}

	deb.writeToCache(md5sum)
	return
}
