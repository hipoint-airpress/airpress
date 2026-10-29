package assembler

import (
	"context"
	"errors"
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
)

// prefixedContentURLService 用前缀标记经过 Resolve 的字段，便于断言 Content
// 与 OriginalContent 都完成解析且未写反。
type prefixedContentURLService struct {
	err error
}

func (f prefixedContentURLService) Normalize(_ context.Context, content string) (string, error) {
	return content, nil
}

func (f prefixedContentURLService) Resolve(_ context.Context, content string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "resolved:" + content, nil
}

type stubBasePostService struct{ service.BasePostService }

func (stubBasePostService) BuildFullPath(context.Context, *entity.Post) (string, error) {
	return "/p", nil
}

func (stubBasePostService) GenerateSummary(context.Context, string) string { return "" }

type stubBaseCommentService struct{ service.BaseCommentService }

func (stubBaseCommentService) CountByContentID(context.Context, int32, consts.CommentType, consts.CommentStatus) (int64, error) {
	return 0, nil
}

func newTestBasePostAssembler(contentURLService service.ContentURLService) BasePostAssembler {
	return &basePostAssembler{
		BasePostService:    stubBasePostService{},
		BaseCommentService: stubBaseCommentService{},
		ContentURLService:  contentURLService,
	}
}

func TestConvertToDetailDTOResolvesBothContents(t *testing.T) {
	post := &entity.Post{
		ID:              1,
		FormatContent:   `<img src="upload/img/a.png">`,
		OriginalContent: `![a](upload/img/a.png)`,
	}

	got, err := newTestBasePostAssembler(prefixedContentURLService{}).ConvertToDetailDTO(context.Background(), post)
	if err != nil {
		t.Fatalf("ConvertToDetailDTO() error = %v", err)
	}
	if got.Content != `resolved:<img src="upload/img/a.png">` {
		t.Fatalf("Content = %q, want resolved content", got.Content)
	}
	if got.OriginalContent != `resolved:![a](upload/img/a.png)` {
		t.Fatalf("OriginalContent = %q, want resolved original content", got.OriginalContent)
	}
}

func TestConvertToDetailDTOReturnsResolveError(t *testing.T) {
	wantErr := errors.New("resolve failed")
	post := &entity.Post{ID: 1, FormatContent: "content", OriginalContent: "original"}

	got, err := newTestBasePostAssembler(prefixedContentURLService{err: wantErr}).ConvertToDetailDTO(context.Background(), post)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ConvertToDetailDTO() error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Fatalf("ConvertToDetailDTO() = %#v, want nil", got)
	}
}
