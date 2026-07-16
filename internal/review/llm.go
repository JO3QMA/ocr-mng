package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/store"
)

// LLMSelection is the resolved provider+model (or legacy config) for one Review Run.
type LLMSelection struct {
	ProviderName string // display / snapshot
	ProviderKey  string
	ModelName    string
	ConfigJSON   string
	ModelFlag    string // OCR --model; empty uses config default
	Ledger       bool
}

// LedgerMode reports whether Global default LLM pair has been set (one-way switch from legacy).
func LedgerMode(gs store.GlobalSettings) bool {
	return gs.DefaultLLMProviderID != 0 && gs.DefaultLLMModelID != 0
}

// ResolveLLMSelection picks the LLM pair or legacy JSON path for a Review Run.
//
// ponytail: dual path until Global default pair is set once; then ledger-only.
// OCR_LLM_* env stripping is intentionally not done here (MVP); empty those in compose.
func ResolveLLMSelection(ctx context.Context, st *store.Store, gs store.GlobalSettings, repo store.RepoView, language string) (LLMSelection, error) {
	if !LedgerMode(gs) {
		return resolveLegacyLLM(gs, repo, language)
	}
	return resolveLedgerLLM(ctx, st, gs, repo, language)
}

func resolveLegacyLLM(gs store.GlobalSettings, repo store.RepoView, language string) (LLMSelection, error) {
	configJSON, err := ocr.ConfigWithLanguage(gs.OCRConfigJSON, language)
	if err != nil {
		return LLMSelection{}, fmt.Errorf("ocr config: %w", err)
	}
	model := strings.TrimSpace(repo.OCRModel)
	return LLMSelection{
		ProviderName: "",
		ModelName:    model,
		ConfigJSON:   configJSON,
		ModelFlag:    model,
		Ledger:       false,
	}, nil
}

func resolveLedgerLLM(ctx context.Context, st *store.Store, gs store.GlobalSettings, repo store.RepoView, language string) (LLMSelection, error) {
	providerID, modelID := gs.DefaultLLMProviderID, gs.DefaultLLMModelID
	if repo.LLMProviderID != 0 || repo.LLMModelID != 0 {
		if err := store.ValidateLLMPairIDs(repo.LLMProviderID, repo.LLMModelID); err != nil {
			return LLMSelection{}, err
		}
		providerID, modelID = repo.LLMProviderID, repo.LLMModelID
	}
	p, err := st.GetLLMProvider(ctx, providerID)
	if err != nil {
		return LLMSelection{}, fmt.Errorf("llm provider: %w", err)
	}
	if !p.Enabled {
		return LLMSelection{}, fmt.Errorf("llm provider %q is disabled", p.Name)
	}
	m, err := st.GetLLMProviderModel(ctx, modelID)
	if err != nil {
		return LLMSelection{}, fmt.Errorf("llm model: %w", err)
	}
	if m.ProviderID != providerID {
		return LLMSelection{}, fmt.Errorf("llm model does not belong to selected provider")
	}
	if !m.Enabled {
		return LLMSelection{}, fmt.Errorf("llm model %q is disabled", m.ModelName)
	}
	apiKey, err := st.LLMProviderAPIKey(ctx, providerID)
	if err != nil {
		return LLMSelection{}, fmt.Errorf("llm api key: %w", err)
	}
	if apiKey == "" {
		return LLMSelection{}, fmt.Errorf("llm provider %q has no api key", p.Name)
	}
	configJSON, err := ocr.BuildProviderConfig(p.Kind, p.ProviderKey, apiKey, p.APIBaseURL, p.Protocol, m.ModelName, language)
	if err != nil {
		return LLMSelection{}, err
	}
	return LLMSelection{
		ProviderName: p.Name,
		ProviderKey:  p.ProviderKey,
		ModelName:    m.ModelName,
		ConfigJSON:   configJSON,
		ModelFlag:    m.ModelName,
		Ledger:       true,
	}, nil
}

// OCRHomeDir returns the per-Review-Run OCR HOME (ADR-0006).
func OCRHomeDir(dataDir string, runID int64) string {
	return filepath.Join(dataDir, "ocr-home", fmt.Sprintf("run-%d", runID))
}

// PruneOrphanOCRHomes removes leftover run-* dirs under ocr-home.
// ponytail: best-effort startup cleanup; ignores errors and non-run entries.
func PruneOrphanOCRHomes(ocrHomeRoot string) {
	entries, err := os.ReadDir(ocrHomeRoot)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "run-") {
			continue
		}
		_ = os.RemoveAll(filepath.Join(ocrHomeRoot, e.Name()))
	}
}
