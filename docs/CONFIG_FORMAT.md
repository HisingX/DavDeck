# Configuration Import/Export Format

## 1. Purpose

YAML is an import/export and automation format. SQLite remains the authoritative runtime desired-state store.

## 2. Versioning

Every file must include a schema version.

Example:

```yaml
version: 1
```

Unsupported versions must fail with `CONFIG_VERSION_UNSUPPORTED` rather than being partially interpreted.

## 3. Example

```yaml
version: 1

server:
  public_base_path: /dav
  http_port: 80
  https_port: 443
  runtime_mode: service

tls:
  mode: automatic
  hostname: dav.example.com
  challenge: dns
  dns_provider: Cloudflare production

users:
  - username: alice
    enabled: true
  - username: bob
    enabled: true

shares:
  - name: Photos
    slug: photos
    path: /srv/photos
    permissions:
      alice: read_write
      bob: read
```

`runtime_mode: service` is intended for Linux headless systemd deployments in
the current milestone. Desktop GUI builds use portable runtime ownership.

## 4. Passwords

Default export contains no plaintext passwords and no password hashes.

Import behavior for users without a password must be explicit. Recommended MVP options:

- create/update non-secret user metadata and require password to be set separately, or
- allow a separate secure secret-input mechanism outside the YAML file

Do not encourage committing password hashes/secrets to Git repositories.

## 5. Secret fields

Default YAML must not contain:

- management token
- TLS private-key content
- DNS provider tokens
- raw sensitive credentials

Custom certificate paths may be exported if the user chooses path-based configuration; this is not equivalent to exporting private key content.

## 6. Import semantics

Before modifying SQLite:

1. parse strictly
2. validate version
3. validate all entities
4. detect duplicates/conflicts
5. validate paths syntactically
6. present/return a plan or validation result
7. apply transactionally

Import should not automatically delete unspecified resources unless an explicit replace mode is designed.

Recommended initial mode: merge/upsert with clear conflict rules, or a simpler “replace desired configuration” mode guarded by explicit confirmation. Choose one and document it before implementation.

DavDeck v1 uses merge/upsert semantics:

- users match by normalized username
- shares match by slug
- listed permissions are upserted; omitted permissions are preserved
- omitted users, shares, permissions, and TLS configuration are preserved
- no import operation deletes physical files
- existing password hashes are preserved
- new users receive an unguessable generated password hash and must have their
  password set separately through authenticated user management

Import validates the complete document and enabled share paths before opening a
write transaction. A failure applies no partial changes. Import changes desired
state only; explicit Apply remains required even though ordinary user, share,
and ACL mutations apply automatically.

## 7. Stable values

Use stable lowercase YAML enum values such as:

```text
none
read
read_write
```

TLS:

```text
automatic
internal
custom
```

Automatic TLS may use `challenge: auto` (the default) or `challenge: dns`.
For a DNS challenge, `dns_provider` is the local provider credential name.
The credential itself must already exist on the importing machine; provider
secrets are never included in YAML exports and are never imported from YAML.

Map them explicitly to internal enum values.

## 8. Export determinism

Export should use deterministic ordering where practical to keep Git diffs readable.
