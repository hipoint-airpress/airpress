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

func newAliyunForTest() *Aliyun {
	return NewAliyun(aliyunOptionStub{values: map[string]string{
		property.AliOssEndpoint.KeyValue:     "oss-cn-hangzhou.aliyuncs.com",
		property.AliOssBucketName.KeyValue:   "mybucket",
		property.AliOssAccessKey.KeyValue:    "ak",
		property.AliOssAccessSecret.KeyValue: "sk",
		property.AliOssProtocol.KeyValue:     "https://",
	}})
}

// OSS 图片处理参数必须能原样往返：GetFilePath 先经 url.JoinPath 转义、再经 url.PathUnescape 还原。
// 这条断言是用 ?x-oss-process=... 形态落库的前提。
func TestAliyunGetFilePathKeepsProcessQuery(t *testing.T) {
	got, err := newAliyunForTest().GetFilePath(context.Background(), "img/2026/10/abc.png?x-oss-process=style/thumb")
	if err != nil {
		t.Fatalf("GetFilePath() err = %v", err)
	}
	want := "https://mybucket.oss-cn-hangzhou.aliyuncs.com/img/2026/10/abc.png?x-oss-process=style/thumb"
	if got != want {
		t.Errorf("GetFilePath()\n got = %q\nwant = %q", got, want)
	}
}
