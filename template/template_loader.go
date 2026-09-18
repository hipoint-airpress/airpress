package template

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// templateSuffix 模板文件扩展名
const templateSuffix = ".tmpl"

// mount 表示一个虚拟命名空间前缀到磁盘目录的映射
type mount struct {
	prefix string
	dir    string
}

// Loader 把多个磁盘根目录挂载到统一的虚拟命名空间，供 pongo2 按虚拟路径解析模板。
//
// 虚拟模板路径 == 原 html/template 中 {{define "..."}} 的名字，即文件相对路径去掉 .tmpl 后缀：
//
//	common/macro/head           -> <TemplateDir>/common/macro/head.tmpl
//	caicai_anatole/index        -> <ThemeDir>/anatole/index.tmpl
//
// 注意前缀取自主题 theme.yaml 的 id 字段，与主题目录名不一定相同，因此由外部显式 Mount。
type Loader struct {
	mu     sync.RWMutex
	mounts []mount
}

// Mount 注册 prefix -> dir 的映射。同一目录重复挂载时覆盖其前缀（主题切换时 id 可能变化）。
func (l *Loader) Mount(prefix, dir string) {
	prefix = cleanVirtualPath(prefix)
	dir = filepath.Clean(dir)
	if prefix == "" || dir == "" || dir == "." {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.mounts {
		if l.mounts[i].dir == dir {
			l.mounts[i].prefix = prefix
			return
		}
	}
	l.mounts = append(l.mounts, mount{prefix: prefix, dir: dir})
}

// PrefixForDir 返回目录已注册的前缀
func (l *Loader) PrefixForDir(dir string) (string, bool) {
	dir = filepath.Clean(dir)

	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, m := range l.mounts {
		if m.dir == dir {
			return m.prefix, true
		}
	}
	return "", false
}

// match 按最长前缀匹配规则定位虚拟路径所属的挂载点，并返回其相对路径。
// 无前导斜杠、路径分隔符统一为 "/"。
func (l *Loader) match(virtual string) (mount, string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var (
		best    mount
		bestRel string
		found   bool
	)
	for _, m := range l.mounts {
		var rel string
		switch {
		case virtual == m.prefix:
			rel = ""
		case strings.HasPrefix(virtual, m.prefix+"/"):
			rel = virtual[len(m.prefix)+1:]
		default:
			continue
		}
		if !found || len(m.prefix) > len(best.prefix) {
			best, bestRel, found = m, rel, true
		}
	}
	return best, bestRel, found
}

// Abs 实现 pongo2.TemplateLoader。
// base 为引用方模板的虚拟路径，name 为待解析的路径。
//
// 解析规则：
//  1. name 若命中已注册前缀，视为绝对虚拟路径直接返回；
//  2. 否则相对 base 所在目录拼接；
//  3. 拼接结果越出 base 所属前缀时钳回该前缀根下，避免借 ".." 逃逸。
func (l *Loader) Abs(base, name string) string {
	cleaned := cleanVirtualPath(name)
	if cleaned == "" {
		return ""
	}
	if _, _, ok := l.match(cleaned); ok {
		return cleaned
	}

	baseClean := cleanVirtualPath(base)
	if baseClean == "" {
		return cleaned
	}

	prefix := firstSegment(baseClean)
	if prefix == "" {
		return cleaned
	}
	joined := path.Join(path.Dir(baseClean), cleaned)
	if joined != prefix && !strings.HasPrefix(joined, prefix+"/") {
		return path.Join(prefix, cleaned)
	}
	return joined
}

// Get 实现 pongo2.TemplateLoader：把虚拟路径解析为磁盘文件并打开。
func (l *Loader) Get(virtualPath string) (io.Reader, error) {
	full, ok := l.Resolve(virtualPath)
	if !ok {
		return nil, os.ErrNotExist
	}
	return os.Open(full)
}

// Resolve 把虚拟路径解析为磁盘上的模板文件路径，文件不存在或越出挂载目录时返回 false。
func (l *Loader) Resolve(virtualPath string) (string, bool) {
	m, rel, ok := l.match(cleanVirtualPath(virtualPath))
	if !ok || rel == "" {
		return "", false
	}

	full := filepath.Join(m.dir, filepath.FromSlash(rel)) + templateSuffix

	// 双保险：即使前缀匹配被绕过，也不允许读到挂载目录之外
	relToMount, err := filepath.Rel(m.dir, full)
	if err != nil || relToMount == ".." || strings.HasPrefix(relToMount, ".."+string(filepath.Separator)) {
		return "", false
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", false
	}
	return full, true
}

// Walk 遍历所有挂载目录下的模板文件，回调参数为虚拟路径（不含 .tmpl 后缀）。
func (l *Loader) Walk(fn func(virtualPath string) error) error {
	l.mu.RLock()
	mounts := make([]mount, len(l.mounts))
	copy(mounts, l.mounts)
	l.mu.RUnlock()

	for _, m := range mounts {
		err := filepath.Walk(m.dir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(p) != templateSuffix {
				return nil
			}
			rel, err := filepath.Rel(m.dir, p)
			if err != nil {
				return err
			}
			virtual := path.Join(m.prefix, filepath.ToSlash(rel))
			return fn(strings.TrimSuffix(virtual, templateSuffix))
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// cleanVirtualPath 归一化虚拟路径：统一分隔符、去掉前导斜杠、用 path.Clean 钳掉 ".."，
// 使 "a/../../etc/passwd" 之类的输入退化为 "etc/passwd" 而无法逃出命名空间。
func cleanVirtualPath(name string) string {
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Clean("/" + name)
	return strings.TrimPrefix(name, "/")
}

// firstSegment 返回虚拟路径的首段
func firstSegment(p string) string {
	if i := strings.Index(p, "/"); i >= 0 {
		return p[:i]
	}
	return p
}
