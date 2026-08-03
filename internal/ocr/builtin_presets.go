package ocr

import "strings"

// BuiltinPresetOther is the form value for a manual provider_key (not in the curated list).
const BuiltinPresetOther = "__other__"

// BuiltinPreset is a curated Open Code Review built-in provider template.
type BuiltinPreset struct {
	Key   string
	Label string
}

// ponytail: static copy of OCR Configuration built-in table; sync when target OCR version bumps.
// Upgrade path: https://open-codereview.ai/docs/configuration
var builtinPresets = []BuiltinPreset{
	{Key: "anthropic", Label: "Anthropic"},
	{Key: "openai", Label: "OpenAI"},
	{Key: "dashscope", Label: "DashScope"},
	{Key: "dashscope-tokenplan", Label: "DashScope TokenPlan"},
	{Key: "volcengine", Label: "Volcengine"},
	{Key: "deepseek", Label: "DeepSeek"},
	{Key: "tencent-tokenhub", Label: "Tencent TokenHub"},
	{Key: "hy-tokenplan", Label: "Tencent Hunyuan TokenPlan"},
	{Key: "iflytek", Label: "iFlytek Spark"},
	{Key: "kimi", Label: "Kimi"},
	{Key: "z-ai", Label: "Z.AI"},
	{Key: "mimo", Label: "MiMo"},
	{Key: "minimax", Label: "MiniMax"},
	{Key: "baidu-qianfan", Label: "Baidu Qianfan"},
}

// BuiltinPresets returns the curated built-in provider templates.
func BuiltinPresets() []BuiltinPreset {
	out := make([]BuiltinPreset, len(builtinPresets))
	copy(out, builtinPresets)
	return out
}

// IsBuiltinPresetKey reports whether key is in the curated built-in list.
func IsBuiltinPresetKey(key string) bool {
	_, ok := BuiltinPresetLabel(key)
	return ok
}

// BuiltinPresetLabel returns the display label for a curated built-in key.
func BuiltinPresetLabel(key string) (string, bool) {
	key = strings.TrimSpace(key)
	for _, p := range builtinPresets {
		if p.Key == key {
			return p.Label, true
		}
	}
	return "", false
}

// SelectedBuiltinPreset maps a stored provider_key to a form preset value.
func SelectedBuiltinPreset(providerKey string) string {
	if IsBuiltinPresetKey(providerKey) {
		return strings.TrimSpace(providerKey)
	}
	return BuiltinPresetOther
}

// BuiltinProviderDocsURL is the OCR Configuration doc for built-in providers.
func BuiltinProviderDocsURL(uiLang string) string {
	if strings.TrimSpace(uiLang) == "ja" {
		return "https://open-codereview.ai/docs/ja/configuration"
	}
	return "https://open-codereview.ai/docs/configuration"
}
