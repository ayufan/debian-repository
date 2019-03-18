package helpers

import (
	"compress/gzip"
	"io"
)

func GzWriter(body func(io.Writer) error) func(io.Writer) error {
	return func(w io.Writer) error {
		gz := gzip.NewWriter(w)
		defer gz.Close()

		return body(gz)
	}
}
