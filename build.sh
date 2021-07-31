#! /bin/bash

set -eu
cd `dirname $0`

NAME="trollibox"
WORKSPACE="$PWD"
BIN="$WORKSPACE/bin"
LIB="$WORKSPACE/lib"
ASSETS="$WORKSPACE/src/handler/webui"
GO_BINDATA="github.com/tmthrgd/go-bindata/go-bindata"
GO_MINIFY="github.com/tdewolff/minify/v2/cmd/minify"

mkdir -p "$BIN"
mkdir -p "$LIB"

echo "*** Building Project ***"
if [ ${RELEASE:-} ]; then
    TEMP=`mktemp -d`
    INCLUDE_DIR="$TEMP"

    mkdir -p "$TEMP/public/js"
    cat `find "$ASSETS" -name "*.js" | sort` \
        | go run $GO_MINIFY --type=js \
        > "$TEMP/public/js/app.js"

    mkdir -p "$TEMP/public/css"
    cat `find "$ASSETS" -name "*.css" | sort` \
        | go run $GO_MINIFY --type=css \
        > "$TEMP/public/css/app.css"

    rsync -rL --exclude="*.css" --exclude="*.js" --exclude="/public/00-dep" "$ASSETS/" "$TEMP/"
    rsync -rL --exclude="*.css" --exclude="*.js" "$ASSETS/public/00-dep/" "$TEMP/public"

    BUILD="release"

else
    INCLUDE_DIR="$ASSETS"
    INCLUDE_FLAGS="-debug"
    BUILD="debug"
fi

go run $GO_BINDATA \
    ${INCLUDE_FLAGS:-} \
    -nocompress \
    -pkg="webui" \
    -prefix="$INCLUDE_DIR" \
    -o="$ASSETS/assets.go" \
    `find "$INCLUDE_DIR" -type d`

VERSION="$(git describe --always --dirty)"
VERSION_DATE="$(date --date="@$(git show -s --format='%ct' HEAD)" '+%F')"

cd "$WORKSPACE/src"
go build \
    -ldflags "
        -X main.build=$BUILD
        -X main.version=$VERSION
        -X main.versionDate=$VERSION_DATE
    " \
    -o "$BIN/$NAME"
cd "$WORKSPACE"
