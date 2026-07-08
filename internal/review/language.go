package review

import (
	"strings"

	"github.com/jo3qma/ocr-mng/internal/store"
)

type wrapperMsgs struct {
	title                string
	noComments           string
	foundComments        string
	suggestion           string
	warnings             string
	general              string
	unknownFile          string
	defaultRequirement   string
	prSection            string
	requirementsSection  string
	titleLabel           string
	bodyLabel            string
	truncationMarker     string
}

var wrappers = map[string]wrapperMsgs{
	"Japanese": {
		title:               "## Open Code Review\n\n",
		noComments:          "コメントは生成されませんでした。",
		foundComments:       "**%d** 件のコメントが見つかりました。\n\n",
		suggestion:          "**提案:** ",
		warnings:            "### 警告\n",
		general:             "(全体)",
		unknownFile:         "(ファイル不明)",
		defaultRequirement:  "各コメントの path には変更対象ファイルのパスを必ず入れ、空文字にしないこと。",
		prSection:           "### PR 説明コンテキスト\n\n",
		requirementsSection: "### 要件\n\n",
		titleLabel:          "**タイトル:** ",
		bodyLabel:           "**本文:**\n",
		truncationMarker:    "\n\n...(本文は先頭 8,000 ルーンで切り詰められました)",
	},
	"English": {
		title:               "## Open Code Review\n\n",
		noComments:          "No comments generated.",
		foundComments:       "Found **%d** comment(s).\n\n",
		suggestion:          "**Suggestion:** ",
		warnings:            "### Warnings\n",
		general:             "(general)",
		unknownFile:         "(file unknown)",
		defaultRequirement:  "Every comment must set path to the changed file path; never leave path empty.",
		prSection:           "### PR Description Context\n\n",
		requirementsSection: "### Requirements\n\n",
		titleLabel:          "**Title:** ",
		bodyLabel:           "**Body:**\n",
		truncationMarker:    "\n\n...(body truncated at 8,000 runes from start)",
	},
	"Chinese": {
		title:               "## Open Code Review\n\n",
		noComments:          "未生成评论。",
		foundComments:       "发现 **%d** 条评论。\n\n",
		suggestion:          "**建议:** ",
		warnings:            "### 警告\n",
		general:             "(通用)",
		unknownFile:         "(文件未知)",
		defaultRequirement:  "每条评论的 path 必须填写变更文件的完整路径，不得留空。",
		prSection:           "### PR 描述上下文\n\n",
		requirementsSection: "### 要求\n\n",
		titleLabel:          "**标题:** ",
		bodyLabel:           "**正文:**\n",
		truncationMarker:    "\n\n...(正文已从开头截断至 8,000 字符)",
	},
}

func wrapperFor(lang string) wrapperMsgs {
	if w, ok := wrappers[lang]; ok {
		return w
	}
	return wrappers["Japanese"]
}

const maxPRBodyRunes = 8000

// BuildReviewBackground merges PR Description Context and OCR requirements for --background.
func BuildReviewBackground(lang, title, body, repoRequirement string) string {
	w := wrapperFor(lang)
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	var parts []string
	if title != "" || body != "" {
		var pr strings.Builder
		pr.WriteString(w.prSection)
		if title != "" {
			pr.WriteString(w.titleLabel)
			pr.WriteString(title)
			if body != "" {
				pr.WriteString("\n\n")
			}
		}
		if body != "" {
			pr.WriteString(w.bodyLabel)
			pr.WriteString(truncateRunes(body, maxPRBodyRunes, w.truncationMarker))
		}
		parts = append(parts, pr.String())
	}

	req := MergeOCRRequirement(lang, repoRequirement)
	if req != "" {
		parts = append(parts, w.requirementsSection+req)
	}
	return strings.Join(parts, "\n\n")
}

func truncateRunes(s string, max int, marker string) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + marker
}

// MergeOCRRequirement prepends the language-specific path requirement.
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
