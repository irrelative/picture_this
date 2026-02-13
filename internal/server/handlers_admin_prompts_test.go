package server

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"picture-this/internal/config"
)

func TestPromptLibraryBasePath(t *testing.T) {
	if got := promptLibraryBasePath(""); got != "/admin/prompts" {
		t.Fatalf("expected base path without query, got %q", got)
	}
	if got := promptLibraryBasePath("space cat"); got != "/admin/prompts?q=space+cat" {
		t.Fatalf("expected encoded query path, got %q", got)
	}
}

func TestPromptLibraryRedirectURL(t *testing.T) {
	if got := promptLibraryRedirectURL("", "Prompt updated."); got != "/admin/prompts?notice=Prompt+updated." {
		t.Fatalf("unexpected redirect URL: %q", got)
	}
	if got := promptLibraryRedirectURL("space cat", "Prompt updated."); got != "/admin/prompts?notice=Prompt+updated.&q=space+cat" {
		t.Fatalf("unexpected redirect URL with query: %q", got)
	}
}

func TestAdminPromptLibraryViewKeepsSearchQuery(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	resp := doRequest(t, ts, http.MethodGet, "/admin/prompts?q=otter", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)
	if !strings.Contains(html, `name="q"`) {
		t.Fatalf("expected search input in admin prompts view")
	}
	if !strings.Contains(html, `value="otter"`) {
		t.Fatalf("expected search value to be retained in admin prompts view")
	}
	if !strings.Contains(html, `href="/admin/prompts">Clear</a>`) {
		t.Fatalf("expected clear search link to be rendered")
	}
}
