package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
)

//go:embed *
var files embed.FS

func Files(build string) fs.FS {
	if build == "release" {
		return files
	}
	if build == "debug" {
		return os.DirFS("src/handler/webui")
	}
	panic(fmt.Errorf("invalid build: %q", build))
}
