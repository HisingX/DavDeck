# Known Limitations

This page describes the limitations of the `0.1.0-rc.1` preview. It is
intentionally separate from the internal acceptance records so users can see
the current product boundary without reading development workflow notes.

## Release trust

- Preview archives are not code-signed or notarized.
- The release process provides SHA-256 checksums, but users must obtain the
  archive and checksum through a trusted channel and verify them locally.
- The preview is not a stable `1.0` release and should not be used as the only
  copy of important data.

## Platform coverage

- macOS ARM64 GUI behavior has received the primary desktop smoke validation.
  Gatekeeper may show an unsigned-app warning.
- Windows x64 is a build target, but Windows GUI validation remains deferred.
  Windows-specific installer, signing, and reparse-point/junction validation
  are not release-complete.
- Linux x64 and ARM64 are supported as headless targets. Native service
  status/install smoke has been checked without changing the host's installed
  service state.
- Linux ARM64 does not currently have a desktop GUI target in the release
  workflow.

## Service lifecycle

The daemon and native service adapters are implemented, but this preview does
not claim complete end-to-end service installation coverage on every operating
system. In particular, reboot and boot-persistence validation was intentionally
not performed. Treat service installation as an administrative operation and
verify it on the target host before relying on it in production.

## HTTPS and certificates

- Internal HTTPS and custom certificate workflows are available.
- Public ACME issuance is delegated to Caddy and requires a hostname and a
  reachable challenge path. HTTP-01 normally needs inbound port 80; DNS-01
  requires a supported DNS provider integration and credentials.
- DavDeck does not currently provide Cloudflare, DNSPod, AliDNS, or other DNS
  provider credential integration. Local-only deployments should use internal
  certificates, custom certificates, or an externally managed reverse proxy.
- Certificate trust is the client's responsibility. Internal or self-signed
  certificates must be installed or explicitly trusted by each client.

## WebDAV and filesystem safety

- Files are preserved when users, shares, application metadata, or services are
  removed; metadata removal is not a physical data deletion operation.
- The authenticated `/dav/` root is a discovery-only collection. It lists the
  current user's enabled shares, while each `/dav/<slug>/` path remains an
  independent WebDAV filesystem and ACL boundary.
- Cross-share `MOVE` and `COPY` operations are not supported through the
  discovery root.
- Unix/macOS confinement is covered by the pinned runtime integration tests.
- Native Windows junction and reparse-point behavior still requires validation
  on a Windows host before a stable `1.0` security claim.

## Management and automation

- The Management API is local-only and is not a remote administration API.
- `davctl logs --follow` is intentionally unsupported until a safe streaming
  contract is available. Use bounded log queries instead.
- Configuration import changes desired state; run `davctl config apply` after
  reviewing an import.
- GitHub private vulnerability reporting must be enabled by the public
  repository administrator before regular security disclosures begin.

## Distribution work not included in this preview

The following are future release work rather than hidden guarantees of this
preview:

- macOS notarization and signed distribution;
- Windows code signing and polished installer/update flows;
- Linux distribution packages such as deb/rpm and third-party package-manager
  formulas;
- automatic update delivery;
- DNS provider ACME integrations;
- complete GUI validation on every desktop target.
