package util

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func Gzip(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Upgrade") != "" || !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			handler.ServeHTTP(res, req)
			return
		}

		res.Header().Set("Content-Encoding", "gzip")
		gzipper := gzip.NewWriter(res)
		defer gzipper.Close()
		handler.ServeHTTP(gzipResponseWriter{Writer: gzipper, ResponseWriter: res}, req)
	})
}
