package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const (
	ModelSourceAPI    = "api"
	ModelSourceManual = "manual"
)

// LLMModelSyncResult counts changes applied by SyncLLMProviderModels.
type LLMModelSyncResult struct {
	Added   int
	Deleted int
	New     int
}

func normalizeModelName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func remoteNameIndex(remote []string) map[string]string {
	out := make(map[string]string, len(remote))
	for _, name := range remote {
		key := normalizeModelName(name)
		if key == "" {
			continue
		}
		if _, ok := out[key]; !ok {
			out[key] = strings.TrimSpace(name)
		}
	}
	return out
}

// SyncLLMProviderModels reconciles the provider ledger with a remote model list.
// New remote names are inserted as disabled api rows with is_new set.
// Manual rows matching a remote name are promoted to api without changing enabled/is_new.
// Api rows missing from remote are deleted and removed from rotation sets.
func (s *Store) SyncLLMProviderModels(ctx context.Context, providerID int64, remote []string) (LLMModelSyncResult, error) {
	models, err := s.ListLLMProviderModels(ctx, providerID)
	if err != nil {
		return LLMModelSyncResult{}, err
	}
	remoteByNorm := remoteNameIndex(remote)

	byNorm := map[string]LLMProviderModel{}
	for _, m := range models {
		byNorm[normalizeModelName(m.ModelName)] = m
	}

	var result LLMModelSyncResult
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LLMModelSyncResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	for norm, m := range byNorm {
		if m.Source != ModelSourceManual {
			continue
		}
		canonical, ok := remoteByNorm[norm]
		if !ok {
			continue
		}
		if canonical != m.ModelName {
			if _, err := tx.ExecContext(ctx, `
				UPDATE llm_provider_models SET model_name=?, source=?, updated_at=?
				WHERE id=? AND provider_id=?`,
				canonical, ModelSourceAPI, now, m.ID, providerID); err != nil {
				return LLMModelSyncResult{}, err
			}
			m.ModelName = canonical
		} else if _, err := tx.ExecContext(ctx, `
			UPDATE llm_provider_models SET source=?, updated_at=?
			WHERE id=? AND provider_id=?`,
			ModelSourceAPI, now, m.ID, providerID); err != nil {
			return LLMModelSyncResult{}, err
		}
		m.Source = ModelSourceAPI
		byNorm[normalizeModelName(m.ModelName)] = m
	}

	for _, canonical := range remoteByNorm {
		norm := normalizeModelName(canonical)
		if _, ok := byNorm[norm]; ok {
			continue
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO llm_provider_models(provider_id, model_name, enabled, sort_order, source, is_new, created_at, updated_at)
			VALUES (?, ?, 0, 0, ?, 1, ?, ?)`,
			providerID, canonical, ModelSourceAPI, now, now)
		if err != nil {
			return LLMModelSyncResult{}, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return LLMModelSyncResult{}, err
		}
		byNorm[norm] = LLMProviderModel{
			ID: id, ProviderID: providerID, ModelName: canonical,
			Source: ModelSourceAPI, IsNew: true,
		}
		result.Added++
		result.New++
	}

	for _, m := range models {
		if m.Source != ModelSourceAPI {
			continue
		}
		if _, ok := remoteByNorm[normalizeModelName(m.ModelName)]; ok {
			continue
		}
		if err := pruneLLMModelFromRotations(ctx, s, tx, m.ID); err != nil {
			return LLMModelSyncResult{}, err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM llm_provider_models WHERE id=? AND provider_id=?`, m.ID, providerID)
		if err != nil {
			return LLMModelSyncResult{}, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return LLMModelSyncResult{}, err
		}
		if n == 0 {
			return LLMModelSyncResult{}, sql.ErrNoRows
		}
		result.Deleted++
	}

	if err := tx.Commit(); err != nil {
		return LLMModelSyncResult{}, err
	}
	return result, nil
}

func pruneLLMModelFromRotations(ctx context.Context, s *Store, tx *sql.Tx, modelID int64) error {
	gs, err := s.GetGlobalSettings(ctx)
	if err != nil {
		return err
	}
	filtered := filterLLMPairsByModel(gs.DefaultLLMRotation, modelID)
	if len(filtered) != len(gs.DefaultLLMRotation) {
		gs.DefaultLLMRotation = filtered
		if err := persistGlobalSettingsDB(ctx, tx, gs); err != nil {
			return err
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, llm_rotation FROM repos WHERE llm_rotation IS NOT NULL AND llm_rotation != ''`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var repoID int64
		var raw sql.NullString
		if err := rows.Scan(&repoID, &raw); err != nil {
			return err
		}
		pairs, err := parseLLMRotationJSON(raw)
		if err != nil {
			return err
		}
		next := filterLLMPairsByModel(pairs, modelID)
		if len(next) == len(pairs) {
			continue
		}
		rot, err := marshalLLMRotation(next)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE repos SET llm_rotation=? WHERE id=?`, rot, repoID); err != nil {
			return err
		}
	}
	return rows.Err()
}

func filterLLMPairsByModel(pairs []LLMPair, modelID int64) []LLMPair {
	if len(pairs) == 0 {
		return pairs
	}
	out := make([]LLMPair, 0, len(pairs))
	for _, p := range pairs {
		if p.ModelID == modelID {
			continue
		}
		out = append(out, p)
	}
	return out
}
