package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAdminPromptGenerateJobCreateRequiresAdmin(t *testing.T) {
	_, ts := newServerHarness(t)

	resp := doRequestNoRedirect(t, ts, http.MethodPost, "/admin/prompts/generate-jobs", nil)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
}

func TestAdminPromptGenerateJobCreateValidation(t *testing.T) {
	srv, ts := newServerHarness(t)

	ensureAuthenticatedUser(t, ts)
	promoteSessionUsersToAdmin(t, srv)
	resp := doFormRequest(t, ts, http.MethodPost, "/admin/prompts/generate-jobs", url.Values{
		"q": {"otter"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "Please provide guidance for the prompt generation.") {
		t.Fatalf("expected validation message, got %q", string(body))
	}
}

func TestAdminPromptGenerateJobPollNotFound(t *testing.T) {
	srv, ts := newServerHarness(t)

	ensureAuthenticatedUser(t, ts)
	promoteSessionUsersToAdmin(t, srv)
	resp := doRequest(t, ts, http.MethodGet, "/admin/prompts/generate-jobs/999999", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "Prompt generation job not found") {
		t.Fatalf("expected not-found message, got %q", string(body))
	}
}

func TestAdminPromptGenerateJobAsyncFailure(t *testing.T) {
	srv, _ := newServerHarness(t)

	job := srv.createPromptGenerateJob("")
	go srv.runPromptGenerateJob(job.ID, "short abstract prompts", 10)

	jobID := job.ID
	var lastBody string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := srv.getPromptGenerateJob(jobID)
		if !ok {
			t.Fatalf("expected job %s to exist", jobID)
		}
		lastBody = snapshot.Error
		if snapshot.State == promptGenerateJobStateFailed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(lastBody, "OpenAI API key is not configured.") {
		t.Fatalf("expected OpenAI config error, got %q", lastBody)
	}
}

func doFormRequest(t *testing.T, ts *httptest.Server, method, path string, form url.Values) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err := testClientForServer(ts).Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	return resp
}
