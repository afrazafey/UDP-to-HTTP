# Release checklist

- [ ] Native Linux AMD64 and ARM64 test, race, multicast, playback, install, and upgrade gates pass on the candidate bytes.
- [ ] Native Windows AMD64 tests, service lifecycle and ACL checks, file-version inspection, and load tests pass.
- [ ] Scan source, dependencies, and release archives. Review findings and resolve all critical and high severity items before stable promotion.
- [ ] Compare baseline and candidate load at 1, 10, 25, 50, and 100 viewers. Verify packet integrity, receiver sharing, slow-client handling, and recovery limits.
- [ ] Review package checksums, Go and frontend SBOMs, linked version, commit and UTC timestamp, and platform evidence.
- [ ] Confirm package distribution and signing requirements and obtain approval before publishing or deployment.

The Phase 4.2 artifacts are release candidates. A production deployment label describes the deployment environment. It is independent of the acceptance of a build.
