package gui

import (
	"encoding/json"
	"log"

	"golang.org/x/text/language"

	"github.com/Xuanwo/go-locale"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func initI18n() {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	bundle.LoadMessageFileFS(i18nFS, "i18n/en.json")
	bundle.LoadMessageFileFS(i18nFS, "i18n/zh.json")

	langTag, err := locale.Detect()
	if err != nil {
		log.Printf("Failed to detect locale, falling back to English: %v", err)
		langTag = language.English
	}

	matcher := language.NewMatcher([]language.Tag{
		language.English,
		language.Chinese,
	})
	tag, _, _ := matcher.Match(langTag)

	localizer = i18n.NewLocalizer(bundle, tag.String())
}

func l(messageID string, templateData ...map[string]interface{}) string {
	var data map[string]interface{}
	if len(templateData) > 0 {
		data = templateData[0]
	}
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		log.Printf("Failed to localize message '%s': %v", messageID, err)
		return messageID // Fallback to message ID
	}
	return msg
}
