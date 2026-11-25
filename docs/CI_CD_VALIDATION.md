# CI and release validation

The previous coverage step used the default matrix shell and ran `go test -coverprofile=build/coverage.out ./...`. Windows PowerShell splits the unquoted dotted expression after the slash. Repeating that exact command locally reproduced the extra `.out` package and produced a profile at the truncated path. Quoting the argument removed the extra package. GitHub documents the Windows default shell and how to select Bash explicitly. [Workflow shell defaults](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)

The reusable workflow now selects Bash on every runner for this step.

```bash
mkdir -p build/coverage
go test "-covermode=atomic" "-coverprofile=build/coverage/coverage.out" ./...
go tool cover "-func=build/coverage/coverage.out" | tee build/coverage/summary.txt
```

From PowerShell, use the same quoted native arguments and check exit codes.

```powershell
New-Item -ItemType Directory -Force build/coverage | Out-Null
go test "-covermode=atomic" "-coverprofile=build/coverage/coverage.out" ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go tool cover "-func=build/coverage/coverage.out"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
```

The matrix builds the frontend before Go compilation on Ubuntu 24.04 AMD64, Ubuntu 24.04 ARM64, and Windows AMD64. Each job retains its own unit and race JSON streams, coverage profile and summary, and platform diagnostics. Tests retain their failing exit status. The AMD64 Linux browser gate uses an isolated daemon and UDP sender. Both Ubuntu runners exercise smoke, fan-out, package lifecycle, and systemd. Windows verifies native SCM and standalone fan-out.

Thin CLI entrypoints and linked metadata have limited standalone coverage. The packaged version check verifies build metadata and system tests indirectly exercise the entrypoint. Embedded static assets are tested via HTTP and browser checks rather than by statement counts. Windows service branches covered by SCM cannot be inferred from Linux coverage.

The manual release workflow requires the same matrix, builds the candidate, verifies the Debian manifests and SHA-256 files, and retains artifacts. Its draft operation is explicitly selected by the caller and uses the release approval environment. Configure that environment with required reviewers before approving a draft. Stable promotion remains a repository owner decision.

The release workflow runs CycloneDX in application mode with the repository root as the module directory and `-main cmd/udphttp-proxy` as the executable entry point. The Go module root has no Go package. The workflow generates the Linux AMD64 SBOM with CGO disabled, matching the release build constraints. [CycloneDX application options](https://github.com/CycloneDX/cyclonedx-gomod/tree/v1.12.0#app)

The Linux verification runners restore the downloaded daemon's execute bit after checking candidate hashes. Zipped GitHub Actions artifacts drop file mode bits. This restores execution for browser and streaming smoke tests on both Linux architectures. [GitHub Actions artifact permissions](https://github.com/actions/upload-artifact#permission-loss)

The Windows backup tests exercise runner paths that may contain an NTFS short name such as `RUNNER~1`. Backup and restore validate each directory entry so ordinary aliases work while symbolic links and linked directories are rejected.
