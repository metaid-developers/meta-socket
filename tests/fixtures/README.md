# Socket Contract Fixtures

This folder contains minimal JSON fixtures for `meta-socket` compatibility tests.

## Files

- `group_chat_event.json`
  - `M=WS_SERVER_NOTIFY_GROUP_CHAT`
  - canonical group chat payload in `D`
- `private_chat_event.json`
  - `M=WS_SERVER_NOTIFY_PRIVATE_CHAT`
  - canonical private chat payload in `D`
- `group_role_event.json`
  - `M=WS_SERVER_NOTIFY_GROUP_ROLE`
  - canonical group role payload in `D`
- `response_success_wrapped_group.json`
  - `M=WS_RESPONSE_SUCCESS`
  - wrapped branch where `D.data` carries group payload (legacy client compatibility)
- `connection_success.json`
  - connect ack payload shape
- `pong_message.json`
  - ping/pong compatibility payload shape

## Usage

1. Emit fixture payload strings through `message` event in integration tests.
2. Validate parser behavior for `M/C/D` envelope.
3. Validate that legacy `WS_RESPONSE_SUCCESS -> D.data` fallback branch works.
4. Keep field names stable to avoid downstream breakage.

## Rules

- Preserve event names and envelope keys exactly (`M`, `C`, `D`).
- Add new fixture files instead of mutating existing ones when introducing non-breaking variants.
