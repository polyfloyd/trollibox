#!/bin/bash

set -eux
cd `dirname $0`

NAME="trollibox"
WORKSPACE="$PWD"
BIN="$WORKSPACE/bin"
WEBUI="$WORKSPACE/src/handler/webui"

mkdir -p "$BIN"

cd $WEBUI
npm run build
cd $WORKSPACE

if [ ${RELEASE:-} ]; then
    BUILD="release"
else
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
