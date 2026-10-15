package filestorageimpl

import (
	"path/filepath"
)

type fileDescriptor interface {
	getFullDirPath() string
	getFullPath() string
	getRelativePath() string
	getFileName() string
	setFileName(string)
	getExtensionName() string
	getRename() renameFunc
}

// renameFunc 是改名策略：返回要使用的文件名主体（不含扩展名）。
// 通过 withRename/withRenameURLOption 指定；不指定时沿用原始文件名基名。
// 策略必须返回非空基名：返回空串会拼出 ".jpg" 这类隐藏文件式路径。
type renameFunc = func() (string, error)

type localFileDescriptor struct {
	OriginalName    string
	Name            string
	Extension       string
	BasePath        string
	SubPath         string
	AutomaticRename bool
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

// rename 执行调用方用 withRename 指定的改名策略生成文件名主体（本功能用它生成随机名）；
// 未指定策略时沿用原始文件名基名。
//
// 冲突不在此处检测：唯一在写入前生效的防护是存储层的条件写
// （阿里云 oss.ForbidOverWrite、MinIO If-None-Match、本地 O_CREATE|O_EXCL，见各驱动的 Upload）。
func rename(f fileDescriptor) error {
	fn := f.getRename()
	if fn == nil {
		return nil
	}
	name, err := fn()
	if err != nil {
		return err
	}
	f.setFileName(name)
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

// withRename 指定改名策略；不指定时沿用原始文件名的基名
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
