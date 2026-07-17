package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDeleteLLMProvider_modelOnlyRefs(t *testing.T) {
	st, err := Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	providerID, err := st.CreateLLMProvider(ctx, LLMProvider{
		Name: "p1", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
	}, "sk")
	if err != nil {
		t.Fatal(err)
	}
	modelID, err := st.CreateLLMProviderModel(ctx, LLMProviderModel{
		ProviderID: providerID, ModelName: "m1", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Drift: global model id set without provider id (API forbids; assert must still block delete).
	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gs.DefaultLLMProviderID = 0
	gs.DefaultLLMModelID = modelID
	b, err := json.Marshal(gs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE global_settings SET value=? WHERE key='settings'`, string(b)); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteLLMProvider(ctx, providerID); err == nil || !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("expected global model ref reject: %v", err)
	}

	gs.DefaultLLMModelID = 0
	b, _ = json.Marshal(gs)
	if _, err := st.db.ExecContext(ctx, `UPDATE global_settings SET value=? WHERE key='settings'`, string(b)); err != nil {
		t.Fatal(err)
	}

	hostID, err := st.CreateGitHost(ctx, GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := st.CreateRepo(ctx, Repo{
		GitHostID: hostID, Owner: "acme", Name: "app",
		DefaultBranch: "main", TriggerLabel: "review", CommentMode: "inline", Enabled: true,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Drift: model FK only.
	if _, err := st.db.ExecContext(ctx,
		`UPDATE repos SET llm_provider_id=NULL, llm_model_id=? WHERE id=?`, modelID, repoID,
	); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteLLMProvider(ctx, providerID); err == nil || !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("expected repo model ref reject: %v", err)
	}
}
