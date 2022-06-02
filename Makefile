SHELL=/bin/bash
VERSION=$(shell git describe --always --dirty --tags)
VERSION_DATE=$(shell date --date="@$$(git show -s --format='%ct' HEAD)" '+%F')

all: bin/trollibox

# Use with -j2
.PHONY: dev
dev: frontend-watch backend-watch

bin/trollibox: frontend-release
	go build -ldflags "-X main.build=release -X main.version=${VERSION} -X main.versionDate=${VERSION_DATE}" -o $@ ./src

src/handler/webui/node_modules: src/handler/webui/package.json src/handler/webui/package-lock.json
	cd src/handler/webui && npm ci

.PHONY: frontend-release frontend-watch
frontend-release: src/handler/webui/node_modules $(find src/handler/webui -not -path '*/build/*')
	cd src/handler/webui && npm run build

frontend-watch: src/handler/webui/node_modules
	cd src/handler/webui && npm run watch

.PHONY: backend-watch
backend-watch:
	find -name '*.go' | entr -rn \
		go run -ldflags "-X main.build=debug -X main.version=${VERSION} -X main.versionDate=${VERSION_DATE}" ./src
