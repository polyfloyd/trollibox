#! /bin/bash

set -eu
cd `dirname $0`

NAME="trollibox"
WORKSPACE="$PWD"
BIN="$WORKSPACE/bin"
LIB="$WORKSPACE/lib"
ASSETS="$WORKSPACE/src/assets"

GO111MODULE=on

mkdir -p "$BIN"
mkdir -p "$LIB"

usage() {
cat << EOF
USAGE
      $ $0 install    # Install dependencies
      $ $0 build      # Build a binary
EOF
}

if [[ ! -v GOPATH ]]; then
	echo GOPATH is not set.
	echo See the readme for instructions.
	exit
fi

case ${1:-} in
	"install")
		echo "*** Installing dependencies ***"
		go mod download
		echo "*** Building minifier ***"
		go get -u github.com/tdewolff/minify/v2/cmd/minify
		echo "*** Building go-bindata ***"
		go get -u github.com/tmthrgd/go-bindata/...
	;;

	"build")
		echo "*** Building Project ***"
		if [ ${RELEASE:-} ]; then
			TEMP=`mktemp -d`
			INCLUDE_DIR="$TEMP"

			mkdir -p "$TEMP/public/js"
			cat `find "$ASSETS" -name "*.js" | sort` \
				| $GOPATH/bin/minify --type=js \
				> "$TEMP/public/js/app.js"

			mkdir -p "$TEMP/public/css"
			cat `find "$ASSETS" -name "*.css" | sort` \
				| $GOPATH/bin/minify --type=css \
				> "$TEMP/public/css/app.css"

			rsync -rL --exclude="*.css" --exclude="*.js" --exclude="/public/00-dep" "$ASSETS/" "$TEMP/"
			rsync -rL --exclude="*.css" --exclude="*.js" "$ASSETS/public/00-dep/" "$TEMP/public"

			BUILD="release"

		else
			INCLUDE_DIR="$ASSETS"
			INCLUDE_FLAGS="-debug"
			BUILD="debug"
		fi

		"$GOPATH/bin/go-bindata" \
			${INCLUDE_FLAGS:-} \
			-nocompress \
			-pkg="assets" \
			-prefix="$INCLUDE_DIR" \
			-o="$ASSETS/assets.go" \
			`find "$INCLUDE_DIR" -type d`

		VERSION="$(git describe --always --dirty)"
		VERSION_DATE="$(date --date="@$(git show -s --format='%ct' HEAD)" '+%F')"
		BUILD_DATE="$(date '+%F_%T_%z')"

		ONAME=${ONAME:-$NAME}
		cd "$WORKSPACE/src"
		go build \
			-ldflags "
				-X main.build=$BUILD
				-X main.version=$VERSION
				-X main.versionDate=$VERSION_DATE
				-X main.buildDate=$BUILD_DATE
			" \
			-o "$WORKSPACE/bin/$ONAME"
		cd "$WORKSPACE"
	;;

	*)
		usage
	;;
esac
