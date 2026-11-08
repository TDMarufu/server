# Manual End-to-End Testing Guide

How to verify the whole system (Android app, server, admin dashboard) is actually working after a deploy.

## 1. The Android app (client)

- Install the app with **Cloud Server** mode toggled on in Settings.
- It auto-registers against `sms.territechnologies.com` and shows:
  - Settings → Cloud Server: the device's username/password (per-device Basic auth credentials)
  - Settings → Local Server → **API Tokens**: for testing Local Server mode separately on the LAN

## 2. The admin dashboard

- Go to `https://admin.territechnologies.com`
- Log in with the device username/password from step 1 (or any existing device's credentials)
- Should show: Devices, Send a test message, Recent Messages, Issue an Integrator Token, and **Issued Tokens** (list + revoke)

## 3. Issue a scoped token (for API testing without the dashboard)

```sh
curl -s -u "<device-username>:<device-password>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"scopes":["messages:send","messages:read","status-webhooks:write"],"ttl":86400}'
```

Save the `access_token` from the response.

## 4. Verify the token-list endpoint

```sh
curl -s -H "Authorization: Bearer <access_token>" \
  https://sms.territechnologies.com/api/3rdparty/v1/auth/token
```

Should list the token just issued (scopes, created, expires) — never the raw token string again.

## 5. Status webhooks

Register one against a public test endpoint (e.g. `https://<slug>.requestcatcher.com/test` — no setup needed, view logged requests by opening that same URL in a browser):

```sh
curl -s -H "Authorization: Bearer <access_token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/status-webhooks \
  -H "Content-Type: application/json" \
  -d '{"url":"https://<slug>.requestcatcher.com/test","events":["message.sent","message.delivered"]}'
```

Save the returned `secret`. Send a real SMS through the device, then check the requestcatcher URL in a browser — expect a logged POST with an `X-Signature-256: sha256=<hex>` header. Verify it's `HMAC-SHA256(secret, raw_request_body)`.

**SSRF rejection** — this must fail with `400`, not `201`:

```sh
curl -s -w "\n%{http_code}\n" -H "Authorization: Bearer <access_token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/status-webhooks \
  -H "Content-Type: application/json" \
  -d '{"url":"https://127.0.0.1/evil","events":["message.sent"]}'
```

Also try a private LAN IP and a non-`https` URL — both should be rejected the same way.

List / clean up:
```sh
curl -s -H "Authorization: Bearer <access_token>" \
  https://sms.territechnologies.com/api/3rdparty/v1/status-webhooks

curl -s -H "Authorization: Bearer <access_token>" -X DELETE \
  https://sms.territechnologies.com/api/3rdparty/v1/status-webhooks/<id>
```

## 6. MMS via the external-integrator API

```sh
curl -s -H "Authorization: Bearer <access_token>" \
  -X POST https://sms.territechnologies.com/api/3rdparty/v1/message \
  -H "Content-Type: application/json" \
  -d '{"mmsMessage":{"subject":"Hello","attachments":[{"contentType":"image/jpeg","data":"<base64>"}]},"phoneNumbers":["+263XXXXXXXXX"]}'
```

## 7. Health check (no auth)

```sh
curl -s https://sms.territechnologies.com/health
```

Expect `{"status":"pass","version":"<current>",...}` with `checks.db:ping.status: "pass"`.

## 8. Revoke and confirm it's dead

```sh
curl -s -u "<device-username>:<device-password>" -X DELETE \
  https://sms.territechnologies.com/api/3rdparty/v1/auth/token/<jti>

curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer <access_token>" \
  https://sms.territechnologies.com/api/3rdparty/v1/auth/token
```
Second call should now return `401`.
