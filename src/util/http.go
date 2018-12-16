package util

import (
	"bufio"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// LogHandler provides middleware that logs all requests and response codes
// using logrus.
func LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rwi := &rwInterceptor{ResponseWriter: w}
		next.ServeHTTP(rwi, r)
		code := rwi.statusCode

		if code >= 500 {
			log.Errorf("%s %s -> %d", r.Method, r.URL.Path, rwi.statusCode)
		} else if code >= 400 {
			log.Warnf("%s %s -> %d", r.Method, r.URL.Path, rwi.statusCode)
		} else {
			log.Debugf("%s %s -> %d", r.Method, r.URL.Path, rwi.statusCode)
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
