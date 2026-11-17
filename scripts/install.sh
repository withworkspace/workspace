#!/usr/bin/env bash
# Install ws from a GitHub release: the build for this machine, with its web
# UI and code navigation in. It downloads with curl.
#
#   scripts/install.sh [version] [dir]
#
# version is a release's tag (v0.0.3); without one, the latest release, which
# is never a prerelease. dir is where workspace goes: $WS_INSTALL_DIR, or
# ~/.local/bin. Without a clone:
#
#   curl -fsSL https://raw.githubusercontent.com/withworkspace/workspace/main/scripts/install.sh | bash -s -- [version] [dir]
set -euo pipefail

repo="${WS_REPO:-withworkspace/workspace}"
version="${1:-}"
dir="${2:-${WS_INSTALL_DIR:-$HOME/.local/bin}}"

fail() {
	echo "$*" >&2
	exit 1
}

command -v curl >/dev/null || fail "curl not found"

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) fail "there's no release of ws for $(uname -s)" ;;
esac
case "$(uname -m)" in
arm64 | aarch64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*) fail "there's no release of ws for $(uname -m)" ;;
esac
# Releases build macOS for Apple silicon only (see .github/workflows/release.yml).
[ "$os/$arch" != darwin/amd64 ] || fail "there's no release of ws for Intel Macs: build from a clone with just build"

if [ -z "$version" ]; then
	# releases/latest redirects to the latest release's page, .../tag/<version>.
	latest="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")" ||
		fail "couldn't find the latest release of $repo"
	version="${latest##*/tag/}"
	[ "$version" != "$latest" ] || fail "$repo has no release yet"
fi
asset="workspace_${version}_${os}_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "downloading $asset from $repo"
base="https://github.com/$repo/releases/download/$version"
curl -fsSL -o "$tmp/$asset" "$base/$asset" && curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" ||
	fail "couldn't download $asset from $repo's release $version"

# The release lists every build's checksum: check this one's before it runs.
grep -q "  $asset\$" "$tmp/checksums.txt" || fail "$asset isn't in the release's checksums.txt"
(
	cd "$tmp"
	if command -v sha256sum >/dev/null; then
		grep "  $asset\$" checksums.txt | sha256sum -c --quiet -
	else
		grep "  $asset\$" checksums.txt | shasum -a 256 -c --quiet -
	fi
) || fail "$asset doesn't match its checksum"

tar -xzf "$tmp/$asset" -C "$tmp" workspace
mkdir -p "$dir"
if [ -L "$dir/workspace" ]; then
	echo "replacing the symlink $dir/workspace (to $(readlink "$dir/workspace")) with the release"
fi
# Written beside it, then renamed over it: a ws that's running (the daemon)
# keeps the copy it started from, rather than having it changed underneath.
cp "$tmp/workspace" "$dir/.workspace.new"
chmod 755 "$dir/.workspace.new"
mv -f "$dir/.workspace.new" "$dir/workspace"

echo "installed $("$dir/workspace" --version) at $dir/workspace"
case ":$PATH:" in
*":$dir:"*)
	if [ "$(command -v workspace)" != "$dir/workspace" ]; then
		echo "but $(command -v workspace) comes before it on your PATH"
	fi
	;;
*) echo "$dir isn't on your PATH: add it, then eval \"\$(workspace shell-init zsh)\" in ~/.zshrc for ws" ;;
esac
echo "a running daemon keeps its old ws until it restarts"
