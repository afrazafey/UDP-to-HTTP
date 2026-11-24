# Management behind same-host Nginx (Ubuntu 24.04)

The daemon still binds **127.0.0.1:8080**. Opt-in reverse-proxy support permits an exact external origin through explicitly trusted local TCP peers. Nginx must authenticate and authorize **every** dashboard, asset, health, token, and API request. The daemon does not add a second password system or trust forwarded usernames. Existing local users remain trusted administrators.

## Backend configuration

Merge this into `/etc/udphttp-proxy/config.yaml`, retaining the existing timeout, streaming, storage, and logging settings:

```yaml
management:
  host: 127.0.0.1
  port: 8080
  reverse_proxy:
    enabled: true
    trusted_proxies: ["127.0.0.1"]
    trusted_origins: ["http://192.168.10.103"]
```

The proxy address is the **actual upstream TCP peer**, not Nginx's LAN address or the Windows client. With `proxy_pass http://127.0.0.1:8080`, it is normally `127.0.0.1`. Because management stays IPv4 loopback-only, only literal IPv4 loopback proxy addresses are accepted; no CIDRs or hostnames. Any local process sharing the trusted address has the existing local-administrator privileges. This is a same-host proxy deployment, not a remote backend listener.

Origins use exact serialized `scheme://host[:port]` matching. Use lowercase DNS names or canonical IPs, no wildcard, slash, path, credentials, query, fragment, or trailing dot. Scheme and port must match what the browser sends. For ordinary HTTP port 80, browsers send `http://192.168.10.103`, not `http://192.168.10.103:80`; the latter is a different spelling and is rejected unless explicitly configured. A nondefault HTTPS port needs e.g. `https://dashboard.example:8443` in both the browser and allowlist. Configuring another approved origin does not permit it to make cross-origin calls to this vhost.

These security settings are YAML-only, survive persisted dashboard settings, and require a daemon restart. They cannot be changed through the management settings API. Defaults in both sample configurations disable proxy support and have empty trust lists.

## Nginx configuration

Review [nginx-management.conf](nginx-management.conf) and merge it into the existing vhost. Preserve the existing password file (adjust the example path) and existing security controls. Do not activate a second server block on the same address/name. The example requires **both** Basic authentication and client `192.168.10.102`, using `satisfy all`; it proxies every path so assets and API requests inherit the same policy. Do not create unauthenticated `/assets/`, `/health`, or `/api/v1/csrf` exceptions. Nginx documents [combined IP and password restrictions](https://docs.nginx.com/nginx/admin-guide/security-controls/configuring-http-basic-authentication/) and [upstream header handling](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/).

Nginx overwrites `X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Forwarded-For`, and `X-Forwarded-Port`. Use `$remote_addr`, **not** `$proxy_add_x_forwarded_for`; multi-proxy chains are intentionally unsupported. With this single-hop layout, do not enable `real_ip_header`/`set_real_ip_from` for arbitrary LAN clients: the IP allowlist and `$remote_addr` must use the actual Windows TCP peer. Incoming `Forwarded`, `X-Real-IP`, and `X-Forwarded-Prefix` are cleared. Other unsupported `X-Forwarded-*` fields fail closed at the backend.

Preserve `Origin`, `Referer`, `Sec-Fetch-Site`, and `X-CSRF-Token`. Do not replace them with trusted values, strip them to suppress errors, add permissive CORS headers, or override response MIME types. HTML, JS, CSS, and API calls are same-origin through Nginx. The example sends `$http_host` as both `Host` and `X-Forwarded-Host` so an incomplete trust configuration rejects the initial navigation. Older configurations that replace `Host` with the loopback address can load the HTML and then reject the browser's origin-bearing assets. The external origin is accepted only with all three single-valued `X-Forwarded-Host/Proto/For` headers from a trusted TCP peer; `X-Forwarded-Port` is optional but must agree if present. All duplicate, comma-separated, empty, contradictory, or unapproved security headers are denied with 403. Forwarded client IPs never grant backend administrative privileges or replace the actual peer.

Basic credentials on HTTP travel without transport encryption; use HTTPS for deployment outside this explicitly trusted LAN. For HTTPS, terminate TLS in this vhost and update the exact origin to `https://...`; keep the upstream on loopback HTTP.

## Build and apply manually

If the installed binary already supports `management.reverse_proxy`, enabling it in the active configuration and updating the vhost is enough to restore dashboard access. Build a new binary to ship the updated embedded frontend. From the source root on Ubuntu with the supported Go/Node toolchains:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
mkdir -p build
CGO_ENABLED=0 go build -trimpath -o build/udphttp-proxy ./cmd/udphttp-proxy
./build/udphttp-proxy config validate --config /etc/udphttp-proxy/config.yaml
sudo nginx -t
```

After reviewing the configuration, deploy the new binary through your existing package/install process and restart `udphttp-proxy`, then reload Nginx. Keep the existing database and service account. Configuration validation and `nginx -t` must pass before service changes. To disable proxy access, set `management.reverse_proxy.enabled: false` and restart the daemon; local access continues working. No database rollback is required.

## Acceptance checks

On Ubuntu, direct access remains available:

```sh
curl --fail http://127.0.0.1:8080/api/v1/status
curl --fail -H 'Origin: http://127.0.0.1:8080' http://127.0.0.1:8080/health
```

From the authorized Windows client, use `curl.exe` (the POSIX commands below use `curl`) and enter the existing Nginx password when prompted; replace `ADMIN` with the existing username:

```sh
curl --fail --user ADMIN -H 'Origin: http://192.168.10.103' http://192.168.10.103/
curl --fail --user ADMIN -H 'Origin: http://192.168.10.103' http://192.168.10.103/api/v1/status
curl --user ADMIN -i -H 'Origin: http://evil.example' http://192.168.10.103/api/v1/status
```

The first two return 200, the third 403. Open the dashboard in the browser, authenticate, and inspect Network: the actual hashed JS/CSS URLs from `index.html` must return 200 with JavaScript and `text/css` MIME types, including requests carrying the LAN Origin. Verify an allowed dashboard settings change succeeds and persists; the UI retrieves `/api/v1/csrf` and sends `X-CSRF-Token` plus JSON. A mutation without that token must return 403, even with valid Basic authentication. Requests without valid Basic credentials fail at Nginx; another LAN client must be denied regardless of credentials. Close/reopen the browser or clear its Basic-auth cache when testing missing credentials.

Security tests cover default/direct behavior, origin scheme/host/port mismatches, malformed and duplicate headers, untrusted peers, forwarded spoofing, actual embedded JS/CSS responses through a test proxy, and real settings writes protected by CSRF. Run the normal CI commands in [LINUX_TESTING.md](../LINUX_TESTING.md), including `go test -race ./...`, before rollout. Nginx/TLS and browser checks above must be run on the target deployment; in-process proxy tests do not prove its password file or network ACL configuration.

## Blank dashboard after signing in

Open **Network** in browser DevTools, reload the page, and inspect the status and response body for the failed JS or CSS request. A 403 with `Cross-origin administration denied` means the backend rejected its origin. The browser may only report that it received `application/json` for the stylesheet or module.

1. Run `systemctl cat udphttp-proxy` on Ubuntu to find the `--config` path actually used by the service. In that file, enable proxy trust with `127.0.0.1` as the proxy and `http://192.168.10.103` as the exact browser origin. Validate the file with `udphttp-proxy config validate --config /etc/udphttp-proxy/config.yaml` (adjust the path), then restart the service with `sudo systemctl restart udphttp-proxy`.
2. Use `sudo nginx -T` to inspect the active vhost. Merge the example's single `location /` and forwarding headers into it, retaining Basic authentication and the IP allowlist. Remove conflicting asset or API locations from this vhost so all requests use the same proxy settings. Run `sudo nginx -t && sudo systemctl reload nginx` after editing.
3. From the authorized client, retry the actual asset URL shown in DevTools with the browser origin. In PowerShell, `curl.exe --include --user admin --header "Origin: http://192.168.10.103" "<asset URL from DevTools>"` prompts for the existing password. Expect 200 with `text/css` for CSS and a JavaScript media type for JS. A 401 with `WWW-Authenticate` indicates the Nginx password challenge. For a 404 on a hashed asset, compare the URL with a fresh `index.html` from the same binary and check for an Nginx cache or static-file location serving stale content.
