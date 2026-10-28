package admin

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/vo"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
)

type stubPostStatusService struct {
	service.PostService
}

func (s *stubPostStatusService) Page(ctx context.Context, query param.PostQuery) ([]*entity.Post, int64, error) {
	return []*entity.Post{{ID: 1, Title: "post", Type: consts.PostTypePost}}, 1, nil
}

type stubPostStatusAssembler struct {
	assembler.PostAssembler
	adminCalls  int
	simpleCalls int
}

func (s *stubPostStatusAssembler) ConvertToAdminListVO(ctx context.Context, posts []*entity.Post) ([]*vo.Post, error) {
	s.adminCalls++
	return []*vo.Post{{Post: dto.Post{PostMinimal: dto.PostMinimal{ID: 1, Title: "post"}}}}, nil
}

func (s *stubPostStatusAssembler) ConvertToSimpleDTO(ctx context.Context, post *entity.Post) (*dto.Post, error) {
	s.simpleCalls++
	return &dto.Post{PostMinimal: dto.PostMinimal{ID: post.ID, Title: post.Title}}, nil
}

func TestListPostsByStatusMoreParameter(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantAdmin  int
		wantSimple int
	}{
		{name: "缺省 more 返回完整列表", query: "", wantAdmin: 1},
		{name: "more=true 返回完整列表", query: "?more=true", wantAdmin: 1},
		{name: "more=false 返回精简列表", query: "?more=false", wantSimple: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("GET", "/api/admin/posts/status/1"+tc.query, nil)
			ctx.Params = gin.Params{{Key: "status", Value: "1"}}

			postAssembler := &stubPostStatusAssembler{}
			handler := &PostHandler{
				PostService:   &stubPostStatusService{},
				PostAssembler: postAssembler,
			}
			if _, err := handler.ListPostsByStatus(ctx); err != nil {
				t.Fatalf("ListPostsByStatus() error = %v", err)
			}
			if postAssembler.adminCalls != tc.wantAdmin || postAssembler.simpleCalls != tc.wantSimple {
				t.Fatalf("adminCalls = %d, simpleCalls = %d, want %d/%d",
					postAssembler.adminCalls, postAssembler.simpleCalls, tc.wantAdmin, tc.wantSimple)
			}
		})
	}
}
