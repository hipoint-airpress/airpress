package theme

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestListAllSkipsInvalidThemeDirs 固化健壮性契约：
// 主题目录根下若混入「不是合法主题」的子目录（如用户误加的空目录、缺少 theme.yaml 的目录），
// ListAll 必须跳过该目录而非整体报错，保证其余合法主题仍可正常加载。
//
// 复现：resources/template/theme 下存在一个空目录时，/admin/themes 及所有主题相关接口返回 500。
func TestListAllSkipsInvalidThemeDirs(t *testing.T) {
	root := t.TempDir()

	// 合法主题
	validDir := filepath.Join(root, "valid-theme")
	mustWriteFile(t, filepath.Join(validDir, "theme.yaml"), "id: valid-theme\nname: Valid\n")

	// 用户误加的空目录（无 theme.yaml）
	mustWriteEmptyDir(t, filepath.Join(root, "empty-dir"))

	s := &propertyScannerImpl{}
	themes, err := s.ListAll(context.Background(), root)
	if err != nil {
		t.Fatalf("ListAll 不应因无效目录失败: %v", err)
	}
	if len(themes) != 1 {
		t.Fatalf("期望仅加载 1 个合法主题，实际 %d 个", len(themes))
	}
	if themes[0].ID != "valid-theme" {
		t.Errorf("期望主题为 valid-theme，实际 %s", themes[0].ID)
	}
}

// TestListAllSkipsMissingThemeYamlDir 覆盖：子目录存在但缺少 theme.yaml 也应被跳过。
func TestListAllSkipsMissingThemeYamlDir(t *testing.T) {
	root := t.TempDir()

	validDir := filepath.Join(root, "valid-theme")
	mustWriteFile(t, filepath.Join(validDir, "theme.yaml"), "id: valid-theme\nname: Valid\n")

	// 有内容但缺少 theme.yaml 的目录
	strayDir := filepath.Join(root, "stray")
	mustWriteFile(t, filepath.Join(strayDir, "readme.txt"), "not a theme")

	s := &propertyScannerImpl{}
	themes, err := s.ListAll(context.Background(), root)
	if err != nil {
		t.Fatalf("ListAll 不应因缺少 theme.yaml 的目录失败: %v", err)
	}
	if len(themes) != 1 || themes[0].ID != "valid-theme" {
		t.Fatalf("期望仅加载 valid-theme，实际 %+v", themes)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteEmptyDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
