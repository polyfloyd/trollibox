package util

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

// DetermineFullURLRoot attempts to guess the canonical path at which the root
// of a webservice can be found.
func DetermineFullURLRoot(root, address string) (string, error) {
	// Handle "http://host:port/"
	if regexp.MustCompile("^https?:\\/\\/").MatchString(root) {
		return root, nil
	}
	// Handle "//host:port/"
	if regexp.MustCompile("^\\/\\/.").MatchString(root) {
		// Assume plain HTTP. If you are smart enough to set up HTTPS you are
		// also smart enough to configure the URLRoot.
		return "http:" + root, nil
	}
	// Handle "/"
	if root == "/" {
		i := strings.LastIndex(address, ":")
		host, port := address[:i], address[i+1:]
		if host == "" || host == "0.0.0.0" {
			host = "127.0.0.1"
		} else if host == "[::]" {
			host = "[::1]"
		}
		return fmt.Sprintf("http://%s:%s/", host, port), nil
	}
	// Give up
	return "", fmt.Errorf("Unsupported URL Root format: %q", root)
}

// TempName returns a path which may be used to create a temporary file at.
func TempName(prefix string) string {
	for {
		var buf [32]byte
		if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
			panic(err)
		}
		file := path.Join(os.TempDir(), fmt.Sprintf("%s-%x", prefix, buf))
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return file
		}
	}
}

// ErrorAsChannel creates a channel from which the provided error can be
// immediately received.
func ErrorAsChannel(err error) <-chan error {
	errs := make(chan error, 1)
	errs <- err
	close(errs)
	return errs
}
