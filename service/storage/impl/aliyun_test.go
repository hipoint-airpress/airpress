package filestorageimpl

import (
	"context"
	"testing"

	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
)

// aliyunOptionStub 只提供 getAliOSSClient 需要的属性读取，其余方法用不到（嵌入接口后未实现即不可达）。
type aliyunOptionStub struct {
	service.OptionService
	values map[string]string
}

func (s aliyunOptionStub) GetOrByDefaultWithErr(ctx context.Context, p property.Property, defaultValue interface{}) (interface{}, error) {
	if v, ok := s.values[p.KeyValue]; ok {
		return v, nil
	}
	return defaultValue, nil
}

// newAliyunForTest 用给定的属性覆盖项构造 Aliyun；未提供的属性回落到其 DefaultValue
func newAliyunForTest(overrides map[string]string) *Aliyun {
	values := map[string]string{
		property.AliOssEndpoint.KeyValue:     "oss-cn-hangzhou.aliyuncs.com",
		property.AliOssBucketName.KeyValue:   "mybucket",
		property.AliOssAccessKey.KeyValue:    "ak",
		property.AliOssAccessSecret.KeyValue: "sk",
		property.AliOssProtocol.KeyValue:     "https://",
	}
	for k, v := range overrides {
		values[k] = v
	}
	return NewAliyun(aliyunOptionStub{values: values})
}

// OSS 图片处理参数必须能原样往返：GetFilePath 先经 url.JoinPath 转义、再经 url.PathUnescape 还原。
// 这条断言是用 ?x-oss-process=... 形态落库的前提。
func TestAliyunGetFilePathKeepsProcessQuery(t *testing.T) {
	got, err := newAliyunForTest(nil).GetFilePath(context.Background(), "img/2026/10/abc.png?x-oss-process=style/thumb")
	if err != nil {
		t.Fatalf("GetFilePath() err = %v", err)
	}
	want := "https://mybucket.oss-cn-hangzhou.aliyuncs.com/img/2026/10/abc.png?x-oss-process=style/thumb"
	if got != want {
		t.Errorf("GetFilePath()\n got = %q\nwant = %q", got, want)
	}
}

func TestOSSProcessQuery(t *testing.T) {
	cases := []struct {
		name string
		rule string
		want string
	}{
		{"含斜杠视为完整处理表达式", "image/resize,w_256", "?x-oss-process=image/resize,w_256"},
		{"不含斜杠视为控制台样式名", "thumb", "?x-oss-process=style/thumb"},
		{"空值不加任何后缀", "", ""},
		{"纯空格等同空值", "   ", ""},
		{"前后空格被修剪", " image/sharpen,50 ", "?x-oss-process=image/sharpen,50"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ossProcessQuery(c.rule); got != c.want {
				t.Errorf("ossProcessQuery(%q) = %q, want %q", c.rule, got, c.want)
			}
		})
	}
}

// 配置值经 getAliOSSClient 解析后，字段存的必须是可直接拼接的最终查询串。
// 缺少这条断言时，删掉 ossProcessQuery 调用（退回原始配置值拼接）不会被任何测试发现。
func TestAliyunClientStoresParsedStyleQuery(t *testing.T) {
	cases := []struct {
		name               string
		overrides          map[string]string
		wantStyle          string
		wantThumbnailStyle string
	}{
		{
			name: "非空规则解析为带前缀的查询串",
			overrides: map[string]string{
				property.AliOssStyleRule.KeyValue:          "image/resize,w_800",
				property.AliOssThumbnailStyleRule.KeyValue: "thumb",
			},
			wantStyle:          "?x-oss-process=image/resize,w_800",
			wantThumbnailStyle: "?x-oss-process=style/thumb",
		},
		{
			name:               "两个规则都不配时字段为空串",
			overrides:          nil,
			wantStyle:          "",
			wantThumbnailStyle: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, err := newAliyunForTest(c.overrides).getAliOSSClient(context.Background())
			if err != nil {
				t.Fatalf("getAliOSSClient() err = %v", err)
			}
			if client.Style != c.wantStyle {
				t.Errorf("Style = %q, want %q", client.Style, c.wantStyle)
			}
			if client.ThumbnailStyle != c.wantThumbnailStyle {
				t.Errorf("ThumbnailStyle = %q, want %q", client.ThumbnailStyle, c.wantThumbnailStyle)
			}
		})
	}
}
