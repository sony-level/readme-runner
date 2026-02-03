package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCopilotProviderCallAPI_SetsHeadersAndDefaultModel(t *testing.T) {
	var gotReq CopilotRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("expected Authorization header, got %q", auth)
		}
		if accept := r.Header.Get("Accept"); accept != "application/vnd.github+json" {
			t.Errorf("expected Accept header, got %q", accept)
		}
		if v := r.Header.Get("X-GitHub-Api-Version"); v != GitHubAPIVersion {
			t.Errorf("expected API version header %q, got %q", GitHubAPIVersion, v)
		}

		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "{\"version\":\"1\",\"project_type\":\"node\",\"prerequisites\":[],\"steps\":[{\"id\":\"install\",\"cmd\":\"npm ci\",\"cwd\":\".\",\"risk\":\"low\",\"requires_sudo\":false}],\"env\":{},\"ports\":[],\"notes\":[]}"
				},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()

	provider := NewCopilotProvider(&ProviderConfig{
		Type:     ProviderCopilot,
		Endpoint: server.URL,
		Timeout:  2 * time.Second,
	})

	plan, err := provider.callAPI("test prompt", "test-token")
	if err != nil {
		t.Fatalf("callAPI failed: %v", err)
	}
	if plan.ProjectType != "node" {
		t.Fatalf("expected project_type node, got %q", plan.ProjectType)
	}
	if gotReq.Model != DefaultCopilotModel {
		t.Fatalf("expected default model %q, got %q", DefaultCopilotModel, gotReq.Model)
	}
}

func TestCopilotProviderCallAPI_ForbiddenReturnsActionableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	provider := NewCopilotProvider(&ProviderConfig{
		Type:     ProviderCopilot,
		Endpoint: server.URL,
		Timeout:  2 * time.Second,
	})

	_, err := provider.callAPI("test prompt", "test-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "HTTP 403") {
		t.Fatalf("expected HTTP 403 in error, got %q", msg)
	}
	if !strings.Contains(msg, "models:read") {
		t.Fatalf("expected models:read hint in error, got %q", msg)
	}
}

func TestCopilotProviderGetTokenPrecedence(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GH_TOKEN", "gh-token")
	t.Setenv("RD_LLM_TOKEN", "rd-token")

	withConfigToken := NewCopilotProvider(&ProviderConfig{
		Type:  ProviderCopilot,
		Token: "config-token",
	})
	if got := withConfigToken.getToken(); got != "config-token" {
		t.Fatalf("expected config token, got %q", got)
	}

	noConfigToken := NewCopilotProvider(&ProviderConfig{
		Type: ProviderCopilot,
	})
	if got := noConfigToken.getToken(); got != "github-token" {
		t.Fatalf("expected GITHUB_TOKEN precedence, got %q", got)
	}
}

