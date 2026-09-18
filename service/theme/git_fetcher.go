package theme

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"go.uber.org/fx"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type gitThemeFetcherImpl struct {
	fx.Out
	PropertyScanner PropertyScanner
}

func (g gitThemeFetcherImpl) FetchTheme(ctx context.Context, file interface{}) (*dto.ThemeProperty, error) {
	gitURL := file.(string)
	splits := strings.Split(gitURL, "/")
	lastSplit := splits[len(splits)-1]
	tempDir := os.TempDir()

	themeDirName := lastSplit
	tmpThemeDir := filepath.Join(tempDir, themeDirName)
	if util.FileIsExisted(tmpThemeDir) {
		err := os.RemoveAll(tmpThemeDir)
		if err != nil {
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("delete tmp theme directory err")
		}
	}
	_, err := git.PlainClone(filepath.Join(tempDir, themeDirName), false, &git.CloneOptions{
		URL: gitURL,
	})
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(err.Error())
	}
	themeProperty, err := g.PropertyScanner.ReadThemeProperty(ctx, filepath.Join(tempDir, themeDirName))
	if err != nil {
		return nil, err
	}
	return themeProperty, nil
}

func NewGitThemeFetcher(propertyScanner PropertyScanner) ThemeFetcher {
	return &gitThemeFetcherImpl{
		PropertyScanner: propertyScanner,
	}
}
