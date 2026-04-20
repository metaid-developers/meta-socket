# meta-socket P0 Compatibility Matrix

## 1. Purpose

This document defines the P0 compatibility contract for `meta-socket`.
Goal: downstream clients (for example IDBots / IDBots-indev / IDBots_cursor) only change `base_url` and keep existing socket logic unchanged.

## 2. Endpoint Compatibility

- Required: `/<base>/socket/socket.io`
- Recommended fallback: `/<base>/socket.io`
- Transport: Socket.IO over websocket (polling compatibility should not be broken)

Pass criteria:
- Client configured with `path: '/socket/socket.io'` connects successfully.
- Existing clients using `/socket.io` are not broken.

## 3. Handshake Contract

Client query parameters:
- `metaid` (required)
- `type` (`pc` or `app`, optional default behavior remains compatible)

Server compatibility behavior:
- Missing `metaid`: reject connection (same as existing behavior).
- Successful connect: send `message` event with `M='WS_RESPONSE_SUCCESS'`, `C=200`.

## 4. Message Envelope Contract

Server pushes use `message` event with JSON payload string in this envelope:

```json
{
  "M": "<method>",
  "C": 0,
  "D": { }
}
```

Compatibility requirements:
- Keep uppercase keys: `M`, `C`, `D`
- Keep event names unchanged
- Keep payload field names and types backward-compatible

## 5. Required Server Push Events (P0)

- `WS_SERVER_NOTIFY_GROUP_CHAT`
- `WS_SERVER_NOTIFY_PRIVATE_CHAT`
- `WS_SERVER_NOTIFY_GROUP_ROLE`
- `WS_RESPONSE_SUCCESS` (including wrapped payload branch used by existing clients)

## 6. Heartbeat Compatibility

Client behavior in the wild:
- `socket.emit('ping')`

Required compatible response:
- Server replies on `message` event:

```json
{
  "M": "pong",
  "C": 200
}
```

Also keep legacy heartbeat packet support:
- Request `M='HEART_BEAT'`
- Response `M='HEART_BEAT'`, `C=10`

## 7. Connection and Delivery Behavior

- Multi-device tracking by user identity is preserved.
- Device-type limits remain compatible (`pc` and `app` limits).
- Group delivery behavior preserved:
  - room broadcast path (when enabled)
  - fallback direct user push

## 8. Error Code Compatibility

- `WS_CODE_SEND_SUCCESS = 200`
- `WS_CODE_SEND_ERROR = 400`
- `WS_CODE_SERVER = 0`
- `WS_CODE_HEART_BEAT_BACK = 10`

## 9. Contract Test Checklist (Must Pass)

### Connectivity
- Connect with `path='/socket/socket.io'` and query `{metaid,type}`
- Connect fallback `path='/socket.io'`

### Envelope and Event
- All pushes arrive under `message`
- JSON parses to `M/C/D`
- Event names unchanged

### Business payload
- Group chat payload is parseable and complete
- Private chat payload is parseable and complete
- Group role payload is parseable and complete
- `WS_RESPONSE_SUCCESS` wrapper branch is parseable (`D.data` fallback)

### Heartbeat
- `emit('ping')` receives `M='pong'`
- Legacy `HEART_BEAT` request receives compatible ack

## 10. IDBots Acceptance Gate (P0)

Pass condition for each client variant (IDBots / IDBots-indev / IDBots_cursor):
- Only update `base_url`
- Keep existing socket listener and parser code unchanged
- Group/private/role message handling remains functional
- No parser/runtime regressions in `message` event flow

## 11. Explicit P0 Non-Goals

- No full migration of legacy group-chat REST APIs
- No luckybag/grpc feature parity in P0
- No protocol model redesign in P0
