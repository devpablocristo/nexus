# API Versioning & Deprecation Policy

## Versioning scheme

All API endpoints are prefixed with a version: `/v1/tools`, `/v1/run`, etc.

### Rules

- **Major version** (`/v1/`, `/v2/`): Breaking changes - new version prefix.
- **Minor additions**: New fields, new endpoints - backwards compatible, same version.
- **No removal without deprecation**: Fields and endpoints are never removed without notice.

## Deprecation process

1. **Announce**: Deprecated items get `X-Nexus-Deprecated: true` header + field in OpenAPI spec.
2. **Grace period**: Minimum **6 months** from announcement before removal.
3. **Sunset header**: Responses include `Sunset: <date>` header per RFC 8594.
4. **Communication**: Email to all org admins + changelog entry + developer portal notice.
5. **Removal**: After sunset date, endpoint returns `410 Gone`.

## Current API versions

| Service | Current | Status |
|---------|---------|--------|
| nexus-core | v1 | Stable |
| nexus-saas | v1 | Stable |

## SDK compatibility

SDKs (Go, TypeScript, Python) target the latest stable API version.
Older SDK versions continue to work until the API version is sunset.
