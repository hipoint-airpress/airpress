package impl

import (
	"context"
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service/storage"
)

func TestExtractContentTargets(t *testing.T) {
	content := `![a](upload/a.png)
<img src="/upload/b.png">
[c](https://example.com/c.png "title")
[d](upload/a.png)`
	// 实现先扫描 HTML 属性、再扫描 Markdown，去重顺序与功能无关。
	want := []string{"/upload/b.png", "upload/a.png", "https://example.com/c.png"}
	if got := extractContentTargets(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", got, want)
	}
}

func TestReplaceContentTargets(t *testing.T) {
	content := `![a](old/a.png)
<img src="old/b.png"><img src='old/c.png'>`
	replacements := map[string]string{
		"old/a.png": "new/a.png",
		"old/b.png": "new/b.png",
		"old/c.png": "new/c.png",
	}
	want := `![a](new/a.png)
<img src="new/b.png"><img src='new/c.png'>`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestExtractContentTargetsSingleQuotedHTML 覆盖单引号 HTML 属性（extract 的 match[5] 分支）。
func TestExtractContentTargetsSingleQuotedHTML(t *testing.T) {
	content := `<a href='sq.png'>x</a>`
	want := []string{"sq.png"}
	if got := extractContentTargets(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", got, want)
	}
}

func TestExtractContentTargetsEmptyContent(t *testing.T) {
	if got := extractContentTargets(""); len(got) != 0 {
		t.Fatalf("extractContentTargets(\"\") = %#v, want empty", got)
	}
}

// TestReplaceContentTargetsNoMatch 验证未命中项（HTML 与 Markdown）原样返回。
func TestReplaceContentTargetsNoMatch(t *testing.T) {
	content := `<img src="miss.png"><a href='miss2.png'>x</a>[y](miss3.png)`
	replacements := map[string]string{"other.png": "new.png"}
	if got := replaceContentTargets(content, replacements); got != content {
		t.Fatalf("replaceContentTargets() = %q, want unchanged %q", got, content)
	}
}

// TestReplaceContentTargetsTrimsWhitespace 验证属性值首尾空白与提取时的裁剪一致。
func TestReplaceContentTargetsTrimsWhitespace(t *testing.T) {
	content := `<img src=" old.png ">`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<img src="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestExtractAndReplaceRoundTripTrimmedHTML 验证提取结果可直接用于替换。
func TestExtractAndReplaceRoundTripTrimmedHTML(t *testing.T) {
	content := `<img src=" old.png ">`
	targets := extractContentTargets(content)
	wantTargets := []string{"old.png"}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", targets, wantTargets)
	}
	replacements := make(map[string]string, len(targets))
	for _, target := range targets {
		replacements[target] = "new.png"
	}
	want := `<img src="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestExtractAndReplaceRoundTripTrimmedMarkdown 记录带空白 Markdown 目标的现状：
// 正则的前后捕获组会消费首尾空白，故提取结果为 old.png，替换后空白保留。
func TestExtractAndReplaceRoundTripTrimmedMarkdown(t *testing.T) {
	content := `![a]( old.png )`
	targets := extractContentTargets(content)
	wantTargets := []string{"old.png"}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", targets, wantTargets)
	}
	replacements := map[string]string{"old.png": "new.png"}
	want := `![a]( new.png )`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsPreservesDataSrcAttribute 验证 data-src 前缀不被破坏。
func TestReplaceContentTargetsPreservesDataSrcAttribute(t *testing.T) {
	content := `<img data-src="old.png">`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<img data-src="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsPreservesMarkdownTitle 验证 Markdown title 保留。
func TestReplaceContentTargetsPreservesMarkdownTitle(t *testing.T) {
	content := `[c](old.png "title")`
	replacements := map[string]string{"old.png": "new.png"}
	want := `[c](new.png "title")`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsIgnoresPlainText 验证正文纯文本中的同名 URL 不被替换。
func TestReplaceContentTargetsIgnoresPlainText(t *testing.T) {
	content := `正文提到 old.png，但这不是链接。`
	replacements := map[string]string{"old.png": "new.png"}
	if got := replaceContentTargets(content, replacements); got != content {
		t.Fatalf("replaceContentTargets() = %q, want unchanged %q", got, content)
	}
}

// TestReplaceContentTargetsEscapesHTMLEntities 验证替换值在 HTML 属性中的转义。
func TestReplaceContentTargetsEscapesHTMLEntities(t *testing.T) {
	content := `<img src="old.png">`
	replacements := map[string]string{"old.png": "new.png?a=1&b=2"}
	want := `<img src="new.png?a=1&amp;b=2">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsUppercaseAttribute 验证大写属性名可被替换。
func TestReplaceContentTargetsUppercaseAttribute(t *testing.T) {
	content := `<img SRC="old.png">`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<img SRC="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsHref 验证 href 属性可被替换。
func TestReplaceContentTargetsHref(t *testing.T) {
	content := `<a href="old.png">link</a>`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<a href="new.png">link</a>`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

func TestNormalizeContentURLs(t *testing.T) {
	db := setupPostAttachmentDB(t)
	attachments := []*entity.Attachment{
		{Name: "local", Path: "upload/img/local.png", FileKey: "upload/img/local.png", Type: consts.AttachmentTypeLocal},
		{Name: "oss", Path: "upload/img/oss.png?x-oss-process=style/thumb", FileKey: "upload/img/oss.png", Type: consts.AttachmentTypeAliOSS},
		{Name: "minio", Path: "upload/img/minio.png", FileKey: "upload/img/minio.png", Type: consts.AttachmentTypeMinIO},
	}
	if err := db.Create(&attachments).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	content := `![local](https://blog.example.com/upload/img/local.png)
<img src="/upload/img/local.png">
![oss](https://cdn.example.com/upload/img/oss.png?x-oss-process=style/thumb)
![minio](https://minio.example.com/bucket/upload/img/minio.png)
![external](https://other.example.com/remote.png)
[internal](/archives/hello)`
	want := `![local](upload/img/local.png)
<img src="upload/img/local.png">
![oss](upload/img/oss.png?x-oss-process=style/thumb)
![minio](upload/img/minio.png)
![external](https://other.example.com/remote.png)
[internal](/archives/hello)`

	got, err := NormalizeContentURLs(context.Background(), content)
	if err != nil {
		t.Fatalf("NormalizeContentURLs() error = %v", err)
	}
	if got != want {
		t.Fatalf("NormalizeContentURLs() =\n%s\nwant:\n%s", got, want)
	}

	again, err := NormalizeContentURLs(context.Background(), got)
	if err != nil {
		t.Fatalf("NormalizeContentURLs() second call error = %v", err)
	}
	if again != got {
		t.Fatalf("NormalizeContentURLs() is not idempotent:\n%s", again)
	}
}

func TestNormalizeContentURLsEdgeCases(t *testing.T) {
	db := setupPostAttachmentDB(t)
	attachments := []*entity.Attachment{
		{Name: "pdf", Path: "upload/doc/a.pdf", FileKey: "upload/doc/a.pdf"},
		{Name: "entity-query", Path: "upload/img/a.png?x=1&y=2", FileKey: "upload/img/a.png"},
		{Name: "chinese", Path: "upload/img/中文.png", FileKey: "upload/img/中文.png"},
		{Name: "oss", Path: "upload/img/oss.png?x-oss-process=style/thumb", FileKey: "upload/img/oss.png"},
		{Name: "manual-local", Path: "upload/doc/manual.pdf", FileKey: "upload/doc/manual.pdf"},
		{Name: "manual-oss", Path: "upload/doc/manual.pdf?x-oss-process=style/thumb", FileKey: "upload/doc/manual.pdf"},
		{Name: "local", Path: "upload/img/local.png", FileKey: "upload/img/local.png"},
	}
	if err := db.Create(&attachments).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "preserve fragment",
			content: `<a href="https://blog.example.com/upload/doc/a.pdf#page=5">x</a>`,
			want:    `<a href="upload/doc/a.pdf#page=5">x</a>`,
		},
		{
			name:    "html entity query",
			content: `<img src="https://blog.example.com/upload/img/a.png?x=1&amp;y=2">`,
			want:    `<img src="upload/img/a.png?x=1&amp;y=2">`,
		},
		{
			name:    "encoded chinese path",
			content: `![a](https://blog.example.com/upload/img/%E4%B8%AD%E6%96%87.png)`,
			want:    `![a](upload/img/中文.png)`,
		},
		{
			name:    "file key fallback",
			content: `![a](https://cdn.example.com/upload/img/oss.png)`,
			want:    `![a](upload/img/oss.png?x-oss-process=style/thumb)`,
		},
		{
			name:    "path wins over file key",
			content: `![a](https://blog.example.com/upload/doc/manual.pdf)`,
			want:    `![a](upload/doc/manual.pdf)`,
		},
		{
			name:    "multiple targets same attachment",
			content: `![a](https://blog.example.com/upload/img/local.png)<img src="/upload/img/local.png">`,
			want:    `![a](upload/img/local.png)<img src="upload/img/local.png">`,
		},
		{
			name:    "uppercase scheme",
			content: `![a](HTTPS://blog.example.com/upload/img/local.png)`,
			want:    `![a](upload/img/local.png)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeContentURLs(context.Background(), tc.content)
			if err != nil {
				t.Fatalf("NormalizeContentURLs() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeContentURLs() = %q, want %q", got, tc.want)
			}
		})
	}
}

type fakeFileStorage struct {
	attachmentType consts.AttachmentType
	baseURL        string
}

func (f *fakeFileStorage) Upload(context.Context, *multipart.FileHeader) (*dto.AttachmentDTO, error) {
	return nil, nil
}

func (f *fakeFileStorage) Delete(context.Context, string) error { return nil }

func (f *fakeFileStorage) GetAttachmentType() consts.AttachmentType { return f.attachmentType }

func (f *fakeFileStorage) GetFilePath(_ context.Context, relativePath string) (string, error) {
	return f.baseURL + "/" + relativePath, nil
}

type fakeFileStorageComposite map[consts.AttachmentType]storage.FileStorage

func (f fakeFileStorageComposite) GetFileStorage(storageType consts.AttachmentType) storage.FileStorage {
	return f[storageType]
}

func TestContentURLServiceResolve(t *testing.T) {
	db := setupPostAttachmentDB(t)
	attachments := []*entity.Attachment{
		{Name: "local", Path: "upload/img/local.png", FileKey: "upload/img/local.png", Type: consts.AttachmentTypeLocal},
		{Name: "oss", Path: "upload/img/oss.png?x-oss-process=style/thumb", FileKey: "upload/img/oss.png", Type: consts.AttachmentTypeAliOSS},
		{Name: "minio", Path: "upload/img/minio.png", FileKey: "upload/img/minio.png", Type: consts.AttachmentTypeMinIO},
	}
	if err := db.Create(&attachments).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal:  &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
		consts.AttachmentTypeAliOSS: &fakeFileStorage{attachmentType: consts.AttachmentTypeAliOSS, baseURL: "https://cdn.example.com"},
		consts.AttachmentTypeMinIO:  &fakeFileStorage{attachmentType: consts.AttachmentTypeMinIO, baseURL: "https://minio.example.com/bucket"},
	}
	svc := &contentURLServiceImpl{FileStorageComposite: composite}

	content := `![local](upload/img/local.png)
![oss](upload/img/oss.png?x-oss-process=style/thumb)
![minio](upload/img/minio.png)
![external](https://other.example.com/remote.png)`
	want := `![local](https://blog.example.com/upload/img/local.png)
![oss](https://cdn.example.com/upload/img/oss.png?x-oss-process=style/thumb)
![minio](https://minio.example.com/bucket/upload/img/minio.png)
![external](https://other.example.com/remote.png)`

	got, err := svc.Resolve(context.Background(), content)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != want {
		t.Fatalf("Resolve() =\n%s\nwant:\n%s", got, want)
	}

	again, err := svc.Resolve(context.Background(), got)
	if err != nil {
		t.Fatalf("Resolve() second call error = %v", err)
	}
	if again != got {
		t.Fatalf("Resolve() is not idempotent:\n%s", again)
	}
}

func TestContentURLServiceResolvePreservesFragment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.Create(&entity.Attachment{
		Name: "doc", Path: "upload/doc/a.pdf", FileKey: "upload/doc/a.pdf", Type: consts.AttachmentTypeLocal,
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal: &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
	}
	svc := &contentURLServiceImpl{FileStorageComposite: composite}

	got, err := svc.Resolve(context.Background(), `<a href="upload/doc/a.pdf#page=5">doc</a>`)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := `<a href="https://blog.example.com/upload/doc/a.pdf#page=5">doc</a>`
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

// TestContentURLServiceResolvePreservesEscapedFragment 验证 fragment 的百分号编码不被解码。
func TestContentURLServiceResolvePreservesEscapedFragment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.Create(&entity.Attachment{
		Name: "doc", Path: "upload/doc/a.pdf", FileKey: "upload/doc/a.pdf", Type: consts.AttachmentTypeLocal,
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal: &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
	}
	svc := &contentURLServiceImpl{FileStorageComposite: composite}

	content := `<a href="upload/doc/a.pdf#nameddest=Chapter%201">doc</a>
[x](upload/doc/a.pdf#nameddest=Chapter%201)`
	want := `<a href="https://blog.example.com/upload/doc/a.pdf#nameddest=Chapter%201">doc</a>
[x](https://blog.example.com/upload/doc/a.pdf#nameddest=Chapter%201)`

	got, err := svc.Resolve(context.Background(), content)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != want {
		t.Fatalf("Resolve() =\n%s\nwant:\n%s", got, want)
	}
}

// TestContentURLServiceResolveIgnoresPlainRelativeLinks 验证未匹配附件的站内相对链接保持原样。
func TestContentURLServiceResolveIgnoresPlainRelativeLinks(t *testing.T) {
	setupPostAttachmentDB(t)
	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal: &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
	}
	svc := &contentURLServiceImpl{FileStorageComposite: composite}

	content := `[about](about.html)
[a](archives/foo)
[a](?page=2)`
	got, err := svc.Resolve(context.Background(), content)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != content {
		t.Fatalf("Resolve() = %q, want unchanged %q", got, content)
	}
}

// TestContentURLServiceResolveSkipsUnsupportedType 验证不支持的存储类型不 panic 且保持原样。
func TestContentURLServiceResolveSkipsUnsupportedType(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.Create(&entity.Attachment{
		Name: "up", Path: "upload/img/up.png", FileKey: "upload/img/up.png", Type: consts.AttachmentTypeUpOSS,
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal: &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
	}
	svc := &contentURLServiceImpl{FileStorageComposite: composite}

	content := `![up](upload/img/up.png)`
	got, err := svc.Resolve(context.Background(), content)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != content {
		t.Fatalf("Resolve() = %q, want unchanged %q", got, content)
	}
}

// TestContentURLServiceResolveDecodesEscapedPath 验证百分号编码的路径可匹配并解析。
func TestContentURLServiceResolveDecodesEscapedPath(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.Create(&entity.Attachment{
		Name: "chinese", Path: "upload/img/中文.png", FileKey: "upload/img/中文.png", Type: consts.AttachmentTypeLocal,
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal: &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
	}
	svc := &contentURLServiceImpl{FileStorageComposite: composite}

	got, err := svc.Resolve(context.Background(), `![a](upload/img/%E4%B8%AD%E6%96%87.png)`)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := `![a](https://blog.example.com/upload/img/中文.png)`
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

// TestContentURLServiceResolveNilComposite 验证缺少存储依赖时返回错误而非 panic。
func TestContentURLServiceResolveNilComposite(t *testing.T) {
	svc := &contentURLServiceImpl{}
	content := `![a](upload/img/a.png)`
	got, err := svc.Resolve(context.Background(), content)
	if err == nil {
		t.Fatalf("Resolve() error = nil, want error")
	}
	if got != content {
		t.Fatalf("Resolve() = %q, want unchanged %q", got, content)
	}
}
