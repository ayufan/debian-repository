package main

import (
	"io/ioutil"
	"archive/zip"
	"fmt"
)

func readFile(zip *zip.Reader, name string) ([]byte, error) {
	for _, zipFile := range zip.File {
		if zipFile.Name == name {
			file, err := zipFile.Open()
			if err != nil {
				return nil, err
			}
			defer file.Close()

			return ioutil.ReadAll(file)
		}
	}

	return nil, fmt.Errorf("file not found: %v", name)
}
