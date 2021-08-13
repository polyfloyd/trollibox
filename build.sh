#! /bin/bash

set -eu
cd `dirname $0`

NAME="trollibox"
WORKSPACE="$PWD"
BIN="$WORKSPACE/bin"
LIB="$WORKSPACE/lib"
ASSETS="$WORKSPACE/src/handler/webui"
GO_MINIFY="github.com/tdewolff/minify/v2/cmd/minify"

mkdir -p "$BIN"
mkdir -p "$LIB"

rm "$ASSETS/static/js/app.js" || true
rm "$ASSETS/static/css/app.css" || true

echo "*** Building Project ***"
if [ ${RELEASE:-} ]; then
    mkdir -p "$ASSETS/static/js"
    cat `find "$ASSETS" -name "*.js" | sort` \
        | go run $GO_MINIFY --type=js \
        > "$ASSETS/static/js/app.js"

    mkdir -p "$ASSETS/static/css"
    cat `find "$ASSETS" -name "*.css" | sort` \
        | go run $GO_MINIFY --type=css \
        > "$ASSETS/static/css/app.css"

    rsync -rL "$ASSETS/static/00-dep/fonts/" "$ASSETS/static/fonts/"

    BUILD="release"

else
    INCLUDE_DIR="$ASSETS"
    INCLUDE_FLAGS="-debug"
    BUILD="debug"
fi

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
