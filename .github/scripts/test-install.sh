#!/bin/sh
# Runs install.sh against a release built from this checkout and served from a file:// URL:
# it installs, it refuses a tampered archive, and it fails on a release that doesn't exist.
set -eu

version=v0.0.0-test
tmp=$(mktemp -d)
trap 'rm -rf "$tmp" dist' EXIT

host="$(go env GOOS)/$(go env GOARCH)"
.github/scripts/build-release.sh "$version" "$host"
mkdir -p "$tmp/releases/download"
cp -r dist "$tmp/releases/download/$version"
export YAWNTEST_BASE_URL="file://$tmp/releases"

./install.sh -b "$tmp/bin" "$version"
got=$("$tmp/bin/yawntest" --version)
case $got in *"$version"*) ;; *) echo "installed binary says $got, want $version"; exit 1 ;; esac

archive=$(cd dist && ls yawntest_*)
echo tampered >> "$tmp/releases/download/$version/$archive"
if ./install.sh -b "$tmp/bin2" "$version" 2> "$tmp/err"; then
	echo "install.sh accepted a tampered archive"
	exit 1
fi
grep -q "checksum mismatch" "$tmp/err" || { cat "$tmp/err"; exit 1; }

if ./install.sh -b "$tmp/bin3" v9.9.9 2> "$tmp/err"; then
	echo "install.sh installed a release that doesn't exist"
	exit 1
fi
[ ! -e "$tmp/bin2/yawntest" ] && [ ! -e "$tmp/bin3/yawntest" ] || { echo "a failed install left a binary"; exit 1; }
echo "install.sh ok"
