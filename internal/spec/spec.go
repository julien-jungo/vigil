package spec

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var md = goldmark.New(goldmark.WithExtensions(meta.Meta))

type Spec struct {
	ID    string
	Title string
	URL   string
	Tags  []string
	Steps []string
}

func Parse(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read spec: %w", err)
	}
	return parse(data)
}

func Load(dir string) ([]Spec, error) {
	var specs []Spec
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		spec, err := Parse(path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}
		spec.ID = strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		specs = append(specs, spec)
		return nil
	})
	return specs, err
}

func Filter(specs []Spec, tags []string) []Spec {
	if len(tags) == 0 {
		return specs
	}
	var result []Spec
	for _, spec := range specs {
		if hasAllTags(spec.Tags, tags) {
			result = append(result, spec)
		}
	}
	return result
}

func parse(src []byte) (Spec, error) {
	ctx := parser.NewContext()
	doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))

	rawMeta := meta.Get(ctx)

	var spec Spec
	spec.URL, _ = rawMeta["url"].(string)
	if tags, ok := rawMeta["tags"].([]any); ok {
		for _, t := range tags {
			if str, ok := t.(string); ok {
				spec.Tags = append(spec.Tags, str)
			}
		}
	}

	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *gast.Heading:
			if node.Level == 1 && spec.Title == "" {
				spec.Title = nodeText(node, src)
			}
		case *gast.ListItem:
			if list, ok := node.Parent().(*gast.List); ok && list.IsOrdered() {
				spec.Steps = append(spec.Steps, nodeText(node, src))
			}
		}
		return gast.WalkContinue, nil
	})

	return spec, nil
}

func nodeText(n gast.Node, src []byte) string {
	var sb strings.Builder
	_ = gast.Walk(n, func(child gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if t, ok := child.(*gast.Text); ok {
			sb.Write(t.Value(src))
			if t.SoftLineBreak() {
				sb.WriteByte(' ')
			}
		}
		return gast.WalkContinue, nil
	})
	return sb.String()
}

func hasAllTags(specTags, filterTags []string) bool {
	set := make(map[string]struct{}, len(specTags))
	for _, t := range specTags {
		set[t] = struct{}{}
	}
	for _, t := range filterTags {
		if _, ok := set[t]; !ok {
			return false
		}
	}
	return true
}
