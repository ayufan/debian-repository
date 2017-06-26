package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

var repositoryCache string

func init() {
	repositoryCache = os.Getenv("REPOSITORY_CACHE")
	if repositoryCache == "" {
		repositoryCache = "tmp-cache"
	}
}

func readFromCache(tag, name string) ([]byte, error) {
	cachePath := filepath.Join(repositoryCache, tag+"."+name)
	return ioutil.ReadFile(cachePath)
}

func writeToCache(tag, name string, content []byte) error {
	cachePath := filepath.Join(repositoryCache, tag+"."+name)

	// create temp file
	f, err := ioutil.TempFile(repositoryCache, name)
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	// write file to cache
	_, err = f.Write(content)
	if err != nil {
		return err
	}

	// remove old file and do in-place replace
	f.Close()
	os.Remove(cachePath)
	return os.Rename(f.Name(), cachePath)
}
