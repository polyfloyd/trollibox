package util

import (
	"fmt"
	"regexp"
	"strings"
)

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
