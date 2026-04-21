# Logging API Specs

## Overview

Logging is optional and client-scoped. This spec defines the `Logger` contract, the package log levels, and the guarantees of the default logger.

## Logger Contract

A valid logger implements all of:

- `Debugf`
- `Infof`
- `Warnf`
- `Errorf`
- `SetLevel`

The package log levels are:

- `LevelDebug`
- `LevelInfo`
- `LevelWarn`
- `LevelError`

## Default Logger

`NewDefaultLogger(output io.Writer, level Level)` creates a package logger backed by `log/slog`.

The level argument MUST be a `requests.Level`, not a `slog.Level`.

`SetLevel` changes the active threshold after construction.

> **Why**: The package exposes its own small logging contract so callers can swap implementations without depending on a specific logging framework.
>
> **Rejected**: Exposing `slog.Logger` directly as the public logging interface.

## Operational Guarantees

- Logging is opt-in. If no logger is configured, the package performs no logging.
- Log messages are operational diagnostics, not a stable parsing interface.
- File-loading helpers and retry paths may log failures or retry events when a logger is configured.

## Forbidden

- Do not pass `slog.Level*` values to `NewDefaultLogger`; use `requests.Level*`.
- Do not implement only part of the `Logger` interface.
- Do not parse package log messages as a stable machine-readable API.

## Acceptance Criteria

- [ ] The required logger methods are explicit.
- [ ] The package level enum is explicit.
- [ ] The boundary between the public logger contract and `slog` internals is explicit.

**Origin:** Migrated from `docs/logging.md`.
