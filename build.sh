#!/bin/sh
# Run as root on Linux with Go and the full apk-tools v3 builder available.
set -eu
cd "$(dirname "$0")"
builder="${APK_BUILDER:-apk}"
mkdir -p files/usr/libexec
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o files/usr/libexec/lan-port-status main.go
chmod 755 files/usr/libexec/lan-port-status
chown -R 0:0 files
"$builder" mkpkg \
 --files files \
 --info name:luci-app-lan-ports \
 --info version:1.1.0-r0 \
 --info arch:aarch64_cortex-a53 \
 --info 'description:WAN and LAN negotiated link speeds for SEED AC3' \
 --info license:MIT \
 --info 'depends:luci-base rpcd-mod-file kmod-mdio-netlink kmod-switch-rtl8373=6.12.94-r1' \
 --script pre-install:scripts/pre-install \
 --script pre-upgrade:scripts/pre-install \
 --script post-install:scripts/post-install \
 --script post-upgrade:scripts/post-install \
 --script post-deinstall:scripts/post-deinstall \
 --output luci-app-lan-ports-1.1.0-r0.apk
