package cache

import (
	"bytes"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func (t *CachingTransport) runPipeline(req *http.Request, header http.Header, body []byte) ([]byte, bool) {
	if t.Processor == nil {
		return body, false
	}

	var bodyStr string
	contentType := header.Get("Content-Type")
	isGBK := false
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		charset := strings.ToLower(params["charset"])
		isGBK = charset == "gbk" || charset == "gb2312"
	}

	if isGBK {
		reader := transform.NewReader(bytes.NewReader(body), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			log.Printf("Error decoding GBK body: %v", err)
			bodyStr = string(body)
		} else {
			bodyStr = string(decoded)
		}
	} else {
		bodyStr = string(body)
	}

	originalBodyStr := bodyStr

	ctx := &models.ProcessingContext{
		ReqURL:     req.URL,
		ReqContext: req.Context(),
		RespHeader: header,
		Proxy:      t.Proxy,
	}

	// Use the injected processor
	bodyStr = t.Processor(ctx, bodyStr)

	if bodyStr != originalBodyStr {
		if isGBK {
			if mediaType, params, err := mime.ParseMediaType(contentType); err == nil {
				params["charset"] = "utf-8"
				header.Set("Content-Type", mime.FormatMediaType(mediaType, params))
			}
		}
		return []byte(bodyStr), true
	}
	return body, false
}
