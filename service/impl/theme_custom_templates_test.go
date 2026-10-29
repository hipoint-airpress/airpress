package impl

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/service/theme"
)

type stubTemplatePropertyScanner struct {
	theme.PropertyScanner
	property *dto.ThemeProperty
}

func (s *stubTemplatePropertyScanner) GetThemeByThemeID(ctx context.Context, themeID string) (*dto.ThemeProperty, error) {
	return s.property, nil
}

func TestListCustomTemplatesMissingThemeReturnsEmpty(t *testing.T) {
	svc := &themeServiceImpl{
		PropertyScanner: &stubTemplatePropertyScanner{},
	}

	templates, err := svc.ListCustomTemplates(context.Background(), "missing", "sheet_")
	if err != nil {
		t.Fatalf("ListCustomTemplates() error = %v", err)
	}
	if len(templates) != 0 {
		t.Fatalf("templates = %v, want empty", templates)
	}
}

func TestListCustomTemplatesMissingDirectoryReturnsEmpty(t *testing.T) {
	svc := &themeServiceImpl{
		PropertyScanner: &stubTemplatePropertyScanner{
			property: &dto.ThemeProperty{ID: "test", ThemePath: filepath.Join(t.TempDir(), "missing")},
		},
	}

	templates, err := svc.ListCustomTemplates(context.Background(), "test", "sheet_")
	if err != nil {
		t.Fatalf("ListCustomTemplates() error = %v", err)
	}
	if len(templates) != 0 {
		t.Fatalf("templates = %v, want empty", templates)
	}
}

func TestListCustomTemplatesFiltersPrefix(t *testing.T) {
	dir := t.TempDir()
	files := []string{"sheet_a.tmpl", "sheet_b.tmpl", "post_c.tmpl", "readme.txt"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	svc := &themeServiceImpl{
		PropertyScanner: &stubTemplatePropertyScanner{
			property: &dto.ThemeProperty{ID: "test", ThemePath: dir},
		},
	}

	templates, err := svc.ListCustomTemplates(context.Background(), "test", "sheet_")
	if err != nil {
		t.Fatalf("ListCustomTemplates() error = %v", err)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(templates, want) {
		t.Fatalf("templates = %v, want %v", templates, want)
	}
}
