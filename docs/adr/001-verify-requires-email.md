# ADR-001: Auth endpoint deviations from OpenAPI spec

## Status

Accepted

## Context

The OpenAPI spec defines three auth endpoints. Two deliberate deviations were made during implementation.

### Deviation 1: Verify requires email

The spec defines `POST /api/v1/auth/verify` with request `{otp}`. However, OTPs are stored in Valkey keyed by email (`otp:{email}`), so we need the email to look up the stored OTP.

Alternatives considered:

1. **Embed user_id in the OTP key** — store `otp:{code}` → `{user_id, email}` so verification is OTP-only. This couples the OTP to a specific user ID and requires a reverse lookup.
2. **Require email in the verify request** — the frontend already knows the email from registration. Simple, no additional storage or lookup complexity.

### Deviation 2: Register does not return access_token

The spec defines the register response as `{message, data: {access_token}}`. Our design intentionally omits the token: registration creates an unverified user, and the user must verify their email via OTP before they can log in. Returning a token on register would allow unverified access, defeating the purpose of email verification.

## Decision

1. Require `email` in the verify request body alongside `otp`.
2. Register returns only `{success, message}` — no `access_token`. User must complete verify → login flow.

## Consequences

- The API contract deviates from the OpenAPI spec in two places. The spec should be updated.
- Frontend must send the email address they registered with during verify. This is the email they just typed, so it's readily available.
- OTP lookup remains a simple `GET otp:{email}` in Valkey — no reverse index needed.
- The register→verify→login flow is explicit and secure: no token is issued until the user proves ownership of their email.
