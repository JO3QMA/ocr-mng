package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

type LLMProvider struct {
	ID          int64
	Name        string
	ProviderKey string
	Kind        string // builtin | custom
	APIBaseURL  string
	Protocol    string
	HasAPIKey   bool
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LLMProviderModel struct {
	ID         int64
	ProviderID int64
	ModelName  string
	Enabled    bool
	SortOrder  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func ValidateLLMPairIDs(providerID, modelID int64) error {
	if providerID == 0 && modelID == 0 {
		return nil
	}
	if providerID == 0 || modelID == 0 {
		return fmt.Errorf("llm provider and model must both be set or both empty")
	}
	return nil
}

func (s *Store) assertLLMPairSelectable(ctx context.Context, providerID, modelID int64) error {
	if err := ValidateLLMPairIDs(providerID, modelID); err != nil {
		return err
	}
	if providerID == 0 {
		return nil
	}
	p, err := s.GetLLMProvider(ctx, providerID)
	if err != nil {
		return fmt.Errorf("llm provider: %w", err)
	}
	if !p.Enabled {
		return fmt.Errorf("llm provider %d is disabled", providerID)
	}
	m, err := s.GetLLMProviderModel(ctx, modelID)
	if err != nil {
		return fmt.Errorf("llm model: %w", err)
	}
	if m.ProviderID != providerID {
		return fmt.Errorf("llm model %d does not belong to provider %d", modelID, providerID)
	}
	if !m.Enabled {
		return fmt.Errorf("llm model %d is disabled", modelID)
	}
	return nil
}

func (s *Store) CreateLLMProvider(ctx context.Context, p LLMProvider, apiKey string) (int64, error) {
	normalizeLLMProvider(&p)
	if err := validateLLMProvider(p); err != nil {
		return 0, err
	}
	enc, err := s.encryptPAT(apiKey)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_providers(name, provider_key, kind, api_base_url, protocol, api_key_encrypted, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.ProviderKey, p.Kind, nullStr(p.APIBaseURL), nullStr(p.Protocol), nullStr(enc), b2i(p.Enabled), now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateLLMProvider(ctx context.Context, p LLMProvider, apiKey string, clearAPIKey bool) error {
	normalizeLLMProvider(&p)
	if err := validateLLMProvider(p); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	setKey, clear := 0, 0
	enc := ""
	if apiKey != "" {
		e, err := s.encryptPAT(apiKey)
		if err != nil {
			return err
		}
		setKey, enc = 1, e
	} else if clearAPIKey {
		clear = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE llm_providers SET name=?, provider_key=?, kind=?, api_base_url=?, protocol=?,
			api_key_encrypted=CASE WHEN ?=1 THEN ? WHEN ?=1 THEN NULL ELSE api_key_encrypted END,
			enabled=?, updated_at=?
		WHERE id=?`,
		p.Name, p.ProviderKey, p.Kind, nullStr(p.APIBaseURL), nullStr(p.Protocol),
		setKey, enc, clear, b2i(p.Enabled), now, p.ID)
	return err
}

func (s *Store) ListLLMProviders(ctx context.Context) ([]LLMProvider, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, provider_key, kind, api_base_url, protocol,
		       CASE WHEN api_key_encrypted IS NOT NULL AND api_key_encrypted != '' THEN 1 ELSE 0 END,
		       enabled, created_at, updated_at
		FROM llm_providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []LLMProvider
	for rows.Next() {
		p, err := scanLLMProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetLLMProvider(ctx context.Context, id int64) (LLMProvider, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, provider_key, kind, api_base_url, protocol,
		       CASE WHEN api_key_encrypted IS NOT NULL AND api_key_encrypted != '' THEN 1 ELSE 0 END,
		       enabled, created_at, updated_at
		FROM llm_providers WHERE id=?`, id)
	return scanLLMProvider(row)
}

func (s *Store) LLMProviderAPIKey(ctx context.Context, id int64) (string, error) {
	var enc sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT api_key_encrypted FROM llm_providers WHERE id=?`, id).Scan(&enc)
	if err != nil {
		return "", err
	}
	if !enc.Valid || enc.String == "" {
		return "", nil
	}
	return s.decryptPAT(enc.String)
}

func (s *Store) DeleteLLMProvider(ctx context.Context, id int64) error {
	if err := s.assertLLMProviderNotReferenced(ctx, id); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM llm_providers WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateLLMProviderModel(ctx context.Context, m LLMProviderModel) (int64, error) {
	if err := validateLLMProviderModel(m); err != nil {
		return 0, err
	}
	if _, err := s.GetLLMProvider(ctx, m.ProviderID); err != nil {
		return 0, fmt.Errorf("llm provider: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_provider_models(provider_id, model_name, enabled, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.ProviderID, m.ModelName, b2i(m.Enabled), m.SortOrder, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateLLMProviderModel(ctx context.Context, m LLMProviderModel) error {
	if err := validateLLMProviderModel(m); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE llm_provider_models SET model_name=?, enabled=?, sort_order=?, updated_at=?
		WHERE id=? AND provider_id=?`,
		m.ModelName, b2i(m.Enabled), m.SortOrder, now, m.ID, m.ProviderID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListLLMProviderModels(ctx context.Context, providerID int64) ([]LLMProviderModel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider_id, model_name, enabled, sort_order, created_at, updated_at
		FROM llm_provider_models WHERE provider_id=?
		ORDER BY sort_order, model_name`, providerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []LLMProviderModel
	for rows.Next() {
		m, err := scanLLMProviderModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetLLMProviderModel(ctx context.Context, id int64) (LLMProviderModel, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, provider_id, model_name, enabled, sort_order, created_at, updated_at
		FROM llm_provider_models WHERE id=?`, id)
	return scanLLMProviderModel(row)
}

func (s *Store) DeleteLLMProviderModel(ctx context.Context, id int64) error {
	if err := s.assertLLMModelNotReferenced(ctx, id); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM llm_provider_models WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) assertLLMProviderNotReferenced(ctx context.Context, id int64) error {
	gs, err := s.GetGlobalSettings(ctx)
	if err != nil {
		return err
	}
	if gs.DefaultLLMProviderID == id {
		return fmt.Errorf("llm provider %d is referenced by global settings", id)
	}
	if gs.DefaultLLMModelID != 0 {
		var n int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM llm_provider_models WHERE id=? AND provider_id=?`,
			gs.DefaultLLMModelID, id,
		).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("llm provider %d is referenced by global settings", id)
		}
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repos WHERE llm_provider_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("llm provider %d is referenced by a registered repo", id)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM repos WHERE llm_model_id IN (SELECT id FROM llm_provider_models WHERE provider_id=?)`,
		id,
	).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("llm provider %d is referenced by a registered repo", id)
	}
	return nil
}

func (s *Store) assertLLMModelNotReferenced(ctx context.Context, id int64) error {
	gs, err := s.GetGlobalSettings(ctx)
	if err != nil {
		return err
	}
	if gs.DefaultLLMModelID == id {
		return fmt.Errorf("llm model %d is referenced by global settings", id)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repos WHERE llm_model_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("llm model %d is referenced by a registered repo", id)
	}
	return nil
}

func normalizeLLMProvider(p *LLMProvider) {
	p.Name = strings.TrimSpace(p.Name)
	p.ProviderKey = strings.TrimSpace(p.ProviderKey)
	p.Kind = strings.TrimSpace(p.Kind)
	p.APIBaseURL = strings.TrimSpace(p.APIBaseURL)
	p.Protocol = strings.ToLower(strings.TrimSpace(p.Protocol))
	if p.Protocol == "" {
		p.Protocol = ocr.InferProtocol(p.APIBaseURL)
	}
}

func validateLLMProvider(p LLMProvider) error {
	if p.Name == "" || p.ProviderKey == "" {
		return fmt.Errorf("name and provider_key are required")
	}
	switch p.Kind {
	case "builtin", "custom":
	default:
		return fmt.Errorf("kind must be builtin or custom")
	}
	if p.Protocol != "" && !ocr.ValidProtocol(p.Protocol) {
		return fmt.Errorf("protocol must be anthropic, openai, or openai-responses")
	}
	if p.Kind == "custom" {
		if p.APIBaseURL == "" {
			return fmt.Errorf("api_base_url is required for custom providers")
		}
		if p.Protocol == "" {
			return fmt.Errorf("protocol is required for custom providers")
		}
	}
	return nil
}

func validateLLMProviderModel(m LLMProviderModel) error {
	if m.ProviderID == 0 || strings.TrimSpace(m.ModelName) == "" {
		return fmt.Errorf("provider_id and model_name are required")
	}
	return nil
}

func scanLLMProvider(scanner interface {
	Scan(dest ...any) error
}) (LLMProvider, error) {
	var p LLMProvider
	var base, protocol sql.NullString
	var has, enabled int
	var created, updated string
	err := scanner.Scan(&p.ID, &p.Name, &p.ProviderKey, &p.Kind, &base, &protocol, &has, &enabled, &created, &updated)
	if err != nil {
		return LLMProvider{}, err
	}
	if base.Valid {
		p.APIBaseURL = base.String
	}
	if protocol.Valid {
		p.Protocol = protocol.String
	}
	p.HasAPIKey = has == 1
	p.Enabled = enabled == 1
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return p, nil
}

func scanLLMProviderModel(scanner interface {
	Scan(dest ...any) error
}) (LLMProviderModel, error) {
	var m LLMProviderModel
	var enabled int
	var created, updated string
	err := scanner.Scan(&m.ID, &m.ProviderID, &m.ModelName, &enabled, &m.SortOrder, &created, &updated)
	if err != nil {
		return LLMProviderModel{}, err
	}
	m.Enabled = enabled == 1
	m.CreatedAt, _ = time.Parse(time.RFC3339, created)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return m, nil
}
