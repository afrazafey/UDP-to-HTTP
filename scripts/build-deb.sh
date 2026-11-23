#!/bin/sh
# Run on Debian/Ubuntu with dpkg-deb. Build the frontend and binary first.
set -eu
arch=${1:?Usage: build-deb.sh amd64|arm64 version binary}
version=${2:?version required}
binary=${3:?binary path required}
case "$arch" in amd64|arm64) ;; *) echo 'Unsupported architecture' >&2; exit 1 ;; esac
case "$version" in *[!0-9A-Za-z.+:~_-]*|'') echo 'Invalid Debian version' >&2; exit 1 ;; esac
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
stage=$(mktemp -d)
trap 'rm -rf -- "$stage"' EXIT HUP INT TERM
mkdir -p "$stage/DEBIAN" "$stage/usr/bin" "$stage/etc/udphttp-proxy" "$stage/usr/lib/systemd/system" "$stage/usr/share/doc/udphttp-proxy" "$root/build"
install -m 0755 "$binary" "$stage/usr/bin/udphttp-proxy"
install -m 0644 "$root/deploy/config.yaml" "$stage/etc/udphttp-proxy/config.yaml"
install -m 0644 "$root/deploy/udphttp-proxy.service" "$stage/usr/lib/systemd/system/udphttp-proxy.service"
install -m 0644 "$root/README.md" "$root/ARCHITECTURE.md" "$root/ROADMAP.md" "$root/PHASE1_REPORT.md" "$root/PHASE2_REPORT.md" "$root/PHASE3_AUDIT.md" "$root/PHASE3_REPORT.md" "$root/LINUX_TESTING.md" "$stage/usr/share/doc/udphttp-proxy/"
mkdir -p "$stage/usr/share/doc/udphttp-proxy/deploy"
install -m 0644 "$root/deploy/NGINX.md" "$root/deploy/nginx-management.conf" "$stage/usr/share/doc/udphttp-proxy/deploy/"
for hook in postinst prerm postrm; do
    install -m 0755 "$root/packaging/debian/$hook" "$stage/DEBIAN/$hook"
done
printf '%s\n' /etc/udphttp-proxy/config.yaml > "$stage/DEBIAN/conffiles"
cat > "$stage/DEBIAN/control" <<EOF
Package: udphttp-proxy
Version: $version
Architecture: $arch
Section: net
Priority: optional
Maintainer: UDPHTTP Proxy maintainers <maintainers@example.invalid>
Depends: adduser, init-system-helpers
Description: UDPHTTP Linux UDP to HTTP MPEG-TS streaming proxy
 Multicast and unicast reception with shared HTTP playback, local management,
 and bounded per-client buffering. Development build; Linux acceptance pending.
 See the validation report before deployment.
EOF
dpkg-deb --root-owner-group --build "$stage" "$root/build/udphttp-proxy_${version}_${arch}.deb"
