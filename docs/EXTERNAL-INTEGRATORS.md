# API Access for External Integrators

This is for anyone outside our team who needs to call our SMS Gateway API
(`sms.territechnologies.com`) from their own application. It only covers
sending and checking messages — we do not hand out raw device credentials or
device-management access to external parties.

## What you get

A **scoped Bearer token**, not a username/password. It can only:
- send messages (`messages:send`)
- read a message's status (`messages:read`)
- list sent messages (`messages:list`)

It cannot register/remove devices, change settings, read incoming SMS, or
manage other tokens. If you need different access, ask us — we issue a new
token with exactly the scopes you need.

## Using the token

```bash
curl -s -H "Authorization: Bearer <your-token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/message \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello","phoneNumbers":["+263XXXXXXXXX"]}'
```

Response:
```json
{
  "id": "dKXOTwhzNLbY_yEwu4oMM",
  "deviceId": "ZK3TUG2GzrgoPX4iM6E-e",
  "state": "Pending",
  "recipients": [{"phoneNumber": "+263XXXXXXXXX", "state": "Pending"}],
  "textMessage": {"text": "Hello"}
}
```

Check status with the returned `id`:
```bash
curl -s -H "Authorization: Bearer <your-token>" \
  https://sms.territechnologies.com/api/3rdparty/v1/message/<id>
```
`state` moves `Pending` → `Processed` → `Sent` → `Delivered` (or `Failed`).
Typically under a second if the phone is online.

## Token lifetime

- Your access token is long-lived (issued for up to 90 days) so you don't need
  to re-authenticate on every call.
- You'll also receive a **refresh token**, valid longer than the access token.
  Before your access token expires, exchange the refresh token for a new pair:
  ```bash
  curl -s -H "Authorization: Bearer <your-refresh-token>" \
    -X POST https://sms.territechnologies.com/api/3rdparty/v1/auth/token/refresh
  ```
- If your access token expires and you don't have a valid refresh token, ask
  us to issue a new pair.

## If your token is compromised

Tell us immediately — we revoke it server-side by its token ID (`jti`, part of
what we issue you) within minutes. Revocation is instant and doesn't require
rotating anything else on our side.

## Rate limits and fair use

- This runs on a single modest server for a handful of phones — avoid sending
  in tight bulk loops. Space out high-volume sends with us ahead of time.
- Phone numbers must be in full international format (e.g. `+263...`).

## Support

Contact whoever issued you this token for help, new scopes, or a new phone
number to send through.
