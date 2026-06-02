package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	currentLang = "en"
	translations = make(map[string]map[string]string)
	mu           sync.RWMutex
)

// SetLang initializes or changes the current language
func SetLang(lang string) {
	mu.Lock()
	defer mu.Unlock()

	if lang == "" {
		lang = "en"
	}
	currentLang = lang

	// Ensure we load at least English as fallback
	if _, ok := translations["en"]; !ok {
		loadLocale("en")
	}

	if lang != "en" {
		loadLocale(lang)
	}
}

// T returns the translated string for a given key.
// If the key is missing in the current language, it falls back to English.
// If English is also missing, it returns the key itself.
func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()

	// Try current language
	if dict, ok := translations[currentLang]; ok {
		if val, ok := dict[key]; ok {
			return val
		}
	}

	// Fallback to English
	if dict, ok := translations["en"]; ok {
		if val, ok := dict[key]; ok {
			return val
		}
	}

	return key
}

func loadLocale(lang string) {
	filename := fmt.Sprintf("locales/%s.json", lang)
	data, err := localesFS.ReadFile(filename)
	if err != nil {
		// If custom locale fails, don't crash, English fallback is already handled
		return
	}

	var dict map[string]string
	if err := json.Unmarshal(data, &dict); err != nil {
		return
	}

	translations[lang] = dict
}
