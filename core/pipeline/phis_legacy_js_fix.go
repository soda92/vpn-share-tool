package pipeline

import (
	"regexp"

	"github.com/soda92/vpn-share-tool/core/models"
)

var (
	reStopItBlock        = regexp.MustCompile(`function\s+_stopIt\(e\)\s*\{[\s\S]*?return\s+false;\s*\}`)
	reShowModalCheck     = regexp.MustCompile(`if\s*\(\s*window\.showModalDialog\s*==\s*undefined\s*\)`)
	reWindowOpenFallback = regexp.MustCompile(`window\.open\(url,obj,"width="\+w\+",height="\+h\+",modal=yes,toolbar=no,menubar=no,scrollbars=yes,resizeable=no,location=no,status=no"\);`)
	reEhrOpenChrome      = regexp.MustCompile(`Ehr\.openChrome\s*=\s*function\s*\(\s*url\s*\)\s*\{`)
	reEhrWindowOpen      = regexp.MustCompile(`window\.open\(\s*url\s*,\s*""\s*,\s*[^;]+\);`)
)

func FixLegacyJS(ctx *models.ProcessingContext, body string) string {
	// Remove disable_backspace script using regex
	body = reStopItBlock.ReplaceAllString(body, "")

	// Replace openModalDialog logic
	body = reShowModalCheck.ReplaceAllString(body, "if(true)")
	body = reWindowOpenFallback.ReplaceAllString(body, `window.open(url, "_blank");`)
	body = reEhrOpenChrome.ReplaceAllString(body, `Ehr.openChrome = function(url){ window.open(url, "_blank"); return;`)
	body = reEhrWindowOpen.ReplaceAllString(body, `window.open(url, "_blank");`)
	return body
}
