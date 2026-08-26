package web

import (
	_ "embed"
	"encoding/json"

	"github.com/jo3qma/ocr-mng/internal/store"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

//go:embed static/llm_rotation.js
var llmRotationJS string

type llmRotationWidget struct {
	FieldName  string
	RequireMin bool
	Values     []string
	Config     llmRotationWidgetConfig
	ConfigJSON string
}

type llmRotationWidgetConfig struct {
	FieldName  string              `json:"fieldName"`
	RequireMin bool                `json:"requireMin"`
	Options    []llmPairOptionJSON `json:"options"`
	Labels     llmRotationUILabels `json:"labels"`
}

type llmPairOptionJSON struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type llmRotationUILabels struct {
	ClearRow       string `json:"clearRow"`
	Add            string `json:"add"`
	ModalTitle     string `json:"modalTitle"`
	SelectAll      string `json:"selectAll"`
	ClearSelection string `json:"clearSelection"`
	Confirm        string `json:"confirm"`
	Cancel         string `json:"cancel"`
	NoAdd          string `json:"noAdd"`
	RequireMin     string `json:"requireMin"`
}

func buildLLMRotationWidget(l i18n.Localizer, fieldName string, requireMin bool, opts []llmPairOption, pairs []store.LLMPair) llmRotationWidget {
	jsonOpts := make([]llmPairOptionJSON, len(opts))
	for i, o := range opts {
		jsonOpts[i] = llmPairOptionJSON{Value: o.Value, Label: o.Label}
	}
	cfg := llmRotationWidgetConfig{
		FieldName:  fieldName,
		RequireMin: requireMin,
		Options:    jsonOpts,
		Labels: llmRotationUILabels{
			ClearRow:       l.T("form.llm_pair_clear_row"),
			Add:            l.T("form.llm_pair_add"),
			ModalTitle:     l.T("form.llm_pair_modal_title"),
			SelectAll:      l.T("form.llm_pair_select_all"),
			ClearSelection: l.T("form.llm_pair_clear_selection"),
			Confirm:        l.T("form.llm_pair_modal_confirm"),
			Cancel:         l.T("btn.cancel"),
			NoAdd:          l.T("form.llm_pair_no_add"),
			RequireMin:     l.T("form.llm_rotation_require_min"),
		},
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		configJSON = []byte("null")
	}
	return llmRotationWidget{
		FieldName:  fieldName,
		RequireMin: requireMin,
		Values:     llmRotationValues(pairs),
		Config:     cfg,
		ConfigJSON: string(configJSON),
	}
}
