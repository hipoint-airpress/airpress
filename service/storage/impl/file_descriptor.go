package filestorageimpl

import (
	"path/filepath"
	"strconv"
	"time"
)

type fileDescriptor interface {
	getFullDirPath() string
	getFullPath() string
	getRelativePath() string
	getFileName() string
	setFileName(string)
	getExtensionName() string
	getShouldRename() shouldRename
	getRename() renameFunc
}

type shouldRename = func(relativePath string) (bool, error)

// renameFunc 是改名策略：返回要使用的文件名主体（不含扩展名）。
// 通过 withRename/withRenameURLOption 指定；不指定时沿用原始文件名基名。
type renameFunc = func() (string, error)

type localFileDescriptor struct {
	OriginalName    string
	Name            string
	Extension       string
	BasePath        string
	SubPath         string
	AutomaticRename bool
	ShouldRename    shouldRename
	Rename          renameFunc
	Suffix          string
}

func newLocalFileDescriptor(opts ...fdOption) (fileDescriptor, error) {
	fd := &localFileDescriptor{}
	for _, opt := range opts {
		opt(fd)
	}
	fd.BasePath = filepath.Clean(fd.BasePath)
	fd.SubPath = filepath.Clean(fd.SubPath)
	fd.OriginalName = filepath.Clean(fd.OriginalName)

	ext := filepath.Ext(fd.OriginalName)
	if ext != "" {
		// remove dot
		fd.Extension = ext[1:]
	}
	originalName := filepath.Base(fd.OriginalName)
	for i := len(originalName) - 1; i >= 0; i-- {
		if originalName[i] == '.' {
			fd.Name = originalName[0:i]
			break
		}
	}
	err := rename(fd)
	if err != nil {
		return fd, err
	}
	return fd, nil
}

// rename 依次执行两段逻辑：
//  1. 调用方用 withRename 指定的改名策略（本功能用它生成随机名），未指定则沿用原始文件名基名；
//  2. 用 shouldRename 对「改名之后」的相对路径做二次查重，判定重复则追加毫秒时间戳防重名。
//
// 两个钩子都不传时行为与改造前一致。
func rename(f fileDescriptor) error {
	if fn := f.getRename(); fn != nil {
		name, err := fn()
		if err != nil {
			return err
		}
		f.setFileName(name)
	}
	if f.getShouldRename() == nil {
		return nil
	}
	should, err := f.getShouldRename()(f.getRelativePath())
	if err != nil {
		return err
	}
	if !should {
		return nil
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	f.setFileName(f.getFileName() + "-" + timestamp)
	return nil
}

func (f *localFileDescriptor) getFullName() string {
	if f.Extension == "" {
		return f.Name + f.Suffix
	}
	return f.Name + f.Suffix + "." + f.Extension
}

func (f *localFileDescriptor) setFileName(name string) {
	f.Name = name
}

func (f *localFileDescriptor) getFullDirPath() string {
	return filepath.Join(f.BasePath, f.SubPath)
}

func (f *localFileDescriptor) getFullPath() string {
	return filepath.Join(f.BasePath, f.SubPath, f.getFullName())
}

func (f *localFileDescriptor) getRelativePath() string {
	return filepath.Join(f.SubPath, f.getFullName())
}

func (f *localFileDescriptor) getExtensionName() string {
	return f.Extension
}

func (f *localFileDescriptor) getFileName() string {
	return f.Name
}

func (f *localFileDescriptor) getShouldRename() shouldRename {
	return f.ShouldRename
}

func (f *localFileDescriptor) getRename() renameFunc {
	return f.Rename
}

type fdOption func(f *localFileDescriptor)

func withOriginalName(originalName string) fdOption {
	return func(f *localFileDescriptor) {
		f.OriginalName = originalName
	}
}

func withBasePath(basePath string) fdOption {
	return func(f *localFileDescriptor) {
		f.BasePath = basePath
	}
}

func withSubPath(subPath string) fdOption {
	return func(f *localFileDescriptor) {
		f.SubPath = subPath
	}
}

func withShouldRename(fn func(relativePath string) (bool, error)) fdOption {
	return func(f *localFileDescriptor) {
		f.ShouldRename = fn
	}
}

// withRename 指定改名策略；策略产出的路径仍会经 shouldRename 二次查重
func withRename(fn renameFunc) fdOption {
	return func(f *localFileDescriptor) {
		f.Rename = fn
	}
}

func withSuffix(suffix string) fdOption {
	return func(f *localFileDescriptor) {
		f.Suffix = suffix
	}
}
