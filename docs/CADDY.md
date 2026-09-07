# Caddy and WebDAV Integration

## 1. Role of Caddy

Caddy is the managed HTTP/TLS/WebDAV runtime behind DavDeck.

DavDeck users manage product concepts such as users, shares, permissions, TLS mode, and managed runtime state. `davd` translates those concepts into Caddy runtime configuration; native system-service state is a Linux headless concern in the current milestone.

The GUI and CLI must not become generic Caddy configuration editors.

## 2. Runtime distribution

DavDeck should ship or build a pinned custom Caddy binary containing the required WebDAV module.

Release metadata should record:

- DavDeck version
- Caddy version
- caddy-webdav version/commit
- Go version used to build Caddy

Do not pull floating latest versions during release builds.

The source-controlled pins live in `caddy/versions.env`. Build and verify the
runtime with:

```text
make caddy-build
make caddy-verify
make caddy-integration-test
```

The build script invokes the pinned xcaddy release, requests the pinned Caddy
release explicitly, and adds the immutable upstream caddy-webdav version with
DavDeck's in-repository root-confinement security patch. The verification script
rejects a version mismatch or a binary that does not report the
`http.handlers.webdav` module and pinned package version. DNS-01 support is
built into the same binary with pinned Cloudflare, TencentCloud, legacy DNSPod
Token, and AliDNS modules. The DNSPod module is built from the local
`caddy/caddy-dnspod` compatibility patch at the same pinned upstream version.
DNSPod's record-create response can omit the TXT type/value; the patch keeps
the requested ACME record data while using the API-returned record ID, so
CertMagic can verify and clean up the temporary record correctly.

## 3. Module verification

At startup/diagnostics, DavDeck should be able to verify that the selected Caddy binary contains the expected WebDAV module.

The managed binary must also contain DavDeck's `http.handlers.davdeck_index`
module, which serves the authenticated virtual discovery collection. A binary
missing either module must fail inspection with a specific actionable error.

If the module is missing, return a specific actionable error rather than allowing runtime failure later.

## 4. Configuration format

Use generated Caddy JSON as the runtime artifact.

Reasons:

- deterministic machine generation
- avoids parsing/re-writing user Caddyfile syntax
- maps naturally to Admin API loading
- easier structural tests

Caddyfile may be shown/exported later only as an optional diagnostic/advanced representation if there is a clear need.

## 5. Compiler boundary

The Caddy compiler accepts a canonical domain snapshot and produces deterministic JSON.

When TLS is enabled, it must also emit Caddy's `http_port` and `https_port`
settings from DavDeck's server settings. This makes automatic HTTP-to-HTTPS
redirects use the managed ports instead of Caddy's default ports 80 and 443.

It must not:

- query SQLite directly
- read GUI state
- interpret CLI flags
- perform network calls
- mutate application state

Deterministic ordering is required for meaningful hashes/revisions/golden tests.

## 6. Share routing model

MVP gives each Share a distinct path, for example:

```text
/dav/photos/
/dav/documents/
/dav/backup/
```

Each path is routed to the corresponding physical root and authorization policy.

DavDeck also exposes an authenticated virtual discovery collection at the
configured public base path (for example, `/dav/`). Its contents are generated
from the authenticated user's enabled READ/READ_WRITE Share permissions. The
collection only links to Share routes; it is not a merged filesystem and does
not perform file operations itself.

A future virtual filesystem layer is outside MVP.

## 7. Authentication and ACL compilation

The compiler must translate enabled users and per-share permissions into a Caddy/WebDAV configuration that enforces:

- disabled users denied
- no-ACL users denied
- READ users read/discovery only
- READ_WRITE users normal read/write WebDAV access

Exact HTTP/WebDAV method behavior must be verified in integration tests against the actual pinned runtime.

Do not assume that a config-looking-correct unit test proves runtime authorization.

## 8. Validation

Before automatic or explicit Apply:

1. serialize generated config
2. run supported Caddy validation/provisioning check
3. reject invalid config
4. only then attempt runtime load/reload

Validation errors should be captured and translated into safe structured diagnostics while preserving enough detail for troubleshooting.

## 9. Admin API

Only backend Caddy integration code may access the Admin API.

Requirements:

- local-only
- never exposed through GUI directly
- never exposed through CLI directly
- never bound to public interfaces as a convenience feature

## 10. Runtime lifecycle

The runtime manager should support:

- start
- stop
- restart
- status
- validate
- reload/load
- version/module inspection
- health check
- log capture/integration

`davd` must distinguish process state from service health. A live Caddy process with a broken WebDAV endpoint is `DEGRADED`, not fully healthy.

## 11. TLS modes

### Automatic

Compiler creates the managed site configuration for the requested public hostname and delegates certificate lifecycle to Caddy.

`challenge: auto` uses Caddy's normal ACME challenge selection. `challenge: dns`
selects the configured DNS provider module for ACME DNS-01 validation. Provider
credentials are resolved by `davd` and injected into the Caddy process
environment only at validation/start time; generated JSON and revisions contain
environment placeholders and public provider metadata, never credential values.
Changing a DNS credential requires an explicit Apply and may restart Caddy so
the new process environment takes effect.

Caddy stores managed ACME data in its local application-data directory: macOS
uses `~/Library/Application Support/Caddy`, Windows uses `%AppData%/Caddy`, and
Linux uses `~/.local/share/caddy` unless `XDG_DATA_HOME` overrides the base
directory. The desktop HTTPS page displays the effective directory and the
public certificate file path. Certificate acquisition is asynchronous: a
successful Apply only means the runtime accepted the configuration, not that the
ACME certificate has already been issued. DavDeck reports the phase as waiting,
issuing, ready, expired, or failed. Private keys remain managed by Caddy and are
never displayed by this status view.

For a one-shot renewal, `davd` calls a loopback-only route registered by the
DavDeck Caddy build. The route invokes the active TLS automation policy's
CertMagic `RenewCertSync(..., force=true)` path and reloads the managed
certificate into CertMagic's cache, replacing older same-subject entries so
new TLS handshakes use the renewed certificate immediately. The operation
reuses the saved challenge
and DNS provider; it does not rewrite the active Caddy JSON or perform a local
TLS handshake. `davd` polls the sanitized operation status and the public
certificate file, reports success/failure, and can cancel an active operation.
The persisted generated config, TLS profile, and existing certificate/private-key
material are preserved. The Caddy binary must be built from the pinned Caddy
version with `patches/caddy-v2.11.4-force-renewal.patch` and the pinned
`caddy/caddy-renewal` admin module.

### Internal

Compiler selects Caddy's internal PKI mode for local/LAN use and explicitly disables
Caddy's automatic system trust-store installation. Product UX must explain client
trust implications and provide the appropriate CA export or setup guidance.

### Custom

Compiler references user-provided certificate and key paths. Do not include private-key contents in generated diagnostic output.

The compiler emits only certificate and private-key file references. The TLS
preflight verifies that both files are readable and form a matching key pair
before the operator applies the desired configuration.

## 12. Config revisions

Store enough metadata to reproduce/debug runtime changes:

- revision id
- timestamp
- generated JSON or canonical representation
- hash
- validation outcome
- apply outcome
- application version
- active/desired marker

Revision creation is idempotent for an identical generated configuration hash.
Starting, stopping, or restarting the managed Caddy process is a runtime
operation and must not create a new configuration revision. Validation failures
should be surfaced as apply errors rather than being presented as restorable
configuration versions.

Sensitive values should not be introduced into revision data unnecessarily.

## 13. Upgrade policy

Caddy or caddy-webdav upgrade should be a dedicated change.

Required validation:

- compiler golden tests
- Caddy validation tests
- authentication tests
- READ/READ_WRITE ACL integration tests
- TLS smoke tests where feasible
- diagnostics module detection

## 14. Future custom module possibility

A future DavDeck-specific Caddy module may be considered for virtual filesystem mapping or more advanced ACLs. It is not part of MVP and must not be implemented speculatively.
