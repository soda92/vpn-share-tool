package pipeline

import (
	"log"
	"regexp"
	"runtime/trace"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/resources"
)

var reCaptchaImage = regexp.MustCompile(`<img[^>]+src=["']/phis/app/login/voCode["']`)

func InjectCaptchaSolver(ctx *models.ProcessingContext, body string) string {
	defer trace.StartRegion(ctx.ReqContext, "InjectCaptchaSolver").End()
	if reCaptchaImage.MatchString(body) {
		log.Println("Injecting Captcha Solver Script")

		return strings.Replace(body, "</body>", `<script>`+string(resources.SolverScript)+`</script>`+"</body>", 1)
	}
	return body
}
