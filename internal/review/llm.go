package review

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/store"
)

// LLMSelection is the resolved Registered LLM Provider + Model for one Review Run.
type LLMSelection struct {
	ProviderName string // display / snapshot
	ProviderKey  string
	ModelName    string
	ConfigJSON   string
	ProviderFlag string // OCR --provider (Provider Key)
	ModelFlag    string // OCR --model
}

// ResolveLLMSelection picks one pair from the effective LLM Rotation Set
// (Repo override replaces Global) for a Review Run.
func ResolveLLMSelection(ctx context.Context, st *store.Store, gs store.GlobalSettings, repo store.RepoView, language string) (LLMSelection, error) {
	pairs := gs.EffectiveLLMRotation()
	setKey := store.LLMRotationKeyGlobal
	if override := repo.EffectiveLLMRotation(); len(override) > 0 {
		pairs = override
		setKey = store.LLMRotationKeyRepo(repo.ID)
	}
	claimed, err := st.ClaimLLMRotation(ctx, setKey, pairs)
	if err != nil {
		return LLMSelection{}, err
	}
	return buildLedgerSelection(ctx, st, claimed.ProviderID, claimed.ModelID, language)
}

func buildLedgerSelection(ctx context.Context, st *store.Store, providerID, modelID int64, language string) (LLMSelection, error) {
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
		ProviderFlag: p.ProviderKey,
		ModelFlag:    m.ModelName,
	}, nil
}

// OCRHomeDir returns the per-Review-Run OCR HOME (ADR-0006).
func OCRHomeDir(dataDir string, runID int64) string {
	return filepath.Join(dataDir, "ocr-home", fmt.Sprintf("run-%d", runID))
}

// PruneOrphanOCRHomes removes leftover run-* dirs under ocr-home.
// best-effort startup cleanup; RemoveAll errors ignored, ReadDir failure is logged.
func PruneOrphanOCRHomes(ocrHomeRoot string, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	entries, err := os.ReadDir(ocrHomeRoot)
	if err != nil {
		log.Warn("ocr home prune: read dir failed", "dir", ocrHomeRoot, "err", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "run-") {
			continue
		}
		_ = os.RemoveAll(filepath.Join(ocrHomeRoot, e.Name()))
	}
}
