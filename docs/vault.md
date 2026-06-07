---
layout: default
title: Vault (Secrets)
nav_order: 8
---

# runtime-vault

Secrets and configuration explorer. Vault inspects, validates, and decodes secret artifacts, showing only non-sensitive metadata — secret values are always masked.

> **Security note:** Vault treats every artifact as sensitive. Inspectors parse, validate, and decode secrets but never log or echo a secret value. Results reference secrets by key name and expose only non-sensitive metadata (key names, value lengths, certificate subjects, JWT claim keys, detected formats). Any value that could be sensitive is rendered masked (e.g. `•••• (24 chars)`).

## Supported inputs

Vault classifies an artifact from its content (and filename hints), then dispatches to the matching inspector.

| Kind | What it inspects | Detection |
|------|------------------|-----------|
| Env file | dotenv `KEY=VALUE` files | `.env` filename or multi-line `KEY=VALUE` content |
| JWT token | `header.payload.signature` JSON Web Tokens | three base64url segments on one line |
| Certificate | PEM-encoded X.509 certificates | `-----BEGIN CERTIFICATE-----` header |
| API key | single-line API keys / tokens | a single token-like line |
| Kubernetes secret | Secret manifests in **JSON or YAML** | `kind: Secret` / `data`/`stringData` alongside `apiVersion`+`metadata` |

YAML and Kubernetes manifests are parsed with the standard library plus YAML support; classification is content-based, so manifests are routed correctly regardless of extension.

## Launch

```sh
runtime-vault [file]
```

The first argument, if present, is loaded and inspected immediately. Examples:

```sh
runtime-vault examples/vault/sample.env
runtime-vault examples/vault/token.jwt
runtime-vault examples/vault/cert.pem
runtime-vault examples/vault/apikey.txt
runtime-vault examples/vault/apikeys.txt
runtime-vault examples/vault/secret.json
runtime-vault examples/vault/secret.yaml
```

## Keymap

Universal bindings (see the [main keymap](../README.md#universal-keymap)):

| Key | Action |
|-----|--------|
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

Vault renders a single inspection result, so it uses the universal help/quit bindings; the navigation keys are reserved by the shared keymap for consistency across the ecosystem.

## Features

- Automatic artifact classification from content
- Masked metadata display — never reveals secret values, only their shape (lengths, key names, claim keys, certificate subjects)
- Validation with issues surfaced as warnings; a clean artifact shows a `✓ valid` indicator

## Configuration

Vault loads its config on startup from the standard per-user location:

- Linux/macOS: `$XDG_CONFIG_HOME/runtime/vault/config.json` (fallback `~/.config/runtime/vault/config.json`)
- Windows: `%APPDATA%\runtime\vault\config.json`

The `theme` field selects the palette. A missing config is fine; a malformed config never prevents launch. See [docs/configuration.md](configuration.md).

## Limitations

- Inspection is read-only: Vault reports on artifacts but does not edit, rotate, or re-sign them.
- Classification uses lightweight heuristics; an unrecognised artifact is reported as such rather than guessed.
