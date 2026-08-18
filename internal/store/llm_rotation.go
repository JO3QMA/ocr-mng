package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
)

// LLMPair is one Registered LLM Provider + Registered LLM Model in an LLM Rotation Set.
type LLMPair struct {
	ProviderID int64 `json:"provider_id"`
	ModelID    int64 `json:"model_id"`
}

func ValidateLLMRotation(pairs []LLMPair) error {
	seen := make(map[string]struct{}, len(pairs))
	for _, p := range pairs {
		if err := ValidateLLMPairIDs(p.ProviderID, p.ModelID); err != nil {
			return err
		}
		if p.ProviderID == 0 {
			return fmt.Errorf("llm rotation pair cannot be empty")
		}
		key := fmt.Sprintf("%d:%d", p.ProviderID, p.ModelID)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate llm pair in rotation set")
		}
		seen[key] = struct{}{}
	}
	return nil
}

// EffectiveLLMRotation returns the Global LLM Rotation Set.
func (gs GlobalSettings) EffectiveLLMRotation() []LLMPair {
	return gs.DefaultLLMRotation
}

// EffectiveLLMRotation returns the Repo override set, or nil to follow Global.
func (r Repo) EffectiveLLMRotation() []LLMPair {
	return r.LLMRotation
}

func marshalLLMRotation(pairs []LLMPair) (sql.NullString, error) {
	if len(pairs) == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(pairs)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func parseLLMRotationJSON(raw sql.NullString) ([]LLMPair, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var pairs []LLMPair
	if err := json.Unmarshal([]byte(raw.String), &pairs); err != nil {
		return nil, err
	}
	return pairs, nil
}

func (s *Store) assertLLMRotationSelectable(ctx context.Context, pairs []LLMPair) error {
	if err := ValidateLLMRotation(pairs); err != nil {
		return err
	}
	for _, p := range pairs {
		if err := s.assertLLMPairSelectable(ctx, p.ProviderID, p.ModelID); err != nil {
			return err
		}
	}
	return nil
}

// PickLLMRotation picks one usable pair uniformly at random.
// Usable means provider/model enabled, model belongs to provider, and API key present.
func (s *Store) PickLLMRotation(ctx context.Context, pairs []LLMPair) (LLMPair, error) {
	if len(pairs) == 0 {
		return LLMPair{}, fmt.Errorf("llm rotation set is empty")
	}
	if err := ValidateLLMRotation(pairs); err != nil {
		return LLMPair{}, err
	}
	var usable []LLMPair
	for _, p := range pairs {
		ok, err := s.llmPairUsable(ctx, p)
		if err != nil {
			return LLMPair{}, err
		}
		if ok {
			usable = append(usable, p)
		}
	}
	if len(usable) == 0 {
		return LLMPair{}, fmt.Errorf("no usable llm pair in rotation set")
	}
	return usable[rand.IntN(len(usable))], nil
}

func (s *Store) llmPairUsable(ctx context.Context, p LLMPair) (bool, error) {
	var providerEnabled int
	var enc sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled, api_key_encrypted FROM llm_providers WHERE id=?`, p.ProviderID).
		Scan(&providerEnabled, &enc)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if providerEnabled != 1 || !enc.Valid || enc.String == "" {
		return false, nil
	}
	var modelProviderID int64
	var modelEnabled int
	err = s.db.QueryRowContext(ctx, `
		SELECT provider_id, enabled FROM llm_provider_models WHERE id=?`, p.ModelID).
		Scan(&modelProviderID, &modelEnabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return modelProviderID == p.ProviderID && modelEnabled == 1, nil
}

func (s *Store) listAllRepoLLMRotations(ctx context.Context) ([][]LLMPair, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT llm_rotation FROM repos`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out [][]LLMPair
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		pairs, err := parseLLMRotationJSON(raw)
		if err != nil {
			return nil, err
		}
		if len(pairs) > 0 {
			out = append(out, pairs)
		}
	}
	return out, rows.Err()
}
