#!/bin/bash

set -eux

main() {
	if [[ -n ${GOARM:-} ]]; then
		export ONAME="trollibox-${TRAVIS_TAG}_${GOARCH}v$GOARM-$GOOS"
	else
		export ONAME="trollibox-${TRAVIS_TAG}_$GOARCH-$GOOS"
	fi
	export RELEASE=1
	./just build

	stage=`mktemp -d`

	src=$PWD
	cp ./bin/$ONAME $stage/
	cp ./README.md $stage/
	cp ./config.example.json $stage/
	cd $stage
	tar czf $src/$ONAME.tar.gz *
	cd $src

	rm -rf $stage
}

main
