//go:build !js

package stdlib

import (
	"net/http"
	"time"

	"github.com/loreste/weft/internal/netsafe"
)

// DefaultHTTPClient is used when host does not inject one (bounded timeout + SSRF guard).
// Private/link-local/metadata destinations are blocked unless WEFT_HTTP_ALLOW_PRIVATE=1.
func DefaultHTTPClient() *http.Client {
	return netsafe.SafeHTTPClient(30 * time.Second)
}
