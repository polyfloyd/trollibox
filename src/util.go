package main

import (
	"time"
)

var httpCacheSince = time.Now()

func HttpCacheTime() time.Time {
	if BUILD == "debug" {
		return time.Now()
	} else {
		return httpCacheSince
	}
}
