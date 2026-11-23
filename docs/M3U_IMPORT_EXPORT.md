# M3U import and export

Streams retain ID/name/source/interface/startup/buffer settings and add `logo_url`, `tvg_id`, `tvg_name`, `group`, `sort_order`, `description`, and `preferred_playback_port` (`primary` or `secondary`). Metadata lengths and control characters are validated. Source identity remains unique; Duplicate channel pre-fills an add form and requires a unique ID/source/interface before saving.

## Import

Playlists & Import accepts `.m3u`/`.m3u8` files or pasted UTF-8 text, up to 1 MiB and 1,000 entries (JSON framing can impose a smaller effective limit). The file is read locally and submitted as bounded JSON; no server filenames are accepted. `#EXTM3U` and quoted `#EXTINF` attributes are parsed, including `tvg-id`, `tvg-name`, `tvg-logo` and `group-title`.

Supported sources:

```text
udp://@239.1.1.61:1234
udp://239.1.1.61:1234
udp://10.14.0.5:1234
http://10.14.0.5:7781/udp/239.1.1.61:1234
```

The last form is converted to a native multicast source, never fetched. Arbitrary HTTP video/HLS URLs, native `/live/ID.ts` exports, RTP, source hostnames, credentials, query strings, fragments and other paths are unsupported. Exported native playback playlists are player playlists, not a round-trip UDP source format. Logo URLs are stored metadata; there is no backend logo downloader or proxy.

Choose an actual local UDP interface and preview. Rows show name, address/port, type/interface, group, duplicate status and current validation error. Sources must already comply with **active** allowlists, reserved exclusions and interface policy. Imports do not authorize sources. Separately review rejected groups/senders in Security settings, apply the authorized policy/restart when needed, then preview again.

New valid rows are selected initially. Existing sources map to their stable ID and are skipped unless the update option is enabled and selected. Existing receiver tuning, enabled/startup state, description and playback-port preference are preserved; name/order and explicitly supplied metadata attributes are updated. An explicitly empty quoted metadata attribute clears that attribute. Duplicates inside the file are rejected. Import selected/all valid requires confirmation. Updating a channel replaces its receiver and disconnects viewers. The server independently revalidates all selected rows and source/bind conflicts; one invalid row or failed database commit rejects the entire selection. No partial import is claimed. Validation errors can be downloaded as JSON.

API: POST `/api/v1/import/preview` with `{ "text": "#EXTM3U\n...", "interface": "ens18" }`; POST `/api/v1/import/commit` with `{ "channels": [ ...preview channel objects... ], "update_existing": false, "confirm": true }`. Both require normal management guards, CSRF, exact JSON and the 1 MiB body limit. Preview is not an authorization token; commit validates again against active policy.

## Export

Set YAML `playlist.advertised_host` to an explicitly reachable IPv4/hostname independently of the listen address; omit scheme and port. Select group and primary/secondary/channel-preferred playback port. Enabled channels export in sort-order/ID order with name, TVG metadata, group, logo and native `/live/ID.ts` URL. Individual M3U buttons use `id` filtering. Missing advertised host or a disabled selected secondary listener produces an error instead of exporting a loopback URL.

Management download: GET `/api/v1/playlist.m3u?port=preferred&group=MUSIC` (`id=trace-urban` optionally). MIME is `audio/x-mpegurl`; the dashboard supports download/copy. The default copy URL requires local management or authenticated trusted-proxy access; it is not a public API.

Remote player download is a separate explicit opt-in:

```yaml
playlist:
  advertised_host: 192.168.10.103
  remote_enabled: true
  allowed_client_networks: [192.168.10.0/24]
```

Then `/playlist.m3u` is available on a playback listener and must pass **both** this playlist CIDR list and `streaming.allowed_client_networks`. It exposes only enabled playlist metadata/URLs, not administrative routes. Forwarded client IP headers are ignored. This is network-restricted access, not signed-token authentication. Prefer remote_enabled false if URLs/metadata should remain local. Review logo destinations before opening an untrusted playlist in a player; those clients may fetch logos independently.
