#!/bin/sh
# Builds the release archives and checksums.txt into dist/, the way install.sh expects them.
#
#   .github/scripts/build-release.sh v0.4.0                 # every platform
#   .github/scripts/build-release.sh v0.4.0 linux/amd64     # just these
set -eu

version=$1
shift
targets=${*:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}

rm -rf dist
mkdir dist
for target in $targets; do
	os=${target%/*}
	arch=${target#*/}
	name="yawntest_${version#v}_${os}_${arch}"
	binary=yawntest
	[ "$os" = windows ] && binary=yawntest.exe
	mkdir "dist/$name"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
		-ldflags "-s -w -X main.stampedVersion=$version" -o "dist/$name/$binary" ./cmd/yawntest
	cp LICENSE README.md "dist/$name/"
	if [ "$os" = windows ]; then
		(cd dist && zip -qr "$name.zip" "$name")
	else
		tar -C dist -czf "dist/$name.tar.gz" "$name"
	fi
	rm -r "dist/$name"
done
if command -v sha256sum >/dev/null; then
	(cd dist && sha256sum -- * > checksums.txt)
else
	(cd dist && shasum -a 256 -- * > checksums.txt)
fi
