package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
)

//go:embed build/release/*
var release embed.FS

func Files(build string) fs.FS {
	if build == "release" {
		sub, err := fs.Sub(release, "build/release")
		if err != nil {
			panic(err)
		}
		return sub
	}
	if build == "debug" {
		return os.DirFS("src/handler/webui/build/dev")
	}
	panic(fmt.Errorf("invalid build: %q", build))
}
