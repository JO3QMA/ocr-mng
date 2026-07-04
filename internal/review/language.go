package review

import "github.com/jo3qma/ocr-mng/internal/store"

type wrapperMsgs struct {
	title         string
	noComments    string
	foundComments string
	suggestion    string
	warnings      string
	general       string
}

var wrappers = map[string]wrapperMsgs{
	"Japanese": {
		title:         "## Open Code Review\n\n",
		noComments:    "コメントは生成されませんでした。",
		foundComments: "**%d** 件のコメントが見つかりました。\n\n",
		suggestion:    "**提案:** ",
		warnings:      "### 警告\n",
		general:       "(全体)",
	},
	"English": {
		title:         "## Open Code Review\n\n",
		noComments:    "No comments generated.",
		foundComments: "Found **%d** comment(s).\n\n",
		suggestion:    "**Suggestion:** ",
		warnings:      "### Warnings\n",
		general:       "(general)",
	},
	"Chinese": {
		title:         "## Open Code Review\n\n",
		noComments:    "未生成评论。",
		foundComments: "发现 **%d** 条评论。\n\n",
		suggestion:    "**建议:** ",
		warnings:      "### 警告\n",
		general:       "(通用)",
	},
}

func wrapperFor(lang string) wrapperMsgs {
	if w, ok := wrappers[lang]; ok {
		return w
	}
	return wrappers["Japanese"]
}

// EffectiveReviewLanguage returns repo override or global default.
func EffectiveReviewLanguage(gs store.GlobalSettings, repoLang string) string {
	if repoLang != "" {
		return store.NormalizeReviewLanguage(repoLang)
	}
	return store.NormalizeReviewLanguage(gs.ReviewLanguage)
}
