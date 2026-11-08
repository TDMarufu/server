// Package pesepay is a minimal client for Pesepay's payments-engine REST API,
// confirmed empirically against their sandbox (their own docs don't fully specify
// the encryption scheme): request/response bodies are wrapped in an encrypted
// "payload" field, AES-256-CBC, IV = first 16 bytes of the encryption key (used
// as raw UTF-8 bytes - a 32-character key is exactly 32 bytes = AES-256), PKCS7
// padding, base64-encoded ciphertext. The integration key goes in a "key" header.
package pesepay

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	sandboxBaseURL    = "https://api.test.sandbox.pesepay.com/payments-engine"
	productionBaseURL = "https://api.pesepay.com/api/payments-engine"

	// statusSuccess is the only transactionStatus value that means "paid" -
	// confirmed against the reference PHP SDK, since Pesepay's own docs don't
	// state this explicitly. Comparison must not treat "INITIATED" as paid.
	statusSuccess = "SUCCESS"
)

type Client struct {
	integrationKey string
	encryptionKey  []byte
	baseURL        string
	httpClient     *http.Client
}

func NewClient(integrationKey, encryptionKey string, sandbox bool) *Client {
	baseURL := productionBaseURL
	if sandbox {
		baseURL = sandboxBaseURL
	}

	return &Client{
		integrationKey: integrationKey,
		encryptionKey:  []byte(encryptionKey),
		baseURL:        baseURL,
		httpClient:     &http.Client{},
	}
}

type InitiateResult struct {
	ReferenceNumber string
	RedirectURL     string
}

// InitiateTransaction starts a redirect-checkout transaction. amount is in major
// currency units (e.g. 25.00 for $25).
func (c *Client) InitiateTransaction(
	ctx context.Context,
	amount float64,
	currencyCode string,
	reason string,
	resultURL string,
	returnURL string,
) (*InitiateResult, error) {
	body := map[string]any{
		"amountDetails": map[string]any{
			"amount":       amount,
			"currencyCode": currencyCode,
		},
		"reasonForPayment": reason,
		"resultUrl":        resultURL,
		"returnUrl":        returnURL,
	}

	var decoded struct {
		ReferenceNumber string `json:"referenceNumber"`
		RedirectURL     string `json:"redirectUrl"`
	}
	if err := c.post(ctx, "/v1/payments/initiate", body, &decoded); err != nil {
		return nil, err
	}

	if decoded.ReferenceNumber == "" || decoded.RedirectURL == "" {
		return nil, fmt.Errorf("pesepay: initiate response missing referenceNumber/redirectUrl")
	}

	return &InitiateResult{ReferenceNumber: decoded.ReferenceNumber, RedirectURL: decoded.RedirectURL}, nil
}

// CheckPayment re-verifies a transaction's status directly with Pesepay. Callers
// must use this (never the raw webhook body) before treating a payment as paid -
// Pesepay's own documentation states the result callback is unsigned/unencrypted
// and must be independently confirmed.
func (c *Client) CheckPayment(ctx context.Context, referenceNumber string) (paid bool, status string, err error) {
	url := fmt.Sprintf("%s/v1/payments/check-payment?referenceNumber=%s", c.baseURL, referenceNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, "", fmt.Errorf("pesepay: build request: %w", err)
	}
	req.Header.Set("key", c.integrationKey)

	var decoded struct {
		TransactionStatus string `json:"transactionStatus"`
	}
	if err := c.do(req, &decoded); err != nil {
		return false, "", err
	}

	return decoded.TransactionStatus == statusSuccess, decoded.TransactionStatus, nil
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	plain, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("pesepay: marshal request: %w", err)
	}

	encrypted, err := c.encrypt(plain)
	if err != nil {
		return fmt.Errorf("pesepay: encrypt request: %w", err)
	}

	wireBody, err := json.Marshal(map[string]string{"payload": encrypted})
	if err != nil {
		return fmt.Errorf("pesepay: marshal wire body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(wireBody))
	if err != nil {
		return fmt.Errorf("pesepay: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", c.integrationKey)

	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pesepay: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pesepay: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pesepay: unexpected status %d: %s", resp.StatusCode, string(rawBody))
	}

	var wrapper struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(rawBody, &wrapper); err != nil || wrapper.Payload == "" {
		return fmt.Errorf("pesepay: unexpected response shape: %s", string(rawBody))
	}

	plain, err := c.decrypt(wrapper.Payload)
	if err != nil {
		return fmt.Errorf("pesepay: decrypt response: %w", err)
	}

	if out != nil {
		if err := json.Unmarshal(plain, out); err != nil {
			return fmt.Errorf("pesepay: unmarshal decrypted response: %w", err)
		}
	}

	return nil
}

func (c *Client) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return "", err
	}

	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, c.encryptionKey[:16]).CryptBlocks(ciphertext, padded)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *Client) decrypt(payloadB64 string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a valid block size")
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, c.encryptionKey[:16]).CryptBlocks(plaintext, ciphertext)

	return pkcs7Unpad(plaintext), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return data
	}
	return data[:len(data)-padding]
}
