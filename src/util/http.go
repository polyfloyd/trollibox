package util

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
)

func LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rwi := &rwInterceptor{ResponseWriter: w}
		next.ServeHTTP(rwi, r)
		code := rwi.statusCode

		if code >= 500 {
			slog.Error("Request handled", "method", r.Method, "path", r.URL.Path, "status", rwi.statusCode)
		} else if code >= 400 {
			slog.Warn("Request handled", "method", r.Method, "path", r.URL.Path, "status", rwi.statusCode)
		} else {
			slog.Debug("Request handled", "method", r.Method, "path", r.URL.Path, "status", rwi.statusCode)
		}
	})
}

type rwInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rwi *rwInterceptor) WriteHeader(code int) {
	rwi.statusCode = code
	rwi.ResponseWriter.WriteHeader(code)
}

func (rwi *rwInterceptor) Write(b []byte) (int, error) {
	if rwi.statusCode == 0 {
		rwi.WriteHeader(http.StatusOK)
	}
	return rwi.ResponseWriter.Write(b)
}

func (rwi *rwInterceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rwi.ResponseWriter.(http.Hijacker).Hijack()
}

func (rwi *rwInterceptor) Flush() {
	rwi.ResponseWriter.(http.Flusher).Flush()
}
