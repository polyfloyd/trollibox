SHELL=/bin/bash
VERSION=$(shell git describe --always --dirty --tags)
VERSION_DATE=$(shell date --date="@$$(git show -s --format='%ct' HEAD)" '+%F')

all: bin/trollibox

# Use with -j2
.PHONY: dev
dev: frontend-watch backend-watch

test: frontend-test backend-test

bin/trollibox: frontend-release
	go build -ldflags "-X main.build=release -X main.version=${VERSION} -X main.versionDate=${VERSION_DATE}" -o $@ ./src

src/handler/webui/node_modules: src/handler/webui/package.json src/handler/webui/package-lock.json
	cd src/handler/webui && npm ci

.PHONY: frontend-release frontend-watch
frontend-release: src/handler/webui/node_modules $(find src/handler/webui -not -path '*/build/*')
	cd src/handler/webui && npm run build

frontend-watch: src/handler/webui/node_modules
	cd src/handler/webui && npm run watch

frontend-test: src/handler/webui/node_modules
	cd src/handler/webui && npm run test

.PHONY: backend-watch
backend-watch:
	(find -name '*.go'; echo config.yaml) | entr -rn \
		go run -ldflags "-X main.build=debug -X main.version=${VERSION} -X main.versionDate=${VERSION_DATE}" ./src

backend-test:
	go test -race -cover ./src/...
