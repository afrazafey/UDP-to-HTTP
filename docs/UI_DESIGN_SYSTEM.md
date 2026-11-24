# Operations dashboard design system

Production branding identifies the deployment environment independently of the service health badge. Release maturity and native acceptance are visible in System Information. The sidebar and mobile drawer contain Afraaz’s attribution and a link to the developer website. Fonts, icons, scripts, and styles ship with the application.

The shell uses a 218px desktop rail and an 80px collapsed rail. Below 1024px, navigation moves to a focus-managed drawer. The topbar shows the route, service state, local instance, activity shortcut, and theme preference. The dashboard uses the existing revisioned settings API to save theme changes.

The theme uses local system fonts, tabular digits, flat surfaces, thin borders, restrained blue controls, and semantic colors paired with text. Light mode uses #F8F9FC for the background and #101828 for primary text. Dark mode uses charcoal backgrounds and slightly lighter cards. Components share the existing CSS tokens and outline icons. Styles and icons ship with the app and work under its existing CSP.

- **Overview** uses five compact metric cards with measured input and output sparklines. The desktop layout puts throughput and channel status on the left, with service health and recent activity on the right. The sidebar shows uptime, process CPU, and resident memory when available.
- **Channel work** keeps search, group, and status visible, with additional filters and sort order in a disclosure. The table groups source, throughput, and receiver activity so more channels fit on screen. Each row has a disclosure for receiver actions and downloads.
- **Playlists and import** share the same export and import components. Export options sit next to the playlist preview on desktop. Import places file upload and interface selection next to the playlist text area, then displays validation before confirmation.
- **Network and diagnostics** show configured topology and the observed socket, membership, UDP, TS, and HTTP states separately.
- **Monitoring** puts measured throughput next to resource cards. **Clients and logs** keep search and filters with the results. **Settings** uses keyboard-accessible tabs, and **System Information** places listener status above detailed runtime and storage information.

The throughput chart offers time ranges based on the measured sample span. The **Inspect samples** disclosure contains a keyboard-accessible slider that reads out the timestamp and both measured values. The default aggregate retention is about 12 minutes, so the 15-minute option requires a longer data span.

Interactive states include a visible keyboard outline, native disclosure controls and a focus trap with Escape and focus restoration. Tables in the operations pages become labeled record cards when their containers are too narrow for columns. The Playwright check exercises 12 routes at 320, 375, 390, 430, 768, 1024, 1280, 1360, 1440, and 1920px.
