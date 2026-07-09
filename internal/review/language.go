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
		defaultRequirement: "各コメントの path には変更対象ファイルのパスを必ず入れ、空文字にしないこと。",
	},
	"English": {
		title:              "## Open Code Review\n\n",
		noComments:         "No comments generated.",
		foundComments:      "Found **%d** comment(s).\n\n",
		suggestion:         "**Suggestion:** ",
		warnings:           "### Warnings\n",
		general:            "(general)",
		unknownFile:        "(file unknown)",
		defaultRequirement: "Every comment must set path to the changed file path; never leave path empty.",
	},
	"Chinese": {
		title:              "## Open Code Review\n\n",
		noComments:         "未生成评论。",
		foundComments:      "发现 **%d** 条评论。\n\n",
		suggestion:         "**建议:** ",
		warnings:           "### 警告\n",
		general:            "(通用)",
		unknownFile:        "(文件未知)",
		defaultRequirement: "每条评论的 path 必须填写变更文件的完整路径，不得留空。",
	},
}

func wrapperFor(lang string) wrapperMsgs {
	if w, ok := wrappers[lang]; ok {
		return w
	}
	return wrappers["Japanese"]
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
