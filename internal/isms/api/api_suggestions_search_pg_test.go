package api

import (
	"encoding/json"
	"strings"
	"testing"

	"isms.sh/internal/isms/db"
)

// Regression for #500: a record created or changed by applying a suggestion was
// missing from search (Cmd/Ctrl+K, the Links picker, # mentions) until the org's
// index was next rebuilt. The HTTP create/update handlers patch the index
// directly; the suggestion apply handlers never did. The index here is built
// BEFORE the applies and stays fresh (searchIndexTTL), so the only way a record
// becomes findable is an apply-time upsert — the same condition the bug shows
// under in production.
//
// Requires a migrated Postgres; skipped otherwise, same as
// change_followup_task_pg_test.go:
//
//	ISMS_TEST_DATABASE_URL="postgres://…/isms_db_test?sslmode=disable" go test ./internal/isms/api/...

// searchFinds asserts exactly one entry of the given type matches q and returns it.
func searchFinds(t *testing.T, s *Server, orgID int, entryType, q string) SearchEntry {
	t.Helper()
	got := s.searchIndex.Search(orgID, q, func() { s.buildSearchIndex(orgID) })
	if len(got) != 1 {
		t.Fatalf("search %q after apply: %d results (%v), want exactly 1 %s", q, len(got), got, entryType)
	}
	if got[0].Type != entryType {
		t.Fatalf("search %q: type %q, want %s", q, got[0].Type, entryType)
	}
	return got[0]
}

// searchFindsNothing asserts nothing matches q.
func searchFindsNothing(t *testing.T, s *Server, orgID int, q string) {
	t.Helper()
	got := s.searchIndex.Search(orgID, q, func() { s.buildSearchIndex(orgID) })
	if len(got) != 0 {
		t.Fatalf("search %q: %d results (%v), want none", q, len(got), got)
	}
}

func TestSuggestionApplyUpdatesSearchIndex(t *testing.T) {
	s := testServer(t)
	orgID := newTestOrg(t, s, "suggestion-search")

	// A record created the normal way, so the update case has an indexed target.
	ctrl := newTestRisk(t, s, orgID, "Quartz leaks secrets")
	// Fresh org: one build so the index exists and is fresh — as if a user had
	// just searched before the suggestion arrived.
	s.buildSearchIndex(orgID)

	// Create through a suggestion: findable by title and identifier right away.
	sgCreate := newTestSuggestion(t, s, orgID, &db.Suggestion{
		EntityType:     "risk",
		SuggestionType: "create",
		Payload: json.RawMessage(`{"title":"Zephyr ambush","risk_type":"` + db.RiskTypes[0] +
			`","origin":"` + db.RiskOrigins[0] + `","status":"` + db.RiskStatuses[0] +
			`","treatment":"mitigate","current_likelihood":2,"current_impact":3}`),
	})
	applySuggestion(t, s, orgID, sgCreate.ID)
	entry := searchFinds(t, s, orgID, "risk", "zephyr ambush")
	if !strings.HasPrefix(entry.ID, "RISK-") {
		t.Errorf("entry.ID = %q, want the RISK-n identifier", entry.ID)
	}

	// Rename through an update suggestion: the new title finds the record, the
	// old one no longer does.
	sgRename := newTestSuggestion(t, s, orgID, &db.Suggestion{
		EntityType:     "risk",
		EntityID:       ctrl.Identifier,
		SuggestionType: "update",
		Payload:        json.RawMessage(`{"fields":{"title":"Obsidian exfiltration"}}`),
	})
	applySuggestion(t, s, orgID, sgRename.ID)
	searchFinds(t, s, orgID, "risk", "obsidian exfiltration")
	searchFindsNothing(t, s, orgID, "quartz leaks secrets")

	// A second type with its own builder: supplier create via suggestion.
	sgSupplier := newTestSuggestion(t, s, orgID, &db.Suggestion{
		EntityType:     "supplier",
		SuggestionType: "create",
		Payload:        json.RawMessage(`{"name":"Halcyon cloud"}`),
	})
	applySuggestion(t, s, orgID, sgSupplier.ID)
	searchFinds(t, s, orgID, "supplier", "halcyon cloud")
}

func TestSuggestionApplyTaskSearchVisibility(t *testing.T) {
	s := testServer(t)
	orgID := newTestOrg(t, s, "suggestion-search-task")
	// tasks.assignee_id is NOT NULL and defaults to the applier, so the acting
	// manager must exist in this org.
	contractTestUser(t, s, orgID, suggestionChangelogActor, "manager")
	s.buildSearchIndex(orgID)

	// A public task created through a suggestion is searchable straight away...
	sgPublic := newTestSuggestion(t, s, orgID, &db.Suggestion{
		EntityType:     "task",
		SuggestionType: "create",
		Payload:        json.RawMessage(`{"title":"Publish menace roster"}`),
	})
	applySuggestion(t, s, orgID, sgPublic.ID)
	searchFinds(t, s, orgID, "task", "publish menace roster")

	// ...while a private task never enters the shared index (#178).
	sgPrivate := newTestSuggestion(t, s, orgID, &db.Suggestion{
		EntityType:     "task",
		SuggestionType: "create",
		Payload:        json.RawMessage(`{"title":"Classified inventory","private":true}`),
	})
	applySuggestion(t, s, orgID, sgPrivate.ID)
	searchFindsNothing(t, s, orgID, "classified inventory")
}
