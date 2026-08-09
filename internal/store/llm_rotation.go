package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// LLMPair is one Registered LLM Provider + Registered LLM Model in an LLM Rotation Set.
type LLMPair struct {
	ProviderID int64 `json:"provider_id"`
	ModelID    int64 `json:"model_id"`
}

const LLMRotationKeyGlobal = "global"

func LLMRotationKeyRepo(repoID int64) string {
	return fmt.Sprintf("repo:%d", repoID)
}

func LLMPairsEqual(a, b []LLMPair) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ProviderID != b[i].ProviderID || a[i].ModelID != b[i].ModelID {
			return false
		}
	}
	return true
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

// EffectiveLLMRotation returns the Global LLM Rotation Set (legacy single pair if needed).
func (gs GlobalSettings) EffectiveLLMRotation() []LLMPair {
	if len(gs.DefaultLLMRotation) > 0 {
		return gs.DefaultLLMRotation
	}
	if gs.DefaultLLMProviderID != 0 && gs.DefaultLLMModelID != 0 {
		return []LLMPair{{ProviderID: gs.DefaultLLMProviderID, ModelID: gs.DefaultLLMModelID}}
	}
	return nil
}

// NormalizeDefaultLLMRotation syncs DefaultLLMRotation with legacy first-pair fields.
func (gs *GlobalSettings) NormalizeDefaultLLMRotation() {
	if len(gs.DefaultLLMRotation) == 0 && gs.DefaultLLMProviderID != 0 && gs.DefaultLLMModelID != 0 {
		gs.DefaultLLMRotation = []LLMPair{{ProviderID: gs.DefaultLLMProviderID, ModelID: gs.DefaultLLMModelID}}
	}
	if len(gs.DefaultLLMRotation) > 0 {
		gs.DefaultLLMProviderID = gs.DefaultLLMRotation[0].ProviderID
		gs.DefaultLLMModelID = gs.DefaultLLMRotation[0].ModelID
	}
}

// EffectiveLLMRotation returns the Repo override set, or nil to follow Global.
func (r Repo) EffectiveLLMRotation() []LLMPair {
	if len(r.LLMRotation) > 0 {
		return r.LLMRotation
	}
	if r.LLMProviderID != 0 && r.LLMModelID != 0 {
		return []LLMPair{{ProviderID: r.LLMProviderID, ModelID: r.LLMModelID}}
	}
	return nil
}

// NormalizeLLMRotation syncs LLMRotation with legacy first-pair columns.
func (r *Repo) NormalizeLLMRotation() {
	if len(r.LLMRotation) == 0 && r.LLMProviderID != 0 && r.LLMModelID != 0 {
		r.LLMRotation = []LLMPair{{ProviderID: r.LLMProviderID, ModelID: r.LLMModelID}}
	}
	if len(r.LLMRotation) > 0 {
		r.LLMProviderID = r.LLMRotation[0].ProviderID
		r.LLMModelID = r.LLMRotation[0].ModelID
	}
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

func (s *Store) ResetLLMRotationCursor(ctx context.Context, setKey string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_rotation_cursors(set_key, cursor_index) VALUES (?, 0)
		ON CONFLICT(set_key) DO UPDATE SET cursor_index=0`, setKey)
	return err
}

func (s *Store) DeleteLLMRotationCursor(ctx context.Context, setKey string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM llm_rotation_cursors WHERE set_key=?`, setKey)
	return err
}

// ClaimLLMRotation picks the next usable pair (round-robin) and advances the cursor.
// Usable means provider/model enabled, model belongs to provider, and API key present.
func (s *Store) ClaimLLMRotation(ctx context.Context, setKey string, pairs []LLMPair) (LLMPair, error) {
	if len(pairs) == 0 {
		return LLMPair{}, fmt.Errorf("llm rotation set is empty")
	}
	if err := ValidateLLMRotation(pairs); err != nil {
		return LLMPair{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LLMPair{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO llm_rotation_cursors(set_key, cursor_index) VALUES (?, 0)`, setKey); err != nil {
		return LLMPair{}, err
	}

	var cursor int
	if err := tx.QueryRowContext(ctx, `SELECT cursor_index FROM llm_rotation_cursors WHERE set_key=?`, setKey).Scan(&cursor); err != nil {
		return LLMPair{}, err
	}
	if cursor < 0 {
		cursor = 0
	}
	cursor %= len(pairs)

	var chosen LLMPair
	chosenIdx := -1
	for i := 0; i < len(pairs); i++ {
		idx := (cursor + i) % len(pairs)
		p := pairs[idx]
		ok, err := llmPairUsableTx(ctx, tx, p)
		if err != nil {
			return LLMPair{}, err
		}
		if ok {
			chosen = p
			chosenIdx = idx
			break
		}
	}
	if chosenIdx < 0 {
		return LLMPair{}, fmt.Errorf("no usable llm pair in rotation set")
	}
	next := (chosenIdx + 1) % len(pairs)
	if _, err := tx.ExecContext(ctx, `
		UPDATE llm_rotation_cursors SET cursor_index=? WHERE set_key=?`, next, setKey); err != nil {
		return LLMPair{}, err
	}
	if err := tx.Commit(); err != nil {
		return LLMPair{}, err
	}
	return chosen, nil
}

func llmPairUsableTx(ctx context.Context, tx *sql.Tx, p LLMPair) (bool, error) {
	var providerEnabled int
	var enc sql.NullString
	err := tx.QueryRowContext(ctx, `
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
	err = tx.QueryRowContext(ctx, `
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
	rows, err := s.db.QueryContext(ctx, `SELECT llm_rotation, llm_provider_id, llm_model_id FROM repos`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out [][]LLMPair
	for rows.Next() {
		var raw sql.NullString
		var pid, mid sql.NullInt64
		if err := rows.Scan(&raw, &pid, &mid); err != nil {
			return nil, err
		}
		pairs, err := parseLLMRotationJSON(raw)
		if err != nil {
			return nil, err
		}
		if len(pairs) == 0 && pid.Valid && mid.Valid && pid.Int64 != 0 && mid.Int64 != 0 {
			pairs = []LLMPair{{ProviderID: pid.Int64, ModelID: mid.Int64}}
		}
		if len(pairs) > 0 {
			out = append(out, pairs)
		}
	}
	return out, rows.Err()
}
