# Phase 4.2 security validation

| Check | Local outcome |
| --- | --- |
| Go vulnerability scan | Pinned govulncheck reported no known vulnerabilities on Windows AMD64. CI runs platform-specific checks on Linux and Windows. |
| npm advisory database | Full and production-only audits each returned zero advisories for the current lockfile. |
| Browser and frontend | Type checking, 31 frontend tests, and production bundle passed. The suite covers an untrusted markup fixture, safe external attribution, settings controls, and API-backed views. |
| Management regressions | The Windows HTTP API suite passed Host and Origin, proxy peer and header handling, CSRF, same-site denial, duplicate media types, duplicate JSON fields, and oversized JSON. |
| Streaming regressions | Source, client, buffer, receiver, and recovery boundaries are covered by existing tests. Baseline and candidate load passed at five viewer counts. The measured Windows resource comparison is in the [release report](PHASE4_2_REPORT.md). |
| Storage and playlists | New regressions cover linked backup parent directories and unsafe stored logo schemes. Existing tests exercise schema migration, transactions, malformed playlist records, and limits. |
| Race detection | Blocked locally because the host has no GCC for cgo. |
| Go test runner | The final run recorded 31 top-level passes. Windows Application Control blocked seven package executables. Go formatting, vet and module verification passed. |
| Static security scanners | Gosec 2.24.7 could not load package types locally. Application Control blocked the pinned Gosec 2.30.0 and Gitleaks 8.30 executables. Scope and replay commands are in CI. |
| Secret checks | The local credential-pattern scan found no matches in the candidate artifacts, included source files, or inspected nested archive members. It checks recognizable formats. The broader Gitleaks history and archive runs are pending. |
| Native Debian and Windows SCM | Pending on CI. The Windows daemon served a live Edge smoke test using the candidate binary. Debian package inspection is static. |

The [release checklist](RELEASE_CHECKLIST.md) describes required promotion evidence. [Audit findings](SECURITY_AUDIT.md)
