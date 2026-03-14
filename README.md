# x-screen-time-limiter-backend

Backend for the multi-device sync feature of [X Screen Time Limiter](https://github.com/shimiwaka/x-screen-time-limiter).

## Overview

A single-file PHP CGI API that stores usage data as a JSON file and syncs it across multiple devices using a max-per-hour merge strategy.

## Files

```
sync.php     # API entrypoint
.htaccess    # DirectoryIndex and access control
data.json    # Auto-generated data store (not tracked by git)
```

## API

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
  }
}
```

- `token`: A secret string that identifies the user. Devices sharing the same token share the same data.
- `usage`: Per-date usage in seconds, as a 24-element array (one value per hour).

Response:
```json
{
  "usage": {
    "2024-01-01": [0, 0, 0, 0, 0, 0, 0, 0, 120, 300, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
  }
}
```

Returns the merged result of the client data and the server data for the given token.

## Setup

1. Upload `sync.php` and `.htaccess` to your server
2. Make sure the directory is writable by the web server
3. In the extension settings, set the API base URL to `https://example.com/path/to/sync.php`

### Changing the data file location

By default, `data.json` is created in the same directory as `sync.php`. To change this, edit the following line in `sync.php`:

```php
$DATA_FILE = __DIR__ . '/data.json';
```

## Data format

```json
{
  "token-string": {
    "YYYY-MM-DD": [seconds, seconds, ...(24 elements)]
  }
}
```

## Sync strategy

Each hour slot is resolved by taking the maximum of the client and server values. This ensures that existing data is never lost, even after reinstalling the browser extension or syncing after a period offline.
