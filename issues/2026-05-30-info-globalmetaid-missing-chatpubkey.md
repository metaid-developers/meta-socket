# Issue: `/api/info/globalmetaid/:globalMetaId` does not return chatPubkey

## Context

Bothub Delivery needs a provider chat public key to decrypt private simplemsg
messages and to send follow-up messages. The bot-hub skill-service list/detail
responses now include provider chat keys, which unblocks newly created orders
when Bothub has already cached the service/order metadata.

However, Delivery also has profile fallback flows for older sessions and manual
provider-key recovery. Those flows call:

```text
GET /api/info/globalmetaid/:globalMetaId
```

For the tested provider, this endpoint currently returns no chat public key.

## Reproduction

Tested locally on 2026-05-30 against:

```text
http://127.0.0.1:18091/api/info/globalmetaid/12FxJzsxhQ5snAieJ5MPo9x9bhAZ2e3ejc
```

Observed response:

```json
{
  "code": 1,
  "data": {
    "globalMetaId": "12FxJzsxhQ5snAieJ5MPo9x9bhAZ2e3ejc",
    "metaid": "",
    "address": "",
    "avatar": "/content/"
  },
  "message": "",
  "processingTime": 1780108332984
}
```

For the same provider, skill-service detail does include the key:

```text
GET /api/bot-hub/skill-service/detail/f6b810a50cf532363fd643ef10b429f809c52670fa16fb22ecb7906485900c1ai0?chainName=mvc
```

Observed provider field:

```json
{
  "globalMetaId": "12FxJzsxhQ5snAieJ5MPo9x9bhAZ2e3ejc",
  "name": "Dan Mercier",
  "chatPubkey": "04e3bf8fef4260ce6b82136de459b23c01676a89767fb6dacf073a1c8bc460ec83115b437dbdf559e47f745ad856f94ef2ea512fe3238030176f7b011242e87c81"
}
```

## Expected

`/api/info/globalmetaid/:globalMetaId` should return the latest chat public key
for the requested identity, using one or more stable field names already
recognized by consumers:

```json
{
  "code": 1,
  "data": {
    "globalMetaId": "12FxJzsxhQ5snAieJ5MPo9x9bhAZ2e3ejc",
    "name": "Dan Mercier",
    "avatar": "https://...",
    "chatPubkey": "04..."
  }
}
```

Accepted aliases for compatibility:

- `chatPubkey`
- `chatpubkey`
- `chatPublicKey`
- `chat_pubkey`
- `chat_public_key`
- `pubkey`

## Why This Matters

Bothub can now use cached order/session keys for newly created orders, but old
Delivery sessions or sessions discovered only from private-chat history may not
have local order metadata. Without a profile-level chat key fallback, those
sessions remain encrypted in the UI and the user cannot reliably send follow-up
messages.

## Impact

- Old/private-chat-only Delivery sessions display ciphertext such as
  `U2FsdGVk...`.
- Manual provider-key fetch in the Delivery composer cannot recover a missing
  key.
- Frontends must special-case bot-hub service metadata instead of relying on the
  profile API as a general identity lookup.
