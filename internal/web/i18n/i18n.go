package i18n

// Localizer resolves UI strings for a single locale.
type Localizer struct {
	lang string
}

func New(lang string) Localizer {
	if lang != "en" {
		lang = "ja"
	}
	return Localizer{lang: lang}
}

func (l Localizer) T(key string) string {
	if msg, ok := catalogs[l.lang][key]; ok {
		return msg
	}
	if msg, ok := catalogs["en"][key]; ok {
		return msg
	}
	return key
}

var catalogs = map[string]map[string]string{
	"en": en,
	"ja": ja,
}
