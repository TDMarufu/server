# Billing (Pesepay subscription)

Flat $25/month subscription, paid via Pesepay, gating the external-integrator
`messages:send` endpoint. Billing attaches to the **account** (the same
`userID` that owns tokens/webhooks), not to individual tokens - every token
issued under an account shares its subscription.

## Flow

1. Client calls `POST /3rdparty/v1/billing/subscribe` (authenticated as their
   account). We call Pesepay's `initiate` API and return `redirectUrl`.
2. Client is redirected to Pesepay's hosted checkout, pays.
3. Pesepay POSTs to our configured `BILLING__RESULT_URL`
   (`POST /3rdparty/v1/billing/pesepay/callback`, public - no auth, since
   Pesepay can't send our Basic auth/JWT).
4. **We never trust that callback body.** Pesepay's own docs state it's
   unsigned and unencrypted. On every callback we call Pesepay's
   `check-payment` API ourselves with the `referenceNumber` and only act on
   *that* response.
5. If actually paid, we extend `subscriptions.current_period_end` by one month
   from `max(now, existing_period_end)` - renewing early doesn't lose unused
   time.
6. `messages:send` checks `current_period_end > now()` on every request
   (`billing.Service.IsActive`) and returns `402 Payment Required` if lapsed.
   This is purely date-based - there's no separate "active" status flag to
   drift out of sync with the actual date.

## Pesepay API details (not fully documented on their site - confirmed empirically against sandbox)

- Sandbox base URL: `https://api.test.sandbox.pesepay.com/payments-engine`
- Production base URL: `https://api.pesepay.com/api/payments-engine`
- Integration key goes in a `key` HTTP header.
- Request/response bodies are wrapped in an encrypted `payload` field:
  AES-256-CBC, key = the encryption key used as raw UTF-8 bytes (a 32-character
  key is exactly 32 bytes = AES-256), IV = first 16 bytes of that same key,
  PKCS7 padding, base64-encoded ciphertext.
- `POST /v1/payments/initiate` → `{referenceNumber, redirectUrl}`
- `GET /v1/payments/check-payment?referenceNumber=...` → includes
  `transactionStatus`; **`"SUCCESS"` means paid** (not `"PAID"` - confirmed
  against the reference PHP SDK's source, since Pesepay's docs don't state
  this explicitly either).

## Config

Set in `.env`/`config.yml` on the host (never commit real keys to the repo):

```yaml
billing:
  pesepay_integration_key: "..."
  pesepay_encryption_key: "..."   # exactly 32 characters for AES-256
  pesepay_sandbox: false          # true for testing
  monthly_price: 25.00
  currency: USD
  result_url: "https://sms.territechnologies.com/api/3rdparty/v1/billing/pesepay/callback"
  return_url: "https://wherever-the-client-should-land-after-paying"
```

## Visibility (not enforcement)

A daily worker task (`billing:expire-visibility`) logs subscriptions that
expired in the last 24h, for ops awareness. It does not gate anything itself -
see step 6 above.

## Admin dashboard

The **Subscription** card on `https://admin.territechnologies.com` (and the
same-origin `/admin` copy baked into the server image, when it's current) does
this without curl: shows active/expiry, a **Subscribe / Renew** button that
opens the Pesepay checkout, and a **Refresh status** button.

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/3rdparty/v1/billing/subscribe` | account | Start a payment, get the Pesepay redirect URL |
| GET | `/3rdparty/v1/billing/status` | account | `{active, currentPeriodEnd}` |
| POST | `/3rdparty/v1/billing/pesepay/callback` | none (public) | Pesepay's result callback |
