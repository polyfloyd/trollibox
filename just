#! /bin/bash

set -eu
cd `dirname $0`

NAME="trollibox"
VERSION="$(git describe --always --dirty) ($(date --date="@$(git show -s --format='%ct' HEAD)" '+%Y-%m-%d'))"
WORKSPACE="$PWD"
BIN="$WORKSPACE/bin"
LIB="$WORKSPACE/lib"
ASSETS="$WORKSPACE/src/assets"

mkdir -p "$BIN"
mkdir -p "$LIB"

usage() {
cat << EOF
USAGE
      $ $0 install    # Install dependencies
      $ $0 build      # Build a binary
EOF
}

case ${1:-} in
	"install")
		echo "*** Installing dependencies ***"
		go get -u github.com/golang/dep/cmd/dep
		dep ensure
		echo "*** Building minifier ***"
		go get -u github.com/tdewolff/minify/...
		echo "*** Building go-bindata ***"
		go get -u github.com/jteeuwen/go-bindata/...
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

			echo 'release'  > "$TEMP/_BUILD"
			echo "$VERSION" > "$TEMP/_VERSION"

		else
			INCLUDE_DIR="$ASSETS"
			INCLUDE_FLAGS="-debug"
			echo 'debug'    > "$ASSETS/_BUILD"
			echo "$VERSION" > "$ASSETS/_VERSION"
		fi

		ASSETS_OUT="$WORKSPACE/src/assets-go"
		mkdir -p "$ASSETS_OUT"

		"$GOPATH/bin/go-bindata" \
			${INCLUDE_FLAGS:-} \
			-nocompress \
			-pkg="static" \
			-prefix="$INCLUDE_DIR" \
			-o="$ASSETS_OUT/static.go" \
			`find "$INCLUDE_DIR" -type d`

		cd "$WORKSPACE/src"
		go build -o "$WORKSPACE/bin/$NAME"
		cd "$WORKSPACE"
	;;

	*)
		usage
	;;
esac
