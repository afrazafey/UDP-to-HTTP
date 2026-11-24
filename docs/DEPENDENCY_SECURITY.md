# Dependency security and inventory

Runtime Go dependencies are the SQLite driver, YAML parser, and platform and network packages declared in `go.mod`. Verify module checksums with `go mod verify`, then run `go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...` on both supported operating systems.

The shipped dashboard embeds React, React DOM, and Lucide icons. Vite, Tailwind, TypeScript, and Vitest are build and test dependencies. `npm ci` installs from the lockfile. The observed install lifecycle message identified esbuild’s platform-binary postinstall. Review lockfile and install-script changes during upgrades. Run both `npm audit` and `npm audit --omit=dev`.

CI pins CycloneDX Go module tooling and uses the lockfile-backed npm SBOM command. These are inventory artifacts. Security acceptance also requires review of scanner output, source findings, package contents, and CI changes. Dependabot covers Go, npm, and GitHub Actions. [CycloneDX Go tooling](https://github.com/CycloneDX/cyclonedx-gomod)
