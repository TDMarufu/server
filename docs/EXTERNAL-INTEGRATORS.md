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
- optionally, register a status webhook (`status-webhooks:write`) so you don't
  have to poll — see below

It cannot register/remove devices, change settings, read incoming SMS, or
manage other tokens. If you need different access, ask us — we issue a new
token with exactly the scopes you need.

## Subscription required

Sending messages (`messages:send`) requires an active $25/month subscription
on the account the token belongs to, paid via Pesepay. If it's lapsed or never
started, you'll get:

```json
{"message": "subscription required - POST /3rdparty/v1/billing/subscribe to pay"}
```
with HTTP status `402 Payment Required`. See `docs/BILLING.md` for how to pay.

## Using the token

```bash
curl -s -H "Authorization: Bearer <your-token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/message \
  -H "Content-Type: application/json" \
  -d '{"textMessage":{"text":"Hello"},"phoneNumbers":["+263XXXXXXXXX"]}'
```

(The flat `message` field shown in older examples still works but is deprecated — use `textMessage` going forward.)

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

### Sending MMS

Same endpoint, `mmsMessage` instead of `textMessage`, with one or more Base64-encoded attachments:

```bash
curl -s -H "Authorization: Bearer <your-token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/message \
  -H "Content-Type: application/json" \
  -d '{"mmsMessage":{"subject":"Hello","attachments":[{"contentType":"image/jpeg","data":"<base64-encoded-bytes>"}]},"phoneNumbers":["+263XXXXXXXXX"]}'
```

Requires the same `messages:send` scope — no separate MMS scope exists.

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

## Getting notified instead of polling

If you'd rather not poll `/message/<id>` repeatedly, register a status webhook
(needs the `status-webhooks:write` scope — ask us to include it):

```bash
curl -s -H "Authorization: Bearer <your-token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/status-webhooks \
  -H "Content-Type: application/json" \
  -d '{"url":"https://your-app.example.com/sms-status","events":["message.sent","message.delivered","message.failed"]}'
```

Response (the `secret` is only ever shown here, at creation/update — it's never
returned again):
```json
{"id":"...","url":"https://your-app.example.com/sms-status","events":["message.sent","message.delivered","message.failed"],"secret":"...","disabled":false}
```

On each matching event, we `POST` to your URL:
```json
{"event":"message.sent","messageId":"dKXOTwhzNLbY_yEwu4oMM","deviceId":"ZK3TUG2GzrgoPX4iM6E-e","state":"Sent","timestamp":"2026-11-04T12:00:00Z"}
```
with a signature header you should verify:
```
X-Signature-256: sha256=<hex-encoded HMAC-SHA256 of the raw request body, using your secret as the key>
```
Reject the request if the signature doesn't match — that's how you know it
genuinely came from us and wasn't forged.

Your URL must be `https://`. We retry a failed delivery once; after 20
consecutive failed events the webhook is automatically disabled (re-register
it to reactivate once your endpoint is fixed). List your webhooks with
`GET /status-webhooks`, remove one with `DELETE /status-webhooks/<id>`.

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
