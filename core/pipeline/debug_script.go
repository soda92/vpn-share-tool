package pipeline

import (
	"fmt"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/resources"
)

func InjectDebugScript(ctx *models.ProcessingContext, body string) string {
	if !ctx.Proxy.Settings.EnableDebugScript {
		return body
	}
	if strings.Contains(ctx.RespHeader.Get("Content-Type"), "text/html") {
		myIP := ctx.Services.MyIP
		apiPort := ctx.Services.APIPort
		if myIP != "" && apiPort != 0 {
			debugURL := fmt.Sprintf("http://%s:%d/debug", myIP, apiPort)
			script := strings.Replace(string(resources.InjectorScript), "__DEBUG_URL__", debugURL, 1)
			injectionHTML := "<script>" + string(script) + "</script>"
			return strings.Replace(body, "</body>", injectionHTML+"</body>", 1)
		}
	}
	return body
}
