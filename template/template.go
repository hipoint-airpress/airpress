package template

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

// Template 基于 pongo2 的模板渲染器。
//
// 与旧实现的差异：
//   - 模板名即虚拟路径（<prefix>/<相对路径>，不含 .tmpl），与原 {{define "..."}} 的名字一致；
//   - sharedVariable 与已编译模板集都用原子指针承载，渲染路径全程无锁，消除了热重载与并发渲染的数据竞争；
//   - 渲染失败时不写出任何内容（pongo2 内部先渲染到缓冲区）。
type Template struct {
	// set 已编译模板集，Load 时整体原子替换
	set atomic.Pointer[pongo2.TemplateSet]
	// shared 全局共享变量，写时复制
	shared atomic.Pointer[pongo2.Context]

	loader *Loader

	loadMu sync.Mutex
	paths  []string

	watcher  *fsnotify.Watcher
	watchMu  sync.Mutex
	watching map[string]struct{}

	funcMu  sync.Mutex
	funcMap map[string]any

	logger *zap.Logger
	bus    event.Bus
}

func NewTemplate(logger *zap.Logger, bus event.Bus) *Template {
	t := &Template{
		loader:   &Loader{},
		watching: map[string]struct{}{},
		logger:   logger,
		funcMap:  map[string]any{},
		bus:      bus,
	}

	empty := pongo2.Context{}
	t.shared.Store(&empty)

	t.addUtilFunc()
	// 先放一个空模板集，保证未 Load 时渲染路径也能给出明确错误而不是空指针
	t.set.Store(t.newSet())

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	t.watcher = watcher
	go t.Watch()
	return t
}

// Mount 显式声明模板目录的命名空间前缀。
// 主题目录名与主题 id 不一定相同（如 default-theme-anatole 的 id 是 caicai_anatole），
// 因此调用方需要在 Load 前用主题 id 挂载其目录。
func (t *Template) Mount(prefix, dir string) {
	t.loader.Mount(prefix, dir)
}

// Exists 判断模板是否存在，用于渲染前的兜底探测（替代旧实现的 HTMLTemplate.Lookup）
func (t *Template) Exists(name string) bool {
	_, ok := t.loader.Resolve(name)
	return ok
}

// Load 挂载并重新编译全部模板。任一模板编译失败时保持原有模板集不变。
func (t *Template) Load(paths []string) error {
	t.loadMu.Lock()
	defer t.loadMu.Unlock()
	return t.load(paths)
}

func (t *Template) load(paths []string) error {
	for _, dir := range paths {
		if _, ok := t.loader.PrefixForDir(dir); !ok {
			// 未显式挂载的目录退回用目录名作前缀（common 目录正好命中）
			t.loader.Mount(filepath.Base(dir), dir)
		}
		if err := t.watchDir(dir); err != nil {
			return xerr.WithMsg(err, "watch template dir err").WithStatus(xerr.StatusInternalServerError)
		}
	}

	next := t.newSet()
	// pongo2 的 {% include "静态路径" %} 在解析期编译引用目标，
	// 因此全量预编译能一次性暴露所有模板语法/引用问题，对齐旧实现 ParseFiles 遇错即失败的行为。
	if err := t.precompile(next); err != nil {
		return xerr.WithMsg(err, "parse template err").WithStatus(xerr.StatusInternalServerError)
	}

	t.set.Store(next)
	t.paths = paths
	t.logger.Info("template.load.success", zap.Int("roots", len(paths)))
	return nil
}

// Reload 清空编译缓存并通知监听器重新加载。
// 模板为按需编译 + 缓存，清空缓存后下次渲染会重新读盘，因此热重载无需重建模板集。
func (t *Template) Reload(_ []string) error {
	if set := t.set.Load(); set != nil {
		set.CleanCache()
	}
	t.bus.Publish(context.Background(), &event.ThemeFileUpdatedEvent{})
	return nil
}

func (t *Template) SetSharedVariable(name string, value interface{}) {
	next := pongo2.Context{}
	if old := t.shared.Load(); old != nil {
		next.Update(*old)
	}
	next[name] = value
	t.shared.Store(&next)
}

func (t *Template) Execute(wr io.Writer, data Model) error {
	return t.execute(wr, "", data)
}

func (t *Template) ExecuteTemplate(wr io.Writer, name string, data Model) error {
	return t.execute(wr, name, data)
}

// ExecuteText 与 Execute 同源，保留仅为不改动调用方。
// pongo2 只有一套引擎，feed 等非 HTML 输出由模板内的 {% autoescape off %} 表达。
func (t *Template) ExecuteText(wr io.Writer, data Model) error {
	return t.execute(wr, "", data)
}

func (t *Template) ExecuteTextTemplate(wr io.Writer, name string, data Model) error {
	return t.execute(wr, name, data)
}

func (t *Template) execute(wr io.Writer, name string, data Model) (err error) {
	set := t.set.Load()
	if set == nil {
		return xerr.WithMsg(fmt.Errorf("template set not loaded"), "render template err").
			WithStatus(xerr.StatusInternalServerError)
	}

	tpl, err := set.FromCache(name)
	if err != nil {
		return xerr.WithMsg(err, "parse template err").WithStatus(xerr.StatusInternalServerError)
	}

	// pongo2 内部没有 recover，模板里访问到 nil 指针等边界会直接 panic 到调用方
	defer func() {
		if r := recover(); r != nil {
			t.logger.Error("template.render.panic", zap.String("name", name), zap.Stack("stack"))
			err = xerr.WithMsg(fmt.Errorf("render template %s panic: %v", name, r), "render template panic").
				WithStatus(xerr.StatusInternalServerError)
		}
	}()

	return tpl.ExecuteWriter(t.buildContext(data), wr)
}

// buildContext 合并共享变量与请求级数据，请求级同名 key 优先，最后注入 now。
// 对应 pongo2 内部 newContextForExecution 的 Update(set.Globals) -> Update(context) 顺序。
func (t *Template) buildContext(data Model) pongo2.Context {
	ctx := pongo2.Context{}
	if shared := t.shared.Load(); shared != nil {
		ctx.Update(*shared)
	}
	if data != nil {
		ctx.Update(pongo2.Context(data))
	}
	ctx["now"] = time.Now()
	return ctx
}

// AddFunc 注册模板辅助函数。
// 启动期（首次 Load 之前）只累积到 funcMap；运行期调用会重建模板集使新函数对已编译模板可见。
func (t *Template) AddFunc(name string, fn interface{}) {
	t.funcMu.Lock()
	t.funcMap[name] = fn
	t.funcMu.Unlock()

	t.loadMu.Lock()
	defer t.loadMu.Unlock()
	if len(t.paths) == 0 {
		return
	}
	if err := t.load(t.paths); err != nil {
		t.logger.Error("template.addfunc.reload", zap.String("name", name), zap.Error(err))
	}
}

func (t *Template) newSet() *pongo2.TemplateSet {
	set := pongo2.NewSet("airpress", t.loader)

	t.funcMu.Lock()
	defer t.funcMu.Unlock()
	for name, fn := range t.funcMap {
		set.Globals[name] = fn
	}
	return set
}

func (t *Template) precompile(set *pongo2.TemplateSet) error {
	return t.loader.Walk(func(virtualPath string) error {
		_, err := set.FromCache(virtualPath)
		return err
	})
}

// watchDir 逐目录注册 fsnotify 监听（fsnotify 不递归），重复调用幂等。
func (t *Template) watchDir(dir string) error {
	t.watchMu.Lock()
	defer t.watchMu.Unlock()

	return filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if _, ok := t.watching[p]; ok {
			return nil
		}
		if err := t.watcher.Add(p); err != nil {
			return err
		}
		t.watching[p] = struct{}{}
		return nil
	})
}

func (t *Template) addUtilFunc() {
	// unix_milli_time_format 的入参放宽为 any：
	// 模板中毫秒时间戳可能来自 int64 字段或经 filter 转换的 int，
	// pongo2 对函数实参做严格类型校验，放宽后由 cast 做容错转换。
	t.funcMap["unix_milli_time_format"] = func(format string, v any) string {
		return time.UnixMilli(cast.ToInt64(v)).Format(format)
	}
	// pongo2 没有 printf/print，直接复用标准库（变参 + any 形参可通过其类型校验）
	t.funcMap["printf"] = fmt.Sprintf
	t.funcMap["print"] = fmt.Sprint
}

// isTemplateEvent 判断文件事件是否需要触发模板重载。
// 除 .tmpl 文件的增删改外，目录的增删也会改变模板扫描范围，同样需要重载。
func (t *Template) isTemplateEvent(e fsnotify.Event) bool {
	if e.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	if filepath.Ext(e.Name) == templateSuffix {
		return true
	}
	if e.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
		info, err := os.Stat(e.Name)
		return err != nil || info.IsDir()
	}
	return false
}