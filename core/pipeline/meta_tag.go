package pipeline

import (
	"fmt"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

// InjectMetaTag injects metadata tags about the API server host and port into the head of HTML responses.
// This allows the companion browser extension to dynamically discover which remote API server to communicate with.
func InjectMetaTag(ctx *models.ProcessingContext, body string) string {
	if strings.Contains(ctx.RespHeader.Get("Content-Type"), "text/html") {
		myIP := ctx.Services.MyIP
		apiPort := ctx.Services.APIPort
		if myIP != "" && apiPort != 0 {
			metaHTML := fmt.Sprintf(`<meta name="vpn-share-api" content="http://%s:%d">`, myIP, apiPort)
			metaHTML += fmt.Sprintf(`<meta name="vpn-share-proxy-port" content="%d">`, ctx.Proxy.RemotePort)

			// Inject into head
			if idx := strings.Index(body, "<head>"); idx != -1 {
				return body[:idx+6] + metaHTML + body[idx+6:]
			}
		}
	}
	return body
}
