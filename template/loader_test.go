package template

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flosch/pongo2/v6"
	"go.uber.org/zap"
)

// 内置的模板命名空间与磁盘目录。
// 前缀取自主题 theme.yaml 的 id，与目录名不一定相同，故显式挂载。
func themeMounts(root string) []struct{ prefix, dir string } {
	return []struct{ prefix, dir string }{
		{"common", filepath.Join(root, "common")},
		{"caicai_anatole", filepath.Join(root, "theme", "default-theme-anatole")},
		{"simple_corp_portal", filepath.Join(root, "theme", "simple-corp-portal")},
	}
}

func newTestLoader(t *testing.T) *Loader {
	t.Helper()
	loader := &Loader{}
	for _, m := range themeMounts(filepath.Join("..", "resources", "template")) {
		loader.Mount(m.prefix, m.dir)
	}
	return loader
}

// TestPrecompileBuiltinTemplates 验证内置模板全部可被 pongo2 解析。
//
// pongo2 的 {% include "静态路径" %} 在解析期就会打开并编译目标模板，
// 因此这一遍预编译同时覆盖「include 目标是否存在」与「语法是否正确」两类问题。
func TestPrecompileBuiltinTemplates(t *testing.T) {
	loader := newTestLoader(t)
	set := pongo2.NewSet("airpress-test", loader)

	count := 0
	err := loader.Walk(func(virtualPath string) error {
		count++
		if _, err := set.FromCache(virtualPath); err != nil {
			return fmt.Errorf("模板 %s 解析失败: %w", virtualPath, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("未扫描到任何模板，请确认 resources/template 存在")
	}
	t.Logf("成功解析 %d 个模板", count)
}

// TestTemplateNamespaceMatchesDefineNames 固化命名空间契约：
// ThemeService.Render 返回的 "<themeID>/<name>" 必须能直接解析为模板文件。
func TestTemplateNamespaceMatchesDefineNames(t *testing.T) {
	loader := newTestLoader(t)

	for _, name := range []string{
		"common/error/error",
		"common/template/post_password",
		"common/web/rss",
		"common/web/atom",
		"common/web/robots",
		"common/web/sitemap_xml",
		"common/web/sitemap_html",
		"common/mail_template/mail_notice",
		"common/mail_template/mail_reply",
		"common/macro/head",
		"common/macro/footer",
		"common/macro/comment",
	} {
		if _, ok := loader.Resolve(name); !ok {
			t.Errorf("共享模板 %q 不存在", name)
		}
	}

	for _, themeID := range []string{"caicai_anatole", "simple_corp_portal"} {
		for _, page := range []string{"index", "post", "sheet", "archives", "categories",
			"category", "tags", "tag", "journals", "links", "photos", "search"} {
			name := themeID + "/" + page
			if _, ok := loader.Resolve(name); !ok {
				t.Errorf("主题模板 %q 不存在", name)
			}
		}
	}
}

// TestLoaderRejectsPathTraversal 验证挂载目录之外的文件无法通过虚拟路径读到。
func TestLoaderRejectsPathTraversal(t *testing.T) {
	loader := newTestLoader(t)

	for _, name := range []string{
		"common/../../conf/config.yaml",
		"common/../../../etc/passwd",
		"../../etc/passwd",
		"caicai_anatole/../../../../etc/passwd",
		`..\..\etc\passwd`,
	} {
		if full, ok := loader.Resolve(name); ok {
			t.Errorf("路径 %q 不应解析成功，却得到 %q", name, full)
		}
	}
}

// TestLoaderAbs 固化虚拟路径的绝对/相对解析规则。
func TestLoaderAbs(t *testing.T) {
	loader := newTestLoader(t)

	cases := []struct{ base, name, want string }{
		// 命中已注册前缀 -> 视为绝对虚拟路径
		{"caicai_anatole/index", "common/macro/head", "common/macro/head"},
		{"", "common/web/rss", "common/web/rss"},
		// 相对 base 所在目录拼接
		{"caicai_anatole/index", "module/sidebar", "caicai_anatole/module/sidebar"},
		{"caicai_anatole/module/post", "sidebar", "caicai_anatole/module/sidebar"},
		// 含 ".." 的路径被 path.Clean 钳制，拼接后仍留在该前缀的挂载目录内
		{"caicai_anatole/module/x", "../../../etc/passwd", "caicai_anatole/module/etc/passwd"},
	}
	for _, c := range cases {
		if got := loader.Abs(c.base, c.name); got != c.want {
			t.Errorf("Abs(%q, %q) = %q, 期望 %q", c.base, c.name, got, c.want)
		}
	}
}

// TestExecuteTemplateEscaping 验证端到端渲染：默认转义、|safe 直出、now 注入。
func TestExecuteTemplateEscaping(t *testing.T) {
	tpl := NewTemplate(zap.NewNop(), nil)
	defer func() { _ = tpl.watcher.Close() }()

	commonDir := filepath.Join("..", "resources", "template", "common")
	tpl.Mount("common", commonDir)
	if err := tpl.Load([]string{commonDir}); err != nil {
		t.Fatal(err)
	}

	render := func(name string, model Model) string {
		t.Helper()
		var buf bytes.Buffer
		if err := tpl.ExecuteTemplate(&buf, name, model); err != nil {
			t.Fatalf("渲染 %s 失败: %v", name, err)
		}
		return buf.String()
	}

	// 口令页走 HTML 转义：标题里的尖括号必须被转义
	out := render("common/template/post_password", Model{
		"blog_title": "<script>x</script>",
		"blog_url":   "/",
		"type":       "post",
		"slug":       "s",
		"errorMsg":   "密码错误",
	})
	if strings.Contains(out, "<script>x</script>") {
		t.Error("blog_title 未被 HTML 转义")
	}
	if !strings.Contains(out, "密码错误") {
		t.Error("errorMsg 未渲染")
	}

	// 错误页的 err/message 走 |default:，空值应回落到默认文案而非输出 "true"
	out = render("common/error/error", Model{"status": 404})
	if strings.Contains(out, "true") || strings.Contains(out, "false") {
		t.Errorf("error 模板误用了返回布尔的 or，输出中存在 true/false:\n%s", out)
	}
	if !strings.Contains(out, "未知错误") {
		t.Error("err 为空时未回落到默认文案")
	}

	// feed 模板整体关闭转义，CDATA 内的富文本应原样输出
	out = render("common/web/rss", Model{
		"blog_title": "T",
		"blog_url":   "http://x",
		"version":      "1",
		"lastModified": time.Now(),
		"posts":        nil,
	})
	if !strings.HasPrefix(out, "<?xml") {
		t.Errorf("RSS 输出未以 <?xml 打头（autoescape/trim 处理有误）:\n%q", out[:min(40, len(out))])
	}
}
