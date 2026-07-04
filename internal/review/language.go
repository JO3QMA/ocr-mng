package review

import (
	"strings"

	"github.com/jo3qma/ocr-mng/internal/store"
)

type wrapperMsgs struct {
	title              string
	noComments         string
	foundComments      string
	suggestion         string
	warnings           string
	general            string
	unknownFile        string
	defaultRequirement string
}

var wrappers = map[string]wrapperMsgs{
	"Japanese": {
		title:              "## Open Code Review\n\n",
		noComments:         "コメントは生成されませんでした。",
		foundComments:      "**%d** 件のコメントが見つかりました。\n\n",
		suggestion:         "**提案:** ",
		warnings:           "### 警告\n",
		general:            "(全体)",
		unknownFile:        "(ファイル不明)",
		defaultRequirement: "各コメントの file_path には変更対象ファイルのパスを必ず入れ、空文字にしないこと。",
	},
	"English": {
		title:              "## Open Code Review\n\n",
		noComments:         "No comments generated.",
		foundComments:      "Found **%d** comment(s).\n\n",
		suggestion:         "**Suggestion:** ",
		warnings:           "### Warnings\n",
		general:            "(general)",
		unknownFile:        "(file unknown)",
		defaultRequirement: "Every comment must set file_path to the changed file path; never leave file_path empty.",
	},
	"Chinese": {
		title:              "## Open Code Review\n\n",
		noComments:         "未生成评论。",
		foundComments:      "发现 **%d** 条评论。\n\n",
		suggestion:         "**建议:** ",
		warnings:           "### 警告\n",
		general:            "(通用)",
		unknownFile:        "(文件未知)",
		defaultRequirement: "每条评论的 file_path 必须填写变更文件的完整路径，不得留空。",
	},
}

func wrapperFor(lang string) wrapperMsgs {
	if w, ok := wrappers[lang]; ok {
		return w
	}
	return wrappers["Japanese"]
}

// MergeOCRRequirement prepends the language-specific file_path requirement.
func MergeOCRRequirement(lang, repoRequirement string) string {
	base := strings.TrimSpace(wrapperFor(lang).defaultRequirement)
	repo := strings.TrimSpace(repoRequirement)
	if repo == "" {
		return base
	}
	if base == "" {
		return repo
	}
	return base + "\n\n" + repo
}

// EffectiveReviewLanguage returns repo override or global default.
func EffectiveReviewLanguage(gs store.GlobalSettings, repoLang string) string {
	if repoLang != "" {
		return store.NormalizeReviewLanguage(repoLang)
	}
	return store.NormalizeReviewLanguage(gs.ReviewLanguage)
}
