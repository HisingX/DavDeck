# Known Limitations

This page describes current product limitations and release trust boundaries. It
is intentionally separate from internal acceptance records so users can see the
product boundary without reading development workflow notes.

## Release trust

- Current release archives are not code-signed or notarized.
- The release process provides SHA-256 checksums, but users must obtain the
  archive and checksum through a trusted channel and verify them locally.
- The initial `1.0.0` release is intentionally planned as an unsigned archive;
  it should not be used as the only copy of important data.

## Platform coverage

- macOS ARM64 GUI behavior has received the primary desktop smoke validation.
  Gatekeeper may show an unsigned-app warning.
- Windows x64 GUI and ACL validation is complete for the current target.
  Windows-specific installer, signing, and reparse-point/junction validation
  are not release-complete.
- Linux x64 has separate Server and Desktop release flavors. Linux ARM64 is
  Server-only and does not currently have a desktop GUI target.

## Service lifecycle

The current milestone supports native system-service installation for Linux
Server archives through `install.sh` and systemd. Desktop GUI
service installation is deferred on Windows and macOS. Their GUI runs in
portable mode: closing the window keeps the process in the tray or menu bar,
while the explicit Exit menu stops it. Linux service boot persistence still
requires validation on the target host's systemd configuration.

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

## Distribution work outside the current release scope

The following are future release work rather than hidden product guarantees:

- macOS notarization and signed distribution;
- Windows code signing and polished installer/update flows;
- Linux distribution packages such as deb/rpm and third-party package-manager
  formulas;
- automatic update delivery;
- DNS provider ACME integrations;
- complete GUI validation on every desktop target.
