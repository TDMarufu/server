package statuswebhooks

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

var (
	ErrInvalidScheme     = errors.New("url must use https://")
	ErrUnresolvableHost  = errors.New("url host could not be resolved")
	ErrPrivateOrInternal = errors.New("url must not resolve to a private/internal network address")
)

// ValidateURL guards against SSRF: a registered webhook URL must not let the server
// POST to internal/private network targets. Mirrors the Android app's
// WebhookUrlValidator.kt (modules/webhooks/WebhookUrlValidator.kt) - resolve the host,
// reject loopback/link-local/private/multicast addresses.
func ValidateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	if parsed.Scheme != "https" {
		return ErrInvalidScheme
	}

	addrs, err := net.LookupIP(parsed.Hostname())
	if err != nil {
		// Fail closed: if the host can't be resolved, don't allow it.
		return fmt.Errorf("%w: %w", ErrUnresolvableHost, err)
	}

	for _, addr := range addrs {
		if isUnsafeAddress(addr) {
			return ErrPrivateOrInternal
		}
	}

	return nil
}

func isUnsafeAddress(addr net.IP) bool {
	return addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsPrivate() ||
		addr.IsUnspecified() ||
		addr.IsMulticast()
}

// newHTTPClient never follows redirects: a destination valid at registration time
// could otherwise redirect to an internal target at delivery time.
func newHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
