package deb

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blakesmith/ar"
	"github.com/ulikunitz/xz"

	"github.com/ayufan/debian-repository/internal/multi_hash"
	"github.com/ayufan/debian-repository/internal/repository_cache"
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

func readControlTarBzip2(r io.Reader) ([]byte, error) {
	bzip2 := bzip2.NewReader(r)

	return readControlTar(bzip2)
}

type Archive struct {
	Control []byte
}

func (d *Archive) parseArchive(r io.Reader) error {
	m := multi_hash.New()
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
		} else if name == "control.tar" || name == "control.tar/" {
			d.Control, err = readControlTar(r)
		} else if name == "control.tar.gz" || name == "control.tar.gz/" {
			d.Control, err = readControlTarGz(r)
		} else if name == "control.tar.xz" || name == "control.tar.xz/" {
			d.Control, err = readControlTarXz(r)
		} else if name == "control.tar.bz2" || name == "control.tar.bz2/" {
			d.Control, err = readControlTarBzip2(r)
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
		m.WritePackageHashes(buffer)
		d.Control = buffer.Bytes()
	}
	return err
}

func (d *Archive) readFromCache(tag string) error {
	data, err := repository_cache.Read(tag, "control")
	if err != nil {
		return err
	}

	d.Control = data
	return nil
}

func (d *Archive) writeToCache(tag string) error {
	return repository_cache.Write(tag, "control", d.Control)
}

func Read(r io.Reader) (*Archive, error) {
	deb := &Archive{}
	err := deb.parseArchive(r)
	if err != nil {
		return nil, err
	}

	return deb, nil
}

func ReadFromCache(md5sum string) *Archive {
	deb := &Archive{}
	if deb.readFromCache(md5sum) != nil {
		return nil
	}

	return deb
}

func ReadFromFile(fileName string) (*Archive, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Read(file)
}

func ReadFromURL(url, cacheKey string) (deb *Archive, err error) {
	if deb := ReadFromCache(cacheKey); deb != nil {
		return deb, nil
	}

	started := time.Now()
	defer func() {
		log.Println("Readed", url, "in", time.Since(started), err)
	}()

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get: %q", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http status code: %d %s", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	deb, err = Read(resp.Body)
	if err != nil {
		return nil, err
	}

	deb.writeToCache(cacheKey)
	return deb, nil
}
