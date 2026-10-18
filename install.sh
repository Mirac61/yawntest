#!/bin/sh
# Installs the yawntest binary from a GitHub release, checked against the release's checksums.
#
#   curl -sSfLO https://raw.githubusercontent.com/Mirac61/yawntest/main/install.sh && sh install.sh
#   curl -sSfLO https://raw.githubusercontent.com/Mirac61/yawntest/main/install.sh && sh install.sh -b /usr/local/bin v0.3.0
#
# -b DIR   where to put the binary (default: $BINDIR, or ~/.local/bin)
# VERSION  a release tag like v0.3.0 (default: the latest release)
#
# With Go installed, `go install github.com/Mirac61/yawntest/cmd/yawntest@latest` does the same.
set -eu

repo=Mirac61/yawntest
base=${YAWNTEST_BASE_URL:-https://github.com/$repo/releases} # overridable for tests
bindir=${BINDIR:-$HOME/.local/bin}
version=latest

fail() {
	echo "yawntest install: $*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case $1 in
	-b) [ $# -ge 2 ] || fail "-b needs a directory"; bindir=$2; shift 2 ;;
	-h | --help) sed -n '2,10s/^# \{0,1\}//p' "$0" 2>/dev/null || true; exit 0 ;;
	-*) fail "unknown flag $1" ;;
	*) version=$1; shift ;;
	esac
done

case $(uname -s) in
Linux) os=linux ;;
Darwin) os=darwin ;;
MINGW* | MSYS* | CYGWIN*) os=windows ;;
*) fail "no release for $(uname -s); use go install instead" ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) fail "no release for $(uname -m); use go install instead" ;;
esac

command -v curl >/dev/null || fail "needs curl"
if [ "$version" = latest ]; then
	# The latest-release page redirects to .../tag/vX.Y.Z.
	url=$(curl -sSfL -o /dev/null -w '%{url_effective}' "$base/latest") || fail "can't find the latest release"
	version=${url##*/}
	case $version in v*) ;; *) fail "can't find the latest release (got $url)" ;; esac
fi

name="yawntest_${version#v}_${os}_${arch}"
archive="$name.tar.gz"
binary=yawntest
if [ "$os" = windows ]; then
	archive="$name.zip"
	binary=yawntest.exe
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "yawntest install: downloading $version for $os/$arch"
curl -sSfL -o "$tmp/$archive" "$base/download/$version/$archive" || fail "no $archive in release $version"
curl -sSfL -o "$tmp/checksums.txt" "$base/download/$version/checksums.txt" || fail "no checksums.txt in release $version"

# Only the line for this archive, so other entries can't make the check pass.
grep "  $archive\$" "$tmp/checksums.txt" > "$tmp/expected" || fail "$archive is not in checksums.txt"
if command -v sha256sum >/dev/null; then
	(cd "$tmp" && sha256sum -c expected >/dev/null) || fail "checksum mismatch for $archive"
elif command -v shasum >/dev/null; then
	(cd "$tmp" && shasum -a 256 -c expected >/dev/null) || fail "checksum mismatch for $archive"
else
	fail "needs sha256sum or shasum to check the download"
fi

if [ "$os" = windows ]; then
	(cd "$tmp" && unzip -q "$archive") || fail "can't unpack $archive"
else
	tar -xzf "$tmp/$archive" -C "$tmp" || fail "can't unpack $archive"
fi
mkdir -p "$bindir"
cp "$tmp/$name/$binary" "$bindir/$binary"
chmod 0755 "$bindir/$binary"
echo "yawntest install: $("$bindir/$binary" --version) installed to $bindir/$binary"

case :$PATH: in
*:"$bindir":*) ;;
*) echo "yawntest install: $bindir is not in your PATH; add it, or call $bindir/$binary" ;;
esac
