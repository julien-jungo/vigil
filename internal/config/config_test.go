package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/julien-jungo/vigil/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vigil.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_HappyPath(t *testing.T) {
	path := writeConfig(t, `
url: https://example.com
specs: ./specs
llm:
  provider: anthropic
  model: claude-sonnet-4-6
  api_key: test-key
playwright:
  headless: true
  viewport:
    width: 1280
    height: 720
reports:
  junit: ./reports/junit.xml
  html: ./reports/report.html
explore:
  max_steps: 50
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", cfg.URL, "https://example.com")
	}
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("LLM.Provider = %q, want %q", cfg.LLM.Provider, "anthropic")
	}
	if cfg.LLM.Model != "claude-sonnet-4-6" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "claude-sonnet-4-6")
	}
	if cfg.Playwright.Viewport.Width != 1280 {
		t.Errorf("Viewport.Width = %d, want 1280", cfg.Playwright.Viewport.Width)
	}
	if cfg.Explore.MaxSteps != 50 {
		t.Errorf("Explore.MaxSteps = %d, want 50", cfg.Explore.MaxSteps)
	}
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_API_KEY", "secret-key")

	path := writeConfig(t, `
url: https://example.com
llm:
  provider: anthropic
  model: claude-sonnet-4-6
  api_key: ${TEST_API_KEY}
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLM.APIKey != "secret-key" {
		t.Errorf("APIKey = %q, want %q", cfg.LLM.APIKey, "secret-key")
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing url",
			content: `
llm:
  provider: anthropic
  model: claude-sonnet-4-6
  api_key: test-key
`,
			wantErr: "url",
		},
		{
			name: "missing llm.provider",
			content: `
url: https://example.com
llm:
  model: claude-sonnet-4-6
  api_key: test-key
`,
			wantErr: "llm.provider",
		},
		{
			name: "missing llm.model",
			content: `
url: https://example.com
llm:
  provider: anthropic
  api_key: test-key
`,
			wantErr: "llm.model",
		},
		{
			name: "missing llm.api_key",
			content: `
url: https://example.com
llm:
  provider: anthropic
  model: claude-sonnet-4-6
`,
			wantErr: "llm.api_key",
		},
		{
			name: "multiple missing fields",
			content: `
specs: ./specs
`,
			wantErr: "url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, `{ invalid yaml: [`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/vigil.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
