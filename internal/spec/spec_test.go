package spec

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	withFrontmatter = `
---
url: /login
tags: [smoke, auth]
---

# Login with valid credentials

1. Navigate to the login page
2. Enter "testuser@example.com" in the email field
3. Click the login button
`[1:]

	withoutFrontmatter = `
# My spec

1. Do something
2. Verify result
`[1:]

	emptyFrontmatter = `
---
---

# Title

1. Step
`[1:]

	noTitle = `
1. Step one
`[1:]

	noSteps = `
# Title

Just some prose.
`[1:]

	multiLineStep = `
1. First line
   continued here
`[1:]
)

func TestParse_withFrontmatter(t *testing.T) {
	s, err := parse([]byte(withFrontmatter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.URL != "/login" {
		t.Errorf("URL = %q, want %q", s.URL, "/login")
	}
	if len(s.Tags) != 2 || s.Tags[0] != "smoke" || s.Tags[1] != "auth" {
		t.Errorf("Tags = %v, want [smoke auth]", s.Tags)
	}
	if s.Title != "Login with valid credentials" {
		t.Errorf("Title = %q, want %q", s.Title, "Login with valid credentials")
	}
	if len(s.Steps) != 3 {
		t.Errorf("Steps count = %d, want 3", len(s.Steps))
	}
	if s.Steps[1] != `Enter "testuser@example.com" in the email field` {
		t.Errorf("Steps[1] = %q", s.Steps[1])
	}
}

func TestParse_withoutFrontmatter(t *testing.T) {
	s, err := parse([]byte(withoutFrontmatter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.URL != "" {
		t.Errorf("URL = %q, want empty", s.URL)
	}
	if s.Tags != nil {
		t.Errorf("Tags = %v, want nil", s.Tags)
	}
	if s.Title != "My spec" {
		t.Errorf("Title = %q, want %q", s.Title, "My spec")
	}
	if len(s.Steps) != 2 {
		t.Errorf("Steps count = %d, want 2", len(s.Steps))
	}
}

func TestParse_emptyFrontmatter(t *testing.T) {
	s, err := parse([]byte(emptyFrontmatter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Title != "Title" {
		t.Errorf("Title = %q, want %q", s.Title, "Title")
	}
	if len(s.Steps) != 1 {
		t.Errorf("Steps count = %d, want 1", len(s.Steps))
	}
}

func TestParse_noTitle(t *testing.T) {
	s, err := parse([]byte(noTitle))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Title != "" {
		t.Errorf("Title = %q, want empty", s.Title)
	}
}

func TestParse_noSteps(t *testing.T) {
	s, err := parse([]byte(noSteps))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Steps != nil {
		t.Errorf("Steps = %v, want nil", s.Steps)
	}
}

func TestParse_multiLineStep(t *testing.T) {
	s, err := parse([]byte(multiLineStep))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Steps) != 1 {
		t.Fatalf("Steps count = %d, want 1", len(s.Steps))
	}
	if s.Steps[0] != "First line continued here" {
		t.Errorf("Steps[0] = %q", s.Steps[0])
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("login.md", withFrontmatter)
	write("sub/signup.md", withoutFrontmatter)
	write("notes.txt", "ignored")

	specs, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("loaded %d specs, want 2", len(specs))
	}

	ids := map[string]bool{}
	for _, s := range specs {
		ids[s.ID] = true
	}
	if !ids["login"] || !ids["sub/signup"] {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestLoad_emptyDir(t *testing.T) {
	specs, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs, got %d", len(specs))
	}
}

func TestLoad_missingDir(t *testing.T) {
	_, err := Load("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestFilter_byTag(t *testing.T) {
	specs := []Spec{
		{ID: "a", Tags: []string{"smoke", "auth"}},
		{ID: "b", Tags: []string{"smoke"}},
		{ID: "c", Tags: []string{"auth"}},
	}

	result := Filter(specs, []string{"smoke"})
	if len(result) != 2 {
		t.Errorf("got %d specs, want 2", len(result))
	}

	result = Filter(specs, []string{"smoke", "auth"})
	if len(result) != 1 || result[0].ID != "a" {
		t.Errorf("got %v, want [a]", result)
	}
}

func TestFilter_noTags(t *testing.T) {
	specs := []Spec{{ID: "a"}, {ID: "b"}}
	result := Filter(specs, nil)
	if len(result) != 2 {
		t.Errorf("got %d specs, want 2", len(result))
	}
}

func TestFilter_noMatch(t *testing.T) {
	specs := []Spec{{ID: "a", Tags: []string{"smoke"}}}
	result := Filter(specs, []string{"auth"})
	if len(result) != 0 {
		t.Errorf("got %d specs, want 0", len(result))
	}
}
