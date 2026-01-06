package pipeline

import (
	_ "embed"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

type ContentProcessor func(ctx *models.ProcessingContext, body string) string

func RunPipeline(ctx *models.ProcessingContext, body string) string {
	// Skip processing for specific dynamic JS patterns or large libraries
	path := strings.ToLower(ctx.ReqURL.Path)
	if path == "*.js" {
		return body
	}

	// 1. Internal URL Rewrite
	if ctx.Proxy.Settings.EnableUrlRewrite {
		body = RewriteInternalURLs(ctx, body)
	}

	// 2. Content Modification (System Specific & Debug)
	if ctx.Proxy.Settings.EnableContentMod {
		// Run Debug Script injection (if it's HTML)
		body = InjectDebugScript(ctx, body)

		// Run System Specific Processors
		for _, activeSysID := range ctx.Proxy.ActiveSystems {
			for _, defSys := range DefinedSystems {
				if defSys.ID == activeSysID {
					for _, p := range defSys.Processors {
						body = p(ctx, body)
					}
				}
			}
		}
	}

	return body
}




