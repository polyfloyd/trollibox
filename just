#! /bin/bash

set -eu
cd `dirname $0`

NAME="trollibox"
VERSION="$(git describe --always --dirty) ($(date --date="@$(git show -s --format='%ct' HEAD)" '+%Y-%m-%d'))"
WORKSPACE="$PWD"
BIN="$WORKSPACE/bin"
LIB="$WORKSPACE/lib"
GOPATH="$LIB/gopath"
DIR_BOWER="$WORKSPACE/lib/bower"
DIR_NPM="$WORKSPACE/lib/node_modules"
ASSETS="$WORKSPACE/src/assets"
MIN_JS="$DIR_NPM/.bin/uglifyjs"
MIN_CSS="$DIR_NPM/.bin/minify"

mkdir -p "$BIN"
mkdir -p "$LIB"

usage() {
cat << EOF
SYNOPSIS

    This is a very handy tool to manage this Go project.

USAGE
      $ $0 install    # Install dependencies
      $ $0 build      # Build a binary
      $ $0 run        # Run the currently built binary
EOF
}

case ${1:-} in
	"install")
		GPM="$LIB/gpm"
		if [ ! -e "$GPM" ]; then
			echo "*** Installing Dependency Manager ***"
			wget -qO- https://raw.githubusercontent.com/pote/gpm/v1.3.2/bin/gpm > "$GPM"
			chmod 755 "$GPM"
		fi

		if [ ! -e "$MIN_JS" ]; then
			npm install --prefix "$LIB" "uglifyjs" "minify" "bower"
		fi

		echo "*** Installing Dependencies ***"
		cd "$WORKSPACE/src"
		GOPATH="$GOPATH" "$GPM" install
		cd "$WORKSPACE"

		"$DIR_NPM/.bin/bower" install
		rm -rf   "$ASSETS/public/00-dep"
		mkdir -p "$ASSETS/public/00-dep/css"
		mkdir -p "$ASSETS/public/00-dep/js"
		mkdir -p "$ASSETS/public/00-dep/fonts"
		ln -s "$DIR_BOWER/bootstrap/dist/css/bootstrap.css"                        "$ASSETS/public/00-dep/css/bootstrap.css"
		ln -s "$DIR_BOWER/bootstrap/dist/css/bootstrap.css.map"                    "$ASSETS/public/00-dep/css/bootstrap.css.map"
		ln -s "$DIR_BOWER/jquery/dist/jquery.js"                                   "$ASSETS/public/00-dep/js/00-jquery.js"
		ln -s "$DIR_BOWER/underscore/underscore.js"                                "$ASSETS/public/00-dep/js/00-underscore.js"
		ln -s "$DIR_BOWER/backbone/backbone.js"                                    "$ASSETS/public/00-dep/js/01-backbone.js"
		ln -s "$DIR_BOWER/bootstrap/dist/js/bootstrap.js"                          "$ASSETS/public/00-dep/js/bootstrap.js"
		ln -s "$DIR_BOWER/html.sortable/dist/html.sortable.js"                     "$ASSETS/public/00-dep/js/html.sortable.js"
		ln -s "$DIR_BOWER/bootstrap/dist/fonts/glyphicons-halflings-regular.eot"   "$ASSETS/public/00-dep/fonts/glyphicons-halflings-regular.eot"
		ln -s "$DIR_BOWER/bootstrap/dist/fonts/glyphicons-halflings-regular.svg"   "$ASSETS/public/00-dep/fonts/glyphicons-halflings-regular.svg"
		ln -s "$DIR_BOWER/bootstrap/dist/fonts/glyphicons-halflings-regular.ttf"   "$ASSETS/public/00-dep/fonts/glyphicons-halflings-regular.ttf"
		ln -s "$DIR_BOWER/bootstrap/dist/fonts/glyphicons-halflings-regular.woff"  "$ASSETS/public/00-dep/fonts/glyphicons-halflings-regular.woff"
		ln -s "$DIR_BOWER/bootstrap/dist/fonts/glyphicons-halflings-regular.woff2" "$ASSETS/public/00-dep/fonts/glyphicons-halflings-regular.woff2"
	;;

	"build")
		echo "*** Building Project ***"
		if [ ${RELEASE:-} ]; then
			TEMP=`mktemp -d`
			INCLUDE_DIR="$TEMP"

			mkdir -p "$TEMP/public/js"
			$MIN_JS \
				`find "$ASSETS" -name "*.js" | sort` \
				--mangle \
				--compress warnings=false\
				--screw-ie8 \
				--output "$TEMP/public/js/app.js"

			mkdir -p "$TEMP/public/css"
			cat `find "$ASSETS" -name "*.css" | sort` \
				| $MIN_CSS -css > "$TEMP/public/css/app.css"

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

		go-bindata \
			${INCLUDE_FLAGS:-} \
			-nocompress \
			-pkg="static" \
			-prefix="$INCLUDE_DIR" \
			-o="$ASSETS_OUT/static.go" \
			`find "$INCLUDE_DIR" -type d`

		cd "$WORKSPACE/src"
		GOPATH="$GOPATH" go build -o "$WORKSPACE/bin/$NAME"
		cd "$WORKSPACE"
	;;

	"run")
		"$BIN/$NAME"
	;;

	*)
		usage
	;;
esac
