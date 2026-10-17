#!/usr/bin/env bash
# Packs build/bin/mssh into build/bin/mssh-<version>-x86_64.AppImage, carrying
# GTK and WebKitGTK so it runs where neither is installed.
#
# WebKitGTK is the awkward part. It starts two helper programs,
# WebKitWebProcess and WebKitNetworkProcess, from a directory fixed when
# WebKitGTK was compiled: /usr/lib/x86_64-linux-gnu/webkit2gtk-4.1 on Ubuntu.
# Inside an AppImage that path is the host's, and the host may not have it.
# This does what Tauri does: ship the helpers at the same path under the
# AppDir, rewrite "/usr" in the WebKit library to "././" - the same length, so
# nothing else in the file moves - and start the app from the AppDir's usr/,
# where "././lib/..." then means the shipped copy. See build/linux/AppRun.
#
# Run from the repository root, on Ubuntu 22.04, after wails build.
set -euo pipefail

version="$1"
appdir=build/AppDir
tools=build/appimage-tools
helpers=/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1

# Pinned: a new release upstream must not change what this produces.
linuxdeploy_release=1-alpha-20251107-1
appimage_plugin_release=1-alpha-20250213-1
gtk_plugin_commit=7a3fbc31a9e5075073ff8790f26effbac5f84453

rm -rf "$appdir"
mkdir -p "$tools"
curl -fsSLo "$tools/linuxdeploy" \
    "https://github.com/linuxdeploy/linuxdeploy/releases/download/$linuxdeploy_release/linuxdeploy-x86_64.AppImage"
curl -fsSLo "$tools/linuxdeploy-plugin-appimage" \
    "https://github.com/linuxdeploy/linuxdeploy-plugin-appimage/releases/download/$appimage_plugin_release/linuxdeploy-plugin-appimage-x86_64.AppImage"
curl -fsSLo "$tools/linuxdeploy-plugin-gtk.sh" \
    "https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/$gtk_plugin_commit/linuxdeploy-plugin-gtk.sh"
chmod +x "$tools"/*

# linuxdeploy looks for its plugins on PATH.
export PATH="$PWD/$tools:$PATH"
# The tools are AppImages themselves, and a CI machine has no FUSE to mount them.
export APPIMAGE_EXTRACT_AND_RUN=1

# 1. WebKit's helpers, at the path WebKit will ask for, before linuxdeploy
#    runs: it brings in the libraries of every program already in the AppDir.
mkdir -p "$appdir$helpers"
cp -r "$helpers/WebKitNetworkProcess" "$helpers/WebKitWebProcess" \
    "$helpers/injected-bundle" "$appdir$helpers/"

# 2. mssh and every library it needs, and GTK's own data and modules.
linuxdeploy --appdir "$appdir" \
    --executable build/bin/mssh \
    --desktop-file build/linux/mssh.desktop \
    --icon-file build/linux/mssh.png \
    --custom-apprun build/linux/AppRun \
    --plugin gtk

# 3. Point WebKit at the helpers shipped in step 1.
find "$appdir/usr/lib" -name 'libwebkit2gtk-4.1.so*' -type f \
    -exec sed -i -e 's|/usr|././|g' {} +

# 4. Pack, without deploying again: that could copy the unpatched library back.
OUTPUT="build/bin/mssh-$version-x86_64.AppImage" ARCH=x86_64 \
    linuxdeploy-plugin-appimage --appdir "$appdir"
