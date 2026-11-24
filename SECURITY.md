# Reporting a security issue

Use [GitHub private vulnerability reporting](https://github.com/afrazafey/UDP-to-HTTP/security/advisories/new) when it is enabled. If it is unavailable, use [Afraaz’s website](https://afraaz.xyz) to identify an established private contact.

Include the version, operating system, affected component, impact, and a description an owner can verify in an isolated environment. Remove credentials, subscriber addresses, configuration files containing secrets, and production packet captures. Coordinate disclosure with the maintainer before publishing technical details.

The current acceptance status and known limitations are tracked in the [release checklist](docs/RELEASE_CHECKLIST.md). The [deployment security guide](docs/SECURITY.md) describes the management trust model.

Playback uses the actual TCP peer and explicit allowlists. Multicast checks always reject local-control groups in `224.0.0.0/24` and give configured blocks priority. Management proxy addresses remain exact IPv4 loopback addresses. Trusted origins retain strict scheme, host, and port matching. [Network policies](docs/NETWORK_CONFIGURATION.md)
