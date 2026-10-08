#!/bin/sh
# Builds a drag-to-install disk image from build/bin/mssh.app: the app and a
# link to /Applications, nothing else. Used by hand and by release.yml, so the
# two can never drift apart.
#
# Usage: scripts/dmg.sh <output.dmg>
set -eu

app=build/bin/mssh.app
out=${1:?usage: scripts/dmg.sh <output.dmg>}

# `wails dev` leaves an app bundle with no binary in it. Packaging that gives a
# disk image that looks fine and opens nothing.
if [ -z "$(ls -A "$app/Contents/MacOS" 2>/dev/null)" ]; then
    echo "$app has no binary in it; run wails build first" >&2
    exit 1
fi

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

cp -R "$app" "$stage/"
ln -s /Applications "$stage/Applications"

hdiutil create -volname mssh -srcfolder "$stage" -ov -format UDZO "$out"
