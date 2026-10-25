# x-screen-time-limiter-backend

Backend for the multi-device sync feature of [X Screen Time Limiter](https://github.com/shimiwaka/x-screen-time-limiter).

## Overview

A single Go CGI binary that stores usage data as a JSON file and syncs it across multiple devices.

## Files

```
src/         # Go source
Makefile     # build / test
.htaccess    # DirectoryIndex, sync.php -> sync.cgi rewrite, access control
sync.cgi     # Built binary (not tracked by git)
data.json    # Auto-generated data store (not tracked by git)
```

## API

The legacy `sync.php` URLs keep working; `.htaccess` rewrites them to `sync.cgi`.

### GET `/ping` — Health check

```
GET https://example.com/path/to/sync.php/ping
```

Response:
```json
{"pong":true}
```

### POST `/sync` — Sync usage data

```
POST https://example.com/path/to/sync.php/sync
Content-Type: application/json
```

Request body:
```json
{
  "token": "your-secret-token",
  "usage": {
    "2024-01-01": [0, 0, 0, 0, 0, 0, 0, 0, 120, 300, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
  },
  "deletedAt": {
    "2024-01-01": {"3": 1700000000000}
  }
}
```

- `token`: A secret string that identifies the user. Devices sharing the same token share the same data.
- `usage`: Per-date usage in seconds, as a 24-element array (one value per hour).
- `deletedAt`: Per-date, per-hour timestamps of when the hour was deleted.

Response:
```json
{
  "usage": {
    "2024-01-01": [0, 0, 0, 0, 0, 0, 0, 0, 120, 300, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
  },
  "deletedAt": {
    "2024-01-01": {"3": 1700000000000}
  }
}
```

Returns the merged result of the client data and the server data for the given token.

## Build

Requires Go 1.18 (the last version supporting FreeBSD 11).

```bash
go install golang.org/dl/go1.18.10@latest
go1.18.10 download
make build
make test
```

## Setup

1. Upload `sync.cgi` (binary mode) and `.htaccess` to your server
2. `chmod 755 sync.cgi`
3. Make sure the directory is writable by the CGI user (`data.json` is created next to `sync.cgi` with mode `600`)
4. In the extension settings, set the API base URL to `https://example.com/path/to/sync.php`

An existing `data.json` written by the former PHP implementation is read as is.

## Data format

```json
{
  "token-string": {
    "usage": {"YYYY-MM-DD": [seconds, seconds, ...(24 elements)]},
    "deletedAt": {"YYYY-MM-DD": {"hour": timestamp}}
  }
}
```

## Sync strategy

For each hour slot, `deletedAt` is the max of the client and server values, and the usage value is the max among the sides whose `deletedAt` is the newest. Writes to `data.json` are serialized with `flock`.
