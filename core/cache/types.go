package cache

import (
	"github.com/soda92/vpn-share-tool/core/models"
)

// StringProcessor processes the string content of a response body.
type StringProcessor func(ctx *models.ProcessingContext, body string) string
