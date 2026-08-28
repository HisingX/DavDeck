# Security Policy

DavDeck handles authentication, TLS configuration, and filesystem access. Security reports should be treated carefully.

## Supported versions

DavDeck has not published a supported stable 1.0 release. The current preview
and later release candidates receive security fixes on the current `main`
branch only. There is no stable-version support commitment yet.

## Reporting a vulnerability

Do not publish unpatched exploit details in a public issue.

The public repository administrator must enable GitHub private vulnerability
reporting before public release. Once enabled, use the repository's private
"Report a vulnerability" flow rather than a public issue. Until that channel
is enabled, do not disclose an unpatched exploit in a public issue; contact
the repository maintainer through an existing private channel if one is
available. If none is available, report only that private coordination is
needed, without exploit details.

A useful report should include:

- affected version/commit
- platform
- reproduction steps
- security impact
- minimal proof of concept if safe

## Technical security design

See `docs/SECURITY.md` for the project threat model and implementation baseline.
Current user-facing preview boundaries are tracked in
`docs/KNOWN_LIMITATIONS.md`.
