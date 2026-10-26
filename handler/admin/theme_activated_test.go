package admin

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type stubActivatedThemeService struct {
	service.ThemeService
	theme *dto.ThemeProperty
	err   error
}

func (s *stubActivatedThemeService) GetThemeByID(ctx context.Context, themeID string) (*dto.ThemeProperty, error) {
	return s.theme, s.err
}

type stubActivatedThemeOptionService struct {
	service.OptionService
	themeID string
}

func (s *stubActivatedThemeOptionService) GetActivatedThemeID(ctx context.Context) (string, error) {
	return s.themeID, nil
}

func TestGetActivatedThemeFallsBackWhenThemeMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	handler := NewThemeHandler(
		&stubActivatedThemeService{
			err: xerr.WithStatus(nil, xerr.StatusBadRequest).WithMsg("theme does not exist"),
		},
		&stubActivatedThemeOptionService{themeID: "missing"},
	)

	result, err := handler.GetActivatedTheme(ctx)
	if err != nil {
		t.Fatalf("GetActivatedTheme() error = %v", err)
	}
	themeProperty, ok := result.(*dto.ThemeProperty)
	if !ok {
		t.Fatalf("result type = %T", result)
	}
	if themeProperty.ID != "missing" {
		t.Fatalf("theme ID = %q, want missing", themeProperty.ID)
	}
}

func TestGetActivatedThemePropagatesOtherErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	handler := NewThemeHandler(
		&stubActivatedThemeService{
			err: xerr.WithStatus(nil, xerr.StatusInternalServerError).WithMsg("boom"),
		},
		&stubActivatedThemeOptionService{themeID: "broken"},
	)

	if _, err := handler.GetActivatedTheme(ctx); err == nil {
		t.Fatal("non-400 theme errors should propagate")
	}
}
