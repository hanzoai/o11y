# Security Policy

## Reporting a vulnerability

Email security@hanzo.ai with details. Encrypt with our PGP key (fingerprint TBD).

We respond within 48 hours. Critical issues receive same-day acknowledgment.

## Scope

This policy covers code in this repository. For the broader Hanzo platform threat model, see [hanzoai/HIPs](https://github.com/hanzoai/HIPs).

## Sandbox boundary

`o11y` ingests telemetry from every Hanzo service, so every query path is scoped by the JWT-validated `X-Org-Id` header — no panel, alert, or query can cross an org boundary. Logs, traces, and metrics are stored on Hanzo Datastore with per-tenant retention, and sensitive headers / payload fields are redacted at ingest time.

For runtime sandbox guarantees, see HIP-0105 (in-process extension runtimes).
