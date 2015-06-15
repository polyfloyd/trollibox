#! /bin/bash

# Requires: https://github.com/jteeuwen/go-bindata

cd `dirname $0`

BIN="$PWD/trollibox"
ASSETS="$PWD/src/assets"
ASSETS_OUT="$PWD/src/assets-go"
DIR_BOWER="$PWD/bower_components"
VERSION="$(git describe --always --dirty) ($(date --date="@$(git show -s --format='%ct' HEAD)" '+%Y-%m-%d'))"


bower install
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


if [ $RELEASE ]; then
	MIN_JS="$PWD/node_modules/.bin/uglifyjs"
	MIN_CSS="$PWD/node_modules/.bin/minify"

	if [ ! -e "$MIN_JS" ]; then
		npm install "uglifyjs" "minify"
	fi

	TEMP=`mktemp -d`

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

	INCLUDE_DIR="$TEMP"

else
	INCLUDE_DIR="$ASSETS"
	INCLUDE_FLAGS="-debug"
	echo 'debug'    > "$ASSETS/_BUILD"
	echo "$VERSION" > "$ASSETS/_VERSION"
fi


mkdir -p "$ASSETS_OUT"

go-bindata \
	$INCLUDE_FLAGS \
	-nocompress \
	-pkg="static" \
	-prefix="$INCLUDE_DIR" \
	-o="$ASSETS_OUT/static.go" \
	`find "$INCLUDE_DIR" -type d` \
	|| exit 1


pushd "src" > /dev/null
go build -o "$BIN" || exit 1
popd > /dev/null
