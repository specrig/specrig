#!/bin/sh
# Builds the hook from its source where Go is installed, and fetches the release's
# binary where it is not; a hook that cannot be had lets every edit through rather
# than blocking the session.
root="$(dirname "$0")/.."
data="${CLAUDE_PLUGIN_DATA:-${TMPDIR:-/tmp}/specrig-plugin}"
bin="$data/specrig-hook"
sum="$(cat "$root/go.mod" "$root"/*/*.go | cksum)"

fail() {
	rm -f "$bin.$$"
	if [ "$1" = session-start ]; then
		echo "specrig plugin: $2, so Rule #11 modes are not enforced in this session."
	fi
	exit 0
}

if [ ! -x "$bin" ] || [ "$(cat "$bin.sum" 2>/dev/null)" != "$sum" ]; then
	mkdir -p "$data"
	if command -v go >/dev/null 2>&1; then
		GOWORK=off go -C "$root" build -o "$bin.$$" ./claude 2>"$data/build.log" ||
			fail "$1" "its hook did not build (see $data/build.log)"
	else
		version="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' "$root/.claude-plugin/plugin.json")"
		os="$(uname -s | tr '[:upper:]' '[:lower:]')"
		case "$(uname -m)" in
		x86_64 | amd64) arch=amd64 ;;
		arm64 | aarch64) arch=arm64 ;;
		*) fail "$1" "there is no hook built for $(uname -m)" ;;
		esac
		name="specrig-hook-$os-$arch"
		url="https://github.com/specrig/specrig/releases/download/v$version"
		curl -fsSL "$url/$name" -o "$bin.$$" && curl -fsSL "$url/checksums.txt" -o "$data/checksums.txt" ||
			fail "$1" "Go is not installed and $name $version could not be fetched"
		want="$(grep " $name\$" "$data/checksums.txt" | cut -d' ' -f1)"
		got="$( (sha256sum "$bin.$$" 2>/dev/null || shasum -a 256 "$bin.$$") | cut -d' ' -f1)"
		[ -n "$want" ] && [ "$want" = "$got" ] || fail "$1" "the fetched $name does not match its checksum"
		chmod +x "$bin.$$"
	fi
	mv "$bin.$$" "$bin"
	echo "$sum" >"$bin.sum"
fi
exec "$bin" "$@"
