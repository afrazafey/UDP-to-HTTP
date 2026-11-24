#!/usr/bin/env bash
set -euo pipefail

package=$(realpath "${1:?candidate .deb required}")
arch=${2:?architecture required}
[[ ${GITHUB_ACTIONS:-} == true && $(dpkg --print-architecture) == "$arch" ]]
[[ $(ps -p 1 -o comm=) == systemd ]]
[[ ! -e /etc/udphttp-proxy && ! -e /var/lib/udphttp-proxy ]]
! dpkg-query -W udphttp-proxy 2>/dev/null
dpkg-deb --info "$package"
dpkg-deb --contents "$package"
dpkg-deb --fsys-tarfile "$package" | tar -tvf -

work=$(mktemp -d)
trap 'sudo rm -rf -- "$work"' EXIT
install_package() {
    sudo env DEBIAN_FRONTEND=noninteractive dpkg --force-confdef --force-confold -i "$1"
    [[ $(dpkg-query -W -f='${Status}' udphttp-proxy) == 'install ok installed' ]]
    getent passwd udphttp-proxy
    test -x /usr/bin/udphttp-proxy
    [[ $(stat -c '%U:%G %a' /var/lib/udphttp-proxy) == 'udphttp-proxy:udphttp-proxy 700' ]]
    [[ $(stat -c '%U:%G %a' /etc/udphttp-proxy/config.yaml) =~ ^root:(root\ 644|udphttp-proxy\ 640)$ ]]
    sudo systemctl daemon-reload
    sudo systemctl enable --now udphttp-proxy
    systemctl is-enabled udphttp-proxy
    systemctl is-active udphttp-proxy
    curl -fsS --retry 8 --retry-connrefused http://127.0.0.1:8080/health
}

# Exercise the candidate on a clean host before introducing the upgrade fixture.
install_package "$package"
sudo systemd-analyze verify /usr/lib/systemd/system/udphttp-proxy.service
sudo dpkg --remove udphttp-proxy
test -f /etc/udphttp-proxy/config.yaml
sudo test -f /var/lib/udphttp-proxy/udphttp-proxy.db
sudo dpkg --purge udphttp-proxy
test ! -e /etc/udphttp-proxy/config.yaml
sudo mv /var/lib/udphttp-proxy "$work/fresh-state"

baseline=35204787ef5f0efb726c72216a60fcbbb62c2846
mkdir "$work/baseline"
git archive "$baseline" | tar -x -C "$work/baseline"
(
    cd "$work/baseline"
    make build "COMMIT=$baseline"
    sh scripts/build-deb.sh "$arch" 0.3.0~dev build/udphttp-proxy
)
install_package "$work/baseline/build/udphttp-proxy_0.3.0~dev_$arch.deb"
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py seed
sudo systemctl restart udphttp-proxy
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py check 3
dpkg-deb -x "$package" "$work/candidate"
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py migration-failure "$work/candidate/usr/bin/udphttp-proxy" "$work"
sudo cp /etc/udphttp-proxy/config.yaml "$work/modified.yaml"
install_package "$package"
sudo cmp "$work/modified.yaml" /etc/udphttp-proxy/config.yaml
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py check 4
install_package "$package"
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py check 4
sudo cmp "$work/modified.yaml" /etc/udphttp-proxy/config.yaml
sudo systemctl stop udphttp-proxy
! systemctl is-active --quiet udphttp-proxy
sudo systemctl start udphttp-proxy
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py check 4
sudo dpkg --remove udphttp-proxy
test -f /etc/udphttp-proxy/config.yaml
sudo dpkg --purge udphttp-proxy
test ! -e /etc/udphttp-proxy/config.yaml
sudo test -f /var/lib/udphttp-proxy/udphttp-proxy.db

# Upgrade directly from the previous release candidate as well.
sudo mv /var/lib/udphttp-proxy "$work/schema4-state"
previous=83f4008862e0696de425c9891f434bf229d26672
mkdir "$work/previous"
git archive "$previous" | tar -x -C "$work/previous"
(
    cd "$work/previous"
    npm --prefix frontend ci
    npm --prefix frontend run build
    mkdir -p build
    CGO_ENABLED=0 go build -trimpath -o build/udphttp-proxy ./cmd/udphttp-proxy
    python scripts/build-deb.py "$arch" 0.4.2~rc.1 build/udphttp-proxy build
)
install_package "$work/previous/build/udphttp-proxy_0.4.2~rc.1_$arch.deb"
systemd-analyze --no-pager security udphttp-proxy.service
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py seed
sudo systemctl restart udphttp-proxy
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py create 4
sudo cp /etc/udphttp-proxy/config.yaml "$work/previous.yaml"
install_package "$package"
[[ $(stat -c '%U:%G %a' /etc/udphttp-proxy/config.yaml) == 'root:udphttp-proxy 640' ]]
sudo cmp "$work/previous.yaml" /etc/udphttp-proxy/config.yaml
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py check 4
systemd-analyze --no-pager security udphttp-proxy.service
sudo env GITHUB_ACTIONS=true python3 scripts/debian-upgrade-fixture.py multicast
sudo dpkg --purge udphttp-proxy
sudo test -f /var/lib/udphttp-proxy/udphttp-proxy.db
echo 'Debian lifecycle passed'
