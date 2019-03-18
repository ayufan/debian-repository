package http_helpers

import (
	"compress/gzip"
	"io"
	"net/http"
)

func WriteGz(w http.ResponseWriter, body func(io.Writer) error) error {
	w.Header().Set("Content-Type", "binary/octet-stream")

	gz := gzip.NewWriter(w)
	defer gz.Close()

	return body(gz)
}
