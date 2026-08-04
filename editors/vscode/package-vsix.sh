#!/usr/bin/env bash
# Usage: package-vsix.sh <version> <archive-directory> <output-directory>
#
# Builds one VSIX per platform carrying the released koment binary for that
# platform, plus a universal VSIX that carries none. Archives are the canonical
# release artifacts and are verified against the release checksum manifest
# before anything is packaged (ADR 0109, ADR 0113).
set -euo pipefail

version=${1:?version}
archives=$(cd "${2:?archive directory}" && pwd)
output=${3:?output directory}

extension=$(cd "$(dirname "$0")" && pwd)
repository=$(cd "$extension/../.." && pwd)
vsce="$extension/node_modules/.bin/vsce"
manifest="$archives/koment_${version}_checksums.txt"

mkdir -p "$output"
output=$(cd "$output" && pwd)

cp "$repository/LICENSE" "$extension/LICENSE"
cp "$repository/internal/ui/assets/koment-logo.png" "$extension/icon.png"

verify() {
  local archive=$1
  (
    cd "$archives"
    if command -v sha256sum >/dev/null; then
      grep " ${archive}\$" "$manifest" | sha256sum -c -
    else
      grep " ${archive}\$" "$manifest" | shasum -a 256 -c -
    fi
  )
}

# vsce target -> koment archive platform. Every koment archive has exactly one
# VS Code target, so a platform that stops being built stops being packaged.
targets="linux-x64:linux_amd64 linux-arm64:linux_arm64 darwin-x64:darwin_amd64 darwin-arm64:darwin_arm64 win32-x64:windows_amd64 win32-arm64:windows_arm64"

for pair in $targets; do
  target=${pair%%:*}
  platform=${pair##*:}

  rm -rf "$extension/bin"
  mkdir -p "$extension/bin"

  case "$platform" in
    windows_*)
      archive="koment_${version}_${platform}.zip"
      verify "$archive"
      unzip -q -o "$archives/$archive" koment.exe -d "$extension/bin"
      ;;
    *)
      archive="koment_${version}_${platform}.tar.gz"
      verify "$archive"
      tar -xzf "$archives/$archive" -C "$extension/bin" koment
      chmod +x "$extension/bin/koment"
      ;;
  esac

  (cd "$extension" && "$vsce" package --target "$target" \
    --out "$output/koment-vscode_${version}_${target}.vsix")
done

rm -rf "$extension/bin"
(cd "$extension" && "$vsce" package --out "$output/koment-vscode_${version}.vsix")

rm -f "$extension/LICENSE" "$extension/icon.png"
ls -1 "$output"
