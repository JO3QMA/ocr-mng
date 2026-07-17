package review

import (
	"strings"

	"github.com/jo3qma/ocr-mng/internal/store"
)

type wrapperMsgs struct {
	title               string
	noComments          string
	foundComments       string
	suggestion          string
	warnings            string
	general             string
	unknownFile         string
	prSection           string
	requirementsSection string
	titleLabel          string
	bodyLabel           string
	truncationMarker    string
	severityLabel       string
	categoryLabel       string
}

var wrappers = map[string]wrapperMsgs{
	"Japanese": {
		title:               "## Open Code Review\n\n",
		noComments:          "コメントは生成されませんでした。",
		foundComments:       "**%d** 件のコメントが見つかりました。",
		suggestion:          "**提案:** ",
		warnings:            "### 警告\n",
		general:             "(全体)",
		unknownFile:         "(ファイル不明)",
		prSection:           "### PR 説明コンテキスト\n\n",
		requirementsSection: "### 要件\n\n",
		titleLabel:          "**タイトル:** ",
		bodyLabel:           "**本文:**\n",
		truncationMarker:    "\n\n...(本文は先頭 8,000 ルーンで切り詰められました)",
		severityLabel:       "深刻度",
		categoryLabel:       "分類",
	},
	"English": {
		title:               "## Open Code Review\n\n",
		noComments:          "No comments generated.",
		foundComments:       "Found **%d** comment(s).",
		suggestion:          "**Suggestion:** ",
		warnings:            "### Warnings\n",
		general:             "(general)",
		unknownFile:         "(file unknown)",
		prSection:           "### PR Description Context\n\n",
		requirementsSection: "### Requirements\n\n",
		titleLabel:          "**Title:** ",
		bodyLabel:           "**Body:**\n",
		truncationMarker:    "\n\n...(body truncated at 8,000 runes from start)",
		severityLabel:       "Severity",
		categoryLabel:       "Category",
	},
	"Chinese": {
		title:               "## Open Code Review\n\n",
		noComments:          "未生成评论。",
		foundComments:       "发现 **%d** 条评论。",
		suggestion:          "**建议:** ",
		warnings:            "### 警告\n",
		general:             "(通用)",
		unknownFile:         "(文件未知)",
		prSection:           "### PR 描述上下文\n\n",
		requirementsSection: "### 要求\n\n",
		titleLabel:          "**标题:** ",
		bodyLabel:           "**正文:**\n",
		truncationMarker:    "\n\n...(正文已从开头截断至 8,000 字符)",
		severityLabel:       "严重程度",
		categoryLabel:       "分类",
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

	if req := strings.TrimSpace(repoRequirement); req != "" {
		parts = append(parts, w.requirementsSection+req)
	}
	return strings.Join(parts, "\n\n")
}

func truncateRunes(s string, max int, marker string) string {
	count := 0
	for i := range s {
		if count >= max {
			return s[:i] + marker
		}
		count++
	}
	return s
}

// ApprovalBody is the short body for a separate APPROVE review (comment mode / inline fallback).
func ApprovalBody(lang string) string {
	switch lang {
	case "English":
		return "Open Code Review: No findings."
	case "Chinese":
		return "Open Code Review: 未发现问题。"
	default:
		return "Open Code Review: 指摘はありませんでした。"
	}
}

// ZeroFindingApprovalEnabled reports whether this run should post APPROVE on GitHub.
func ZeroFindingApprovalEnabled(repo store.RepoView, commentCount int) bool {
	return commentCount == 0 && repo.ApproveOnZeroFindings && repo.HostKind == "github"
}

// EffectiveReviewLanguage returns repo override or global default.
func EffectiveReviewLanguage(gs store.GlobalSettings, repoLang string) string {
	if repoLang != "" {
		return store.NormalizeReviewLanguage(repoLang)
	}
	return store.NormalizeReviewLanguage(gs.ReviewLanguage)
}
