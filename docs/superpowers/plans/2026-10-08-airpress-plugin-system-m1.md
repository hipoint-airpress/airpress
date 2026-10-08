# airpress 插件系统 M1 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按规格 `docs/superpowers/specs/2026-10-08-airpress-plugin-system-design.md`（下称 spec）落地 M1：manifest 校验、wazero 实例与极小 JSON ABI、权限门控宿主函数（kv/log/now/random/option.self）、`plugin`/`plugin_kv` 两张表、生命周期服务（zip 上传→启停→卸载→设置）、`/api/admin/plugins` 管理端点。

**Architecture:** 每插件一个独立 `wazero.Runtime` + 只含被授权导出名的 host module `airpress`（越权 import 在实例化期链接失败，权限=接口）；插件是 reactor（只导出 `alloc/free/handle/abi_version`，不导出 `_start`，实例化用 `WithStartFunctions()` 清空启动函数）；请求/响应均为 JSON 信封，经线性内存借用传递。测试夹具（echo/kvsink/malicious/zerohandle/hang 5 个）由测试内纯 Go 微型 wasm 构造器生成，不依赖 TinyGo、不依赖外网。

**Tech Stack:** Go 1.27；`github.com/tetratelabs/wazero v1.12.0`（已在 `~/go/pkg/mod` 缓存）；`gopkg.in/yaml.v2`（go.mod 已有，**不新增 YAML 库**）；gin/gorm/`gorm.io/driver/sqlite`（CGO）/fx 均为现有依赖。

**执行环境事实（写计划时逐一核实，可直接信任）：**
- wazero API：`wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(p).WithCloseOnContextDone(true))`（后两个方法在 **RuntimeConfig**）；`rt.NewHostModuleBuilder("airpress").NewFunctionBuilder().WithFunc(f).Export(name)`；host 闭包签名 `(ctx context.Context, mod api.Module, p uint32...) ...`；`rt.CompileModule(ctx, bytes)`；`rt.InstantiateModule(ctx, cm, wazero.NewModuleConfig().WithName(n).WithStartFunctions())`；`fn.Call(ctx, ...uint64) ([]uint64, error)`；`mod.Memory().Read(offset,size)`/`Write(offset,[]byte)`/`ReadUint32Le(offset)`；`mod.Close(ctx)`、`rt.Close(ctx)`。
- 实体表名为裸名（`plugin`），建表在 `dal/dal.go` 的 `dbMigrate()` AutoMigrate 列表（约 :96）。
- 处理器约定：`func (h *XHandler) M(ctx *gin.Context) (interface{}, error)` + `s.wrapHandler(...)`；错误 `xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(...)`；构造函数在 `handler/admin/init.go` 的 `injection.Provide` 登记；`Server` 字段（约 :36-68）、`ServerParams`（:83）、`NewServer` 赋值（:149 起）、路由注册 `handler/router.go` 的 `authRouter` 块。
- 工具链：`go` 1.27 linux；GOPROXY=goproxy.cn 可达；**github 直连超时**（所以绝不引入需要现下载 TinyGo 的步骤）；Go 1.28 将支持 wasip1 `-buildmode=plugin` 常驻 reactor（用户确认），M1 用不到。

**文件结构总览（本计划创建/修改；职责一句话）**

```
go.mod                                  # +wazero
config/model.go                         # AirPress + PluginDir 字段
config/config.go                        # normalizeDir(PluginDir,"plugins")
plugin/manifest/manifest.go(+_test.go)  # plugin.yaml 类型/严格解析/校验（纯函数）
plugin/guard/guard.go(+_test.go)        # 域名通配、scheme、内网 IP 判定（M1 纯函数；fetch 接线在 M3）
model/entity/plugin.go                  # Plugin 实体
model/entity/pluginkv.go                # PluginKV 实体
dal/dal.go                              # AutoMigrate 追加两实体
plugin/store/store.go(+_test.go)        # plugin/plugin_kv/option 行的 gorm 仓储
plugin/runtime/envelope.go(+_test.go)   # 信封类型与 kind 常量
plugin/runtime/wasmfix/builder.go       # 微型 wasm 二进制构造器（仅测试夹具用）
plugin/runtime/wasmfix/fixtures.go(+_test.go) # echo/kvsink/malicious/zerohandle/hang
plugin/host/host.go(+_test.go)          # 权限→host module 构造 + 各 host 函数实现
plugin/runtime/instance.go(+_test.go)   # 实例：握手/调用/超时/trap 重建/互斥
plugin/zipx/zipx.go(+_test.go)          # 安全解包（防穿越/限额）
plugin/lifecycle/service.go(+_test.go,init.go) # 生命周期编排 + fx 接线 + OnStop
plugin/dto/plugin.go                    # 管理 API 的出参 DTO
handler/admin/plugin.go(+_test.go)      # /api/admin/plugins 端点
handler/admin/init.go                   # Provide NewPluginHandler
handler/server.go                       # Server 字段 + ServerParams + 赋值
handler/router.go                       # pluginRouter 注册
```

依赖方向：`manifest`,`guard` 无内部依赖；`store`→entity；`host`→store,manifest；`runtime`→host,manifest；`lifecycle`→runtime,host,store,manifest,zipx,config；`handler/admin`→lifecycle。无环。

---

### Task 1: 依赖与配置字段

**Files:**
- Modify: `go.mod` / `go.sum`
- Modify: `config/model.go`（`type AirPress` 结构体，约 :51-63）
- Modify: `config/config.go`（`normalizeDir(...ThemeDir...)` 之后，约 :74）

- [ ] **Step 1: 添加 wazero 依赖**

```bash
cd /mnt/d/codes/github/hipoint-airpress/airpress
go get github.com/tetratelabs/wazero@v1.12.0
```
预期：go.mod require 出现 `github.com/tetratelabs/wazero v1.12.0`（缓存已存在，不依赖外网）。

- [ ] **Step 2: config/model.go 的 AirPress 结构体加字段**

在 `AdminURLPath` 行后追加：

```go
	// PluginDir 是 WASM 插件的安装根目录（plugin/<name>/<version>/ 布局）
	PluginDir string `mapstructure:"plugin_dir"`
```

- [ ] **Step 3: config/config.go 设默认值**

在 `normalizeDir(&conf.AirPress.ThemeDir, "resources/template/theme")` 之后加一行：

```go
	normalizeDir(&conf.AirPress.PluginDir, "plugins")
```
不做存在性 panic 检查（目录由生命周期服务按需创建）。

- [ ] **Step 4: 验证编译与现有测试不回归**

```bash
go build ./... && go test ./config/ ./dal/ 2>&1 | tail -5
```
预期：build 通过；无 FAIL。

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum config/model.go config/config.go
git commit -m "chore(plugin): 引入 wazero 依赖并新增 plugin_dir 配置"
```

---

### Task 2: manifest 包（解析与校验，纯函数）

**Files:**
- Create: `plugin/manifest/manifest.go`
- Test: `plugin/manifest/manifest_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/manifest/manifest_test.go`：

```go
package manifest

import (
	"strings"
	"testing"
)

const validYAML = `
apiVersion: airpress/v1alpha1
kind: Plugin
metadata:
  name: linkscraper
  title: 链接导入
  version: 1.0.0
  author: ryan
spec:
  entry: main.wasm
  permissions: [kv, log, option.self, http.fetch]
  events: [post.updated]
  filters: [post.content.saved]
  fetch:
    allowedHosts: ["*.example.com", "*"]
  menu:
    - { title: 链接导入, path: admin/scrape, icon: link }
  api: ["GET admin/scrape", "POST admin/scrape"]
  timeouts: { api: 60s }
  settings:
    - { name: max_images, type: number }
`

func TestParseValid(t *testing.T) {
	m, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if m.Metadata.Name != "linkscraper" || m.Spec.Entry != "main.wasm" {
		t.Fatalf("bad fields: %+v", m)
	}
	if len(m.Spec.Permissions) != 4 || m.Spec.Permissions[3] != PermHTTPFetch {
		t.Fatalf("bad permissions: %v", m.Spec.Permissions)
	}
}

func mustFail(t *testing.T, yamlStr string, wantSub string) {
	t.Helper()
	_, err := Parse([]byte(yamlStr))
	if err == nil || !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("want error containing %q, got %v", wantSub, err)
	}
}

func head(s string) string {
	return "apiVersion: airpress/v1alpha1\nkind: Plugin\nmetadata: {name: p, title: P, version: 1.0.0}\nspec:\n" + s
}

func TestValidateRejects(t *testing.T) {
	cases := []struct{ body, wantSub string }{
		{"apiVersion: airpress/v2\nkind: Plugin\nmetadata: {name: p, title: P, version: 1.0.0}\nspec: {entry: m.wasm}\n", "apiVersion"},
		{"apiVersion: airpress/v1alpha1\nkind: Theme\nmetadata: {name: p, title: P, version: 1.0.0}\nspec: {entry: m.wasm}\n", "kind"},
		{head("  entry: main.wasm\n  permissions: [nope]"), "permission"},
		{head("  entry: ../evil.wasm"), "entry"},
		{head("  entry: main.js"), "entry"},
		{head("  entry: main.wasm\n  events: [post.deleted]"), "event"},
		{head("  entry: main.wasm\n  filters: [post.content.rendered]"), "filter"},
		{head("  entry: main.wasm\n  timeouts: {api: 200s}"), "timeout"},
		{head("  entry: main.wasm\n  api: [\"DELETE x\"]"), "api"},
		{head("  entry: main.wasm\n  fetch: {allowedHosts: [\"http://x.com\"]}"), "host"},
		{"apiVersion: airpress/v1alpha1\nkind: Plugin\nmetadata: {name: P!, title: P, version: 1.0.0}\nspec: {entry: m.wasm}\n", "name"},
		{head("  entry: main.wasm\n  unknownKey: 1"), "unknown"},
	}
	for _, c := range cases {
		mustFail(t, c.body, c.wantSub)
	}
}

func TestValidateUnknownFieldAtRoot(t *testing.T) {
	mustFail(t, "apiVersion: airpress/v1alpha1\nkind: Plugin\nbogus: 1\n", "unknown")
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./plugin/manifest/ -v
```
预期：编译错误 `undefined: Parse`（实现还不存在）。

- [ ] **Step 3: 实现 `plugin/manifest/manifest.go`**

```go
// Package manifest 解析并校验插件包里的 plugin.yaml（spec §2.2）。
package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	APIVersion = "airpress/v1alpha1"
	KindPlugin = "Plugin"
	// OptionKeyPrefix 是插件设置在 option 表中的键前缀（spec §2.3）
	OptionKeyPrefix = "plugin."
)

// Permission 是宿主能力的授权单位（spec §4.1）；未声明 = 未授权。
type Permission string

const (
	PermKV              Permission = "kv"
	PermLog             Permission = "log"
	PermOptionSelf      Permission = "option.self"
	PermPostRead        Permission = "post.read"
	PermPostWrite       Permission = "post.write"
	PermAttachmentWrite Permission = "attachment.write"
	PermHTTPFetch       Permission = "http.fetch"
)

var knownPermissions = map[Permission]bool{
	PermKV: true, PermLog: true, PermOptionSelf: true, PermPostRead: true,
	PermPostWrite: true, PermAttachmentWrite: true, PermHTTPFetch: true,
}

// KnownEvents/KnownFilters 是对外承诺的钩子目录（spec §5）。
// 不在表内的名字直接拒绝——宁可拒绝拼写错误，不可静默。
var KnownEvents = map[string]bool{
	"startup": true, "post.created": true, "post.updated": true,
	"comment.new": true, "comment.reply": true, "user.updated": true,
	"option.updated": true, "theme.activated": true, "theme.updated": true,
	"theme.file.updated": true, "attachment.uploaded": true,
}

// log/now/random 是免费能力，声明与否不影响 host module，仅用于文档完整。
var knownFreePerms = map[Permission]bool{PermLog: true}

var KnownFilters = map[string]bool{
	"post.content.saved": true,
}

var settingTypes = map[string]bool{
	"text": true, "textarea": true, "select": true,
	"number": true, "checkbox": true, "switch": true, "color": true,
}

// 超时上限（spec §3.4）：manifest 可下调不可上调。
var timeoutLimits = map[string]time.Duration{
	"event": 120 * time.Second, "filter": 120 * time.Second,
	"api": 120 * time.Second, "init": 60 * time.Second,
	"shutdown": 10 * time.Second,
}

var (
	nameRe    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	entryRe   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*\.wasm$`)
	menuRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
	apiRe     = regexp.MustCompile(`^(GET|POST|PUT|DELETE) [a-z0-9][a-z0-9._/-]*$`)
	hostRe    = regexp.MustCompile(`^(\*\.)?[a-z0-9][a-z0-9.-]*$`)
)

type Metadata struct {
	Name        string `yaml:"name"`
	Title       string `yaml:"title"`
	Version     string `yaml:"version"`
	Author      string `yaml:"author"`
	Description string `yaml:"description"`
}

type FetchSpec struct {
	AllowedHosts []string `yaml:"allowedHosts"`
	AllowHTTP    bool     `yaml:"allowHTTP"`
}

type MenuItem struct {
	Title string `yaml:"title"`
	Path  string `yaml:"path"`
	Icon  string `yaml:"icon"`
}

// SettingItem 字段规范对齐主题 settings.yaml：除 name/type 外宽松透传（inline 兜底）。
type SettingItem struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	Rest  map[string]interface{} `yaml:",inline"`
}

type Spec struct {
	Entry       string            `yaml:"entry"`
	Permissions []Permission      `yaml:"permissions"`
	Events      []string          `yaml:"events"`
	Filters     []string          `yaml:"filters"`
	Fetch       FetchSpec         `yaml:"fetch"`
	Menu        []MenuItem        `yaml:"menu"`
	API         []string          `yaml:"api"`
	Settings    []SettingItem     `yaml:"settings"`
	Timeouts    map[string]string `yaml:"timeouts"`
	Rest        map[string]interface{} `yaml:",inline"`
}

type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// OptionKey 返回该插件设置所在的 option 键（spec §2.3）。
func OptionKey(name string) string { return OptionKeyPrefix + name + ".settings" }

// Parse 严格解析并校验；任何不合法都拒绝安装。
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.UnmarshalStrict(data, &m); err != nil {
		return nil, fmt.Errorf("unknown or bad fields: %w", err)
	}
	if err := validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func validate(m *Manifest) error {
	var errs []string
	add := func(f string, a ...interface{}) { errs = append(errs, fmt.Sprintf(f, a...)) }

	if m.APIVersion != APIVersion {
		add("apiVersion must be %q, got %q", APIVersion, m.APIVersion)
	}
	if m.Kind != KindPlugin {
		add("kind must be %q, got %q", KindPlugin, m.Kind)
	}
	if !nameRe.MatchString(m.Metadata.Name) {
		add("metadata.name %q: want [a-z][a-z0-9-]*", m.Metadata.Name)
	}
	if m.Metadata.Title == "" {
		add("metadata.title is required")
	}
	if !versionRe.MatchString(m.Metadata.Version) {
		add("metadata.version %q: want x.y.z", m.Metadata.Version)
	}
	if !entryRe.MatchString(m.Spec.Entry) || strings.Contains(m.Spec.Entry, "..") || strings.HasPrefix(m.Spec.Entry, "/") {
		add("spec.entry %q: want relative *.wasm without ..", m.Spec.Entry)
	}
	seen := map[Permission]bool{}
	for _, p := range m.Spec.Permissions {
		if !knownPermissions[p] {
			add("unknown permission %q", string(p))
		}
		if seen[p] {
			add("duplicate permission %q", string(p))
		}
		seen[p] = true
	}
	for _, e := range m.Spec.Events {
		if !KnownEvents[e] {
			add("unknown event %q", e)
		}
	}
	for _, f := range m.Spec.Filters {
		if !KnownFilters[f] {
			add("unknown filter %q", f)
		}
	}
	for _, h := range m.Spec.Fetch.AllowedHosts {
		if h != "*" && !hostRe.MatchString(h) {
			add("bad allowedHost %q", h)
		}
	}
	for _, mi := range m.Spec.Menu {
		if mi.Title == "" || !menuRe.MatchString(mi.Path) || strings.Contains(mi.Path, "..") {
			add("bad menu item %+v", mi)
		}
	}
	for _, a := range m.Spec.API {
		if !apiRe.MatchString(a) {
			add("bad api %q: want \"METHOD path\"", a)
		}
	}
	for _, s := range m.Spec.Settings {
		if !nameRe.MatchString(s.Name) {
			add("bad setting name %q", s.Name)
		}
		if !settingTypes[s.Type] {
			add("bad setting type %q for %q", s.Type, s.Name)
		}
	}
	for k, v := range m.Spec.Timeouts {
		limit, ok := timeoutLimits[k]
		if !ok {
			add("unknown timeout key %q", k)
			continue
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			add("bad timeout %s: %q", k, v)
		} else if d <= 0 || d > limit {
			add("timeout %s=%v exceeds limit %v", k, d, limit)
		}
	}
	if len(errs) > 0 {
		return errors.New("invalid manifest: " + strings.Join(errs, "; "))
	}
	return nil
}

// Granted 把权限列表转成查找表（host 构造用）。
func (m *Manifest) Granted() map[Permission]bool {
	g := map[Permission]bool{}
	for _, p := range m.Spec.Permissions {
		g[p] = true
	}
	return g
}
```

- [ ] **Step 4: 运行确认通过**

```bash
go test ./plugin/manifest/ -v
```
预期：`ok github.com/hipoint-airpress/airpress/plugin/manifest`（全部 PASS）。

- [ ] **Step 5: Commit**

```bash
git add plugin/manifest/
git commit -m "feat(plugin): manifest 解析与校验"
```

---

### Task 3: guard 包（域名/scheme/IP 纯判定）

**Files:**
- Create: `plugin/guard/guard.go`
- Test: `plugin/guard/guard_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/guard/guard_test.go`：

```go
package guard

import (
	"net"
	"testing"
)

func TestMatchHost(t *testing.T) {
	cases := []struct{ pattern, host string; want bool }{
		{"*", "any.example.com", true},
		{"example.com", "example.com", true},
		{"example.com", "sub.example.com", false},
		{"*.example.com", "a.example.com", true},
		{"*.example.com", "example.com", false},
		{"*.example.com", "a.b.example.com", true},
		{"*.example.com", "evil.com", false},
		{"*.example.com", "notexample.com", false},
	}
	for _, c := range cases {
		if got := MatchHost(c.pattern, c.host); got != c.want {
			t.Errorf("MatchHost(%q,%q)=%v want %v", c.pattern, c.host, got, c.want)
		}
	}
}

func TestHostListAllows(t *testing.T) {
	if !HostListAllows([]string{"a.com", "*.b.com"}, "x.b.com") {
		t.Fatal("want allow")
	}
	if HostListAllows([]string{"a.com"}, "b.com") {
		t.Fatal("want deny")
	}
	if HostListAllows(nil, "a.com") {
		t.Fatal("empty list must deny all")
	}
	if !HostListAllows([]string{"*"}, "whatever.io") {
		t.Fatal("\"*\" must allow")
	}
}

func TestCheckScheme(t *testing.T) {
	if err := CheckScheme("http://example.com", false); err == nil {
		t.Fatal("http must be denied by default")
	}
	if err := CheckScheme("http://example.com", true); err != nil {
		t.Fatalf("http allowed after opt-in: %v", err)
	}
	if err := CheckScheme("https://example.com", false); err != nil {
		t.Fatalf("https: %v", err)
	}
	if err := CheckScheme("ftp://example.com", true); err == nil {
		t.Fatal("only http/https")
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.2.3.4", "10.1.2.3", "172.16.0.1", "172.31.255.255",
		"192.168.4.5", "169.254.169.254", "100.64.0.1", "0.0.0.0",
		"::1", "fe80::1", "fc00::1", "::",
	}
	for _, s := range blocked {
		if !IsBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "93.184.216.34", "172.15.0.1", "100.127.0.1", "2606:4700:4700::1111"}
	for _, s := range allowed {
		if IsBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./plugin/guard/`
Expected: 编译失败 `undefined: MatchHost`。

- [ ] **Step 3: 实现 `plugin/guard/guard.go`**

```go
// Package guard 实现 spec §4.2 的纯判定部分（M1 不含 http 传输接线，那是 M3）。
package guard

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// MatchHost 单条模式匹配：支持 "*"、"*.example.com"（不含裸域）与精确域名。
// 大小写不敏感（入参应为 url.Parse 后的小写 host）。
func MatchHost(pattern, host string) bool {
	host = strings.ToLower(host)
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[len("*"):] // ".example.com"
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return pattern == host
}

// HostListAllows 任一模式命中即放行；空列表=全部拒绝（白名单语义）。
func HostListAllows(patterns []string, host string) bool {
	for _, p := range patterns {
		if MatchHost(p, host) {
			return true
		}
	}
	return false
}

// CheckScheme：默认仅 https；allowHTTP 放开 http。其它 scheme 一律拒绝。
func CheckScheme(rawURL string, allowHTTP bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("bad url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		if allowHTTP {
			return nil
		}
		return fmt.Errorf("http not allowed (set fetch.allowHTTP)")
	default:
		return fmt.Errorf("scheme %q not allowed", u.Scheme)
	}
}

// IsBlockedIP：回环/私网/链路本地/运营商 NAT(100.64/10)/ unspecified。
// IPv4-mapped IPv6 先 To4() 再判。M3 的 Transport.DialContext 在“已连接 socket 的
// peer IP”上调用本函数，使 DNS rebinding 无效（spec §4.2 第 3 条）。
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.IsLoopback() || v4.IsPrivate() || v4.IsLinkLocalUnicast() ||
			v4.IsUnspecified() || (v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127)
	}
	// fc00::/7 由 IsPrivate() 覆盖（Go ≥1.13），测试用例 fc00::1 因此通过
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsUnspecified()
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./plugin/guard/ -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add plugin/guard/
git commit -m "feat(plugin): guard 域名/scheme/IP 纯判定"
```

---

### Task 4: 实体 + AutoMigrate + store 仓储

**Files:**
- Create: `model/entity/plugin.go`、`model/entity/pluginkv.go`
- Modify: `dal/dal.go`（`dbMigrate()` 的 AutoMigrate 列表，约 :96-98）
- Create: `plugin/store/store.go`
- Test: `plugin/store/store_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/store/store_test.go`：

```go
package store

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/model/entity"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Plugin{}, &entity.PluginKV{}, &entity.Option{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPluginCRUD(t *testing.T) {
	r := NewRepo(newTestDB(t))
	ctx := context.Background()
	p := &entity.Plugin{Name: "demo", Title: "Demo", Version: "1.0.0",
		Status: "inactive", InstallPath: "/tmp/demo", Sha256: "abc", Manifest: "{}"}
	if err := r.CreatePlugin(ctx, p); err != nil {
		t.Fatal(err)
	}
	// name 唯一约束
	if err := r.CreatePlugin(ctx, &entity.Plugin{Name: "demo", Title: "X", Version: "1.0.0",
		Status: "inactive", InstallPath: "/x", Sha256: "x", Manifest: "{}"}); err == nil {
		t.Fatal("duplicate name must fail")
	}
	got, err := r.GetPluginByName(ctx, "demo")
	if err != nil || got.ID != p.ID || got.Title != "Demo" {
		t.Fatalf("get by name: %v %+v", err, got)
	}
	got.Status = "active"
	if err := r.UpdatePlugin(ctx, got); err != nil {
		t.Fatal(err)
	}
	got2, _ := r.GetPluginByID(ctx, p.ID)
	if got2.Status != "active" {
		t.Fatalf("status not updated: %v", got2.Status)
	}
	list, err := r.ListPlugins(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", err, list)
	}
	if err := r.DeletePlugin(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetPluginByID(ctx, p.ID); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestKV(t *testing.T) {
	r := NewRepo(newTestDB(t))
	ctx := context.Background()
	if err := r.KVSet(ctx, 1, "seen:a", "10"); err != nil {
		t.Fatal(err)
	}
	if err := r.KVSet(ctx, 1, "seen:b", "11"); err != nil { // upsert
		t.Fatal(err)
	}
	if err := r.KVSet(ctx, 2, "seen:a", "other-plugin"); err != nil { // 命名空间隔离
		t.Fatal(err)
	}
	v, err := r.KVGet(ctx, 1, "seen:a")
	if err != nil || v != "10" {
		t.Fatalf("get: %v %q", err, v)
	}
	if _, err := r.KVGet(ctx, 1, "nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound got %v", err)
	}
	rows, err := r.KVList(ctx, 1, "seen:", 100)
	if err != nil || len(rows) != 2 {
		t.Fatalf("list plugin1 prefix: %v %v", err, rows)
	}
	rows2, _ := r.KVList(ctx, 2, "", 100)
	if len(rows2) != 1 {
		t.Fatalf("namespace leak: %v", rows2)
	}
	if err := r.KVDel(ctx, 1, "seen:a"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.KVGet(ctx, 1, "seen:a"); err != ErrNotFound {
		t.Fatal("del failed")
	}
	if err := r.KVDeleteByPlugin(ctx, 1); err != nil {
		t.Fatal(err)
	}
	rows, _ = r.KVList(ctx, 1, "", 100)
	if len(rows) != 0 {
		t.Fatalf("delete by plugin failed: %v", rows)
	}
}

func TestOption(t *testing.T) {
	r := NewRepo(newTestDB(t))
	ctx := context.Background()
	if _, err := r.OptionGet(ctx, "plugin.demo.settings"); err != ErrNotFound {
		t.Fatal("want not found")
	}
	if err := r.OptionSet(ctx, "plugin.demo.settings", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if err := r.OptionSet(ctx, "plugin.demo.settings", `{"a":2}`); err != nil { // upsert
		t.Fatal(err)
	}
	v, err := r.OptionGet(ctx, "plugin.demo.settings")
	if err != nil || v != `{"a":2}` {
		t.Fatalf("option: %v %q", err, v)
	}
	if err := r.OptionDel(ctx, "plugin.demo.settings"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.OptionGet(ctx, "plugin.demo.settings"); err != ErrNotFound {
		t.Fatal("del failed")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./plugin/store/`
Expected: 编译失败（`entity.Plugin` 未定义等）。

- [ ] **Step 3: 实体文件**

创建 `model/entity/plugin.go`（手写实体，字段风格对齐 `.gen.go`）：

```go
package entity

import "time"

const TableNamePlugin = "plugin"

// Plugin 是 WASM 插件注册行（spec §2.3）；Status: inactive|active。
type Plugin struct {
	ID          int32      `gorm:"column:id;type:int;primaryKey;autoIncrement:true" json:"id"`
	Name        string     `gorm:"column:name;type:varchar(64);not null;uniqueIndex:uk_plugin_name" json:"name"`
	Title       string     `gorm:"column:title;type:varchar(128);not null" json:"title"`
	Version     string     `gorm:"column:version;type:varchar(32);not null" json:"version"`
	Status      string     `gorm:"column:status;type:varchar(16);not null;default:inactive" json:"status"`
	InstallPath string     `gorm:"column:install_path;type:varchar(255);not null" json:"install_path"`
	Sha256      string     `gorm:"column:sha256;type:varchar(64);not null" json:"sha256"`
	Manifest    string     `gorm:"column:manifest;type:text;not null" json:"manifest"`
	CreateTime  time.Time  `gorm:"column:create_time;type:datetime;not null" json:"create_time"`
	UpdateTime  *time.Time `gorm:"column:update_time;type:datetime" json:"update_time"`
}

// TableName Plugin's table name
func (*Plugin) TableName() string {
	return TableNamePlugin
}
```

创建 `model/entity/pluginkv.go`：

```go
package entity

import "time"

const TableNamePluginKV = "plugin_kv"

// PluginKV 是插件命名空间 KV（spec §2.3）；(plugin_id,key) 联合唯一。
type PluginKV struct {
	ID         int64      `gorm:"column:id;type:integer;primaryKey;autoIncrement:true" json:"id"`
	PluginID   int32      `gorm:"column:plugin_id;type:int;not null;uniqueIndex:uk_plugin_kv,priority:1" json:"plugin_id"`
	Key        string     `gorm:"column:key;type:varchar(255);not null;uniqueIndex:uk_plugin_kv,priority:2" json:"key"`
	Value      string     `gorm:"column:value;type:longtext;not null" json:"value"`
	CreateTime time.Time  `gorm:"column:create_time;type:datetime;not null" json:"create_time"`
	UpdateTime *time.Time `gorm:"column:update_time;type:datetime" json:"update_time"`
}

// TableName PluginKV's table name
func (*PluginKV) TableName() string {
	return TableNamePluginKV
}
```

- [ ] **Step 4: 注册 AutoMigrate**

`dal/dal.go` 的 `dbMigrate()` 中，把
`&entity.PostCategory{}, &entity.PostTag{}, &entity.Tag{}, &entity.ThemeSetting{}, &entity.User{})`
结尾改为在其前面追加 `&entity.Plugin{}, &entity.PluginKV{}, `（即列表变为含 20 个实体）。

- [ ] **Step 5: 实现 `plugin/store/store.go`**

```go
// Package store 是 plugin 注册表、插件 KV、插件设置 option 行的 gorm 仓储。
package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

// ErrNotFound 统一仓储读取的“无此行”语义。
var ErrNotFound = errors.New("plugin store: not found")

type Repo struct{ db *gorm.DB }

func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

func wrap(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (r *Repo) CreatePlugin(ctx context.Context, p *entity.Plugin) error {
	p.CreateTime = time.Now()
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *Repo) UpdatePlugin(ctx context.Context, p *entity.Plugin) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.Plugin{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"title": p.Title, "version": p.Version, "status": p.Status,
			"install_path": p.InstallPath, "sha256": p.Sha256, "manifest": p.Manifest,
			"update_time": now,
		}).Error
}

func (r *Repo) ListPlugins(ctx context.Context) ([]*entity.Plugin, error) {
	var rows []*entity.Plugin
	err := r.db.WithContext(ctx).Order("id asc").Find(&rows).Error
	return rows, err
}

func (r *Repo) GetPluginByID(ctx context.Context, id int32) (*entity.Plugin, error) {
	var p entity.Plugin
	if err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		return nil, wrap(err)
	}
	return &p, nil
}

func (r *Repo) GetPluginByName(ctx context.Context, name string) (*entity.Plugin, error) {
	var p entity.Plugin
	if err := r.db.WithContext(ctx).First(&p, "name = ?", name).Error; err != nil {
		return nil, wrap(err)
	}
	return &p, nil
}

func (r *Repo) DeletePlugin(ctx context.Context, id int32) error {
	return r.db.WithContext(ctx).Delete(&entity.Plugin{}, "id = ?", id).Error
}

// ---- KV（plugin_id 硬命名空间，spec §2.3）----
// 已知限制：KVList 的 prefix 走 LIKE，键中含 %/_ 时可能多匹配；
// 键由插件自控，M1 接受（M3 如需严格语义改为转义）。

func (r *Repo) KVSet(ctx context.Context, pluginID int32, key, value string) error {
	now := time.Now()
	row := entity.PluginKV{PluginID: pluginID, Key: key, Value: value, CreateTime: now}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "plugin_id"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"value": value, "update_time": now}),
	}).Create(&row).Error
}

func (r *Repo) KVGet(ctx context.Context, pluginID int32, key string) (string, error) {
	var row entity.PluginKV
	if err := r.db.WithContext(ctx).First(&row, "plugin_id = ? AND key = ?", pluginID, key).Error; err != nil {
		return "", wrap(err)
	}
	return row.Value, nil
}

func (r *Repo) KVDel(ctx context.Context, pluginID int32, key string) error {
	return r.db.WithContext(ctx).Delete(&entity.PluginKV{}, "plugin_id = ? AND key = ?", pluginID, key).Error
}

func (r *Repo) KVList(ctx context.Context, pluginID int32, prefix string, limit int) ([]entity.PluginKV, error) {
	var rows []entity.PluginKV
	err := r.db.WithContext(ctx).Where("plugin_id = ? AND key LIKE ?", pluginID, prefix+"%").
		Order("key asc").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repo) KVDeleteByPlugin(ctx context.Context, pluginID int32) error {
	return r.db.WithContext(ctx).Delete(&entity.PluginKV{}, "plugin_id = ?", pluginID).Error
}

// ---- 插件设置 option 行（键 plugin.<name>.settings，spec §2.3）----

func (r *Repo) OptionGet(ctx context.Context, key string) (string, error) {
	var o entity.Option
	if err := r.db.WithContext(ctx).First(&o, "option_key = ?", key).Error; err != nil {
		return "", wrap(err)
	}
	return o.OptionValue, nil
}

func (r *Repo) OptionSet(ctx context.Context, key, value string) error {
	var o entity.Option
	err := r.db.WithContext(ctx).First(&o, "option_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(&entity.Option{
			CreateTime: time.Now(), OptionKey: key,
			Type: consts.OptionTypeCustom, OptionValue: value,
		}).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&entity.Option{}).Where("id = ?", o.ID).
		Updates(map[string]interface{}{"option_value": value, "update_time": time.Now()}).Error
}

func (r *Repo) OptionDel(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Delete(&entity.Option{}, "option_key = ?", key).Error
}
```

- [ ] **Step 6: 运行确认通过**

Run: `go test ./plugin/store/ -v`
Expected: PASS 三个测试。

- [ ] **Step 7: Commit**

```bash
git add model/entity/plugin.go model/entity/pluginkv.go dal/dal.go plugin/store/
git commit -m "feat(plugin): plugin/plugin_kv 实体与仓储"
```

---

### Task 5: 信封类型（runtime 包第一步）

**Files:**
- Create: `plugin/runtime/envelope.go`
- Test: `plugin/runtime/envelope_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/runtime/envelope_test.go`：

```go
package runtime

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRequestOmitEmpty(t *testing.T) {
	rq, err := NewRequest("id-1", KindEvent, "post.updated", nil, map[string]int{"post_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(rq)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `"config"`) {
		t.Fatalf("nil config must be omitted: %s", s)
	}
	if !strings.Contains(s, `"post_id":7`) || !strings.Contains(s, `"hook":"post.updated"`) {
		t.Fatalf("payload/hook missing: %s", s)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	var rp Response
	if err := json.Unmarshal([]byte(`{"ok":false,"error":"boom"}`), &rp); err != nil {
		t.Fatal(err)
	}
	if rp.OK || rp.Error != "boom" {
		t.Fatalf("%+v", rp)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./plugin/runtime/` → 编译失败 `undefined: NewRequest`。

- [ ] **Step 3: 实现 `plugin/runtime/envelope.go`**

```go
// Package runtime 实现插件调用信封与 wazero 实例管理（spec §3）。
package runtime

import "encoding/json"

// ABIVersion 是 handle 握手中插件 abi_version() 必须返回的值（spec §3.2）。
const ABIVersion = 1

// 信封 kind（spec §3.3）。
const (
	KindInit           = "init"
	KindShutdown       = "shutdown"
	KindSettingsChanged = "settings_changed"
	KindEvent          = "event"
	KindFilter         = "filter"
	KindAPI            = "api"
)

// Request 是一次调用的信封。Config 为设置快照 JSON；Payload 形状随 Kind 而定。
type Request struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Hook    string          `json:"hook,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Response 是回传信封。Data 形状随 Kind：filter=改写后的值；api={status,headers,body_b64}。
type Response struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// NewRequest 预序列化 config/payload（保证 Call 时 json.Marshal 不会再失败）。
func NewRequest(id, kind, hook string, config, payload interface{}) (*Request, error) {
	rq := &Request{ID: id, Kind: kind, Hook: hook}
	var err error
	if config != nil {
		if rq.Config, err = json.Marshal(config); err != nil {
			return nil, err
		}
	}
	if payload != nil {
		if rq.Payload, err = json.Marshal(payload); err != nil {
			return nil, err
		}
	}
	return rq, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./plugin/runtime/ -v` → PASS。

- [ ] **Step 5: Commit**

```bash
git add plugin/runtime/envelope.go plugin/runtime/envelope_test.go
git commit -m "feat(plugin): 调用信封类型"
```

---

### Task 6: 微型 wasm 构造器 + 5 个测试夹具

**Files:**
- Create: `plugin/runtime/wasmfix/builder.go`
- Create: `plugin/runtime/wasmfix/fixtures.go`
- Test: `plugin/runtime/wasmfix/fixtures_test.go`

说明：测试夹具用纯 Go 手写 wasm 二进制，**不依赖 TinyGo/外网**；真实插件作者工具链是
官方 Go ≥1.24 的 wasip1 reactor（`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`，
`//go:wasmexport` 导出 alloc/free/handle/abi_version，初始化在 `init()`，`main()` 不被调用；
Go 1.28 的实验性 `-buildmode=plugin`+WIT 是后续演进）。夹具导出 `_initialize` 的有无都被
Task 8 的实例逻辑兼容（存在则调用一次）。

- [ ] **Step 1: 实现构造器 `plugin/runtime/wasmfix/builder.go`**

（夹具正确性由"测试必须能编译运行"自证，先给实现再给测试。）

```go
// Package wasmfix 用纯 Go 构造最小的 reactor wasm 模块，仅供测试：
// 只依赖标准库与 wazero（编译校验），产物只含 spec §3.2 的 4 个导出 + 静态数据段。
package wasmfix

const (
	I32 = 0x7f
	I64 = 0x7e

	opLoop     = 0x02
	opI32Const = 0x41
	opCall     = 0x10
	opDrop     = 0x1a
	opBr       = 0x0c
	opEnd      = 0x0b
	blockEmpty = 0x40
)

type typeDef struct{ params, results []byte }
type importDef struct{ mod, field string; typeIdx uint32 }
type exportDef struct {
	name string
	kind byte
	idx  uint32
}
type dataDef struct{ off uint32; b []byte }

// Module 按"先 import 后 function"的全局函数索引规则编号（wasm 规定 import 函数在前）。
type Module struct {
	types   []typeDef
	imports []importDef
	funcs   []uint32
	bodies  [][]byte
	exports []exportDef
	datas   []dataDef
}

func New() *Module { return &Module{} }

func (m *Module) Type(params, results []byte) uint32 {
	for i, t := range m.types {
		if string(t.params) == string(params) && string(t.results) == string(results) {
			return uint32(i)
		}
	}
	m.types = append(m.types, typeDef{params: params, results: results})
	return uint32(len(m.types) - 1)
}

// ImportFunc 声明一个宿主导入函数，返回其函数索引。
func (m *Module) ImportFunc(mod, field string, params, results []byte) uint32 {
	ti := m.Type(params, results)
	m.imports = append(m.imports, importDef{mod: mod, field: field, typeIdx: ti})
	return uint32(len(m.imports) - 1)
}

// Func 定义一个函数；instrs 为指令流（不含 locals 计数与末尾 end）。返回函数索引。
func (m *Module) Func(params, results []byte, instrs []byte) uint32 {
	ti := m.Type(params, results)
	m.funcs = append(m.funcs, ti)
	m.bodies = append(m.bodies, instrs)
	return uint32(len(m.imports) + len(m.funcs) - 1)
}

func (m *Module) ExportFunc(name string, idx uint32) {
	m.exports = append(m.exports, exportDef{name, 0x00, idx})
}
func (m *Module) ExportMemory(name string) {
	m.exports = append(m.exports, exportDef{name, 0x02, 0})
}
func (m *Module) Data(off uint32, b []byte) { m.datas = append(m.datas, dataDef{off, b}) }

func uleb(n uint64) []byte {
	var out []byte
	for {
		b := byte(n & 0x7f)
		n >>= 7
		if n != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			return out
		}
	}
}

func sleb(n int32) []byte {
	var out []byte
	v := int64(n)
	for {
		b := byte(v & 0x7f)
		v >>= 7
		done := (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0)
		if !done {
			b |= 0x80
		}
		out = append(out, b)
		if done {
			return out
		}
	}
}

func nameBytes(s string) []byte {
	out := uleb(uint64(len(s)))
	return append(out, []byte(s)...)
}

func vec(items [][]byte) []byte {
	out := uleb(uint64(len(items)))
	for _, it := range items {
		out = append(out, it...)
	}
	return out
}

func section(id byte, payload []byte) []byte {
	out := []byte{id}
	out = append(out, uleb(uint64(len(payload)))...)
	return append(out, payload...)
}

// Bytes 输出完整模块二进制（节顺序按 id 升序：type/import/function/memory/export/code/data）。
func (m *Module) Bytes() []byte {
	out := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	tItems := make([][]byte, 0, len(m.types))
	for _, t := range m.types {
		tb := []byte{0x60}
		tb = append(tb, uleb(uint64(len(t.params)))...)
		tb = append(tb, t.params...)
		tb = append(tb, uleb(uint64(len(t.results)))...)
		tb = append(tb, t.results...)
		tItems = append(tItems, tb)
	}
	out = append(out, section(0x01, vec(tItems))...)

	iItems := make([][]byte, 0, len(m.imports))
	for _, imp := range m.imports {
		ib := nameBytes(imp.mod)
		ib = append(ib, nameBytes(imp.field)...)
		ib = append(ib, 0x00)
		ib = append(ib, uleb(uint64(imp.typeIdx))...)
		iItems = append(iItems, ib)
	}
	out = append(out, section(0x02, vec(iItems))...)

	fItems := make([][]byte, 0, len(m.funcs))
	for _, ti := range m.funcs {
		fItems = append(fItems, uleb(uint64(ti)))
	}
	out = append(out, section(0x03, vec(fItems))...)

	// memory：1 个，min 1 页（64KiB；夹具所有静态地址 < 65536）
	out = append(out, section(0x05, vec([][]byte{{0x00, 0x01}}))...)

	eItems := make([][]byte, 0, len(m.exports))
	for _, e := range m.exports {
		eb := nameBytes(e.name)
		eb = append(eb, e.kind)
		eb = append(eb, uleb(uint64(e.idx))...)
		eItems = append(eItems, eb)
	}
	out = append(out, section(0x07, vec(eItems))...)

	cItems := make([][]byte, 0, len(m.bodies))
	for _, instrs := range m.bodies {
		content := []byte{0x00} // locals: 0 项
		content = append(content, instrs...)
		content = append(content, opEnd)
		c := uleb(uint64(len(content)))
		c = append(c, content...)
		cItems = append(cItems, c)
	}
	out = append(out, section(0x0a, vec(cItems))...)

	dItems := make([][]byte, 0, len(m.datas))
	for _, d := range m.datas {
		db := uleb(0) // memidx
		db = append(db, opI32Const)
		db = append(db, sleb(int32(d.off))...)
		db = append(db, opEnd)
		db = append(db, uleb(uint64(len(d.b)))...)
		db = append(db, d.b...)
		dItems = append(dItems, db)
	}
	out = append(out, section(0x0b, vec(dItems))...)

	return out
}

// ---- 指令助手 ----

func I32Const(v int32) []byte { return append([]byte{opI32Const}, sleb(v)...) }
func CallF(idx uint32) []byte { return append([]byte{opCall}, uleb(uint64(idx))...) }

var Drop = []byte{opDrop}

// LoopForever：loop br 0 的死循环，配合 WithCloseOnContextDone 验证超时杀实例。
var LoopForever = []byte{opLoop, blockEmpty, opBr, 0x00, opEnd}

func instrs(parts ...[]byte) []byte {
	var o []byte
	for _, p := range parts {
		o = append(o, p...)
	}
	return o
}
```

- [ ] **Step 2: 实现夹具 `plugin/runtime/wasmfix/fixtures.go`**

```go
package wasmfix

import "encoding/binary"

// 静态内存布局（全部夹具共用；地址为 uleb/sleb 友好的小常数）：
const (
	RespHdr  = 60 // [u32LE len][body@RespBody]
	RespBody = 64
	Scratch  = 128 // alloc 返回值；夹具不读请求，固定一块即可
	Key1     = 200 // "k"
	Val1     = 201 // "v"
	Key2     = 202 // "k2"
	Val2     = 204 // "v2"
	OptJSON  = 210 // {"a":1}
	LogText  = 220 // "hi"
	RandBuf  = 240 // random 的目标缓冲(16B)
)

const echoResp = `{"ok":true,"data":"echo"}`

func frameBuf(data []byte) []byte {
	buf := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint32(buf, uint32(len(data)))
	copy(buf[4:], data)
	return buf
}

// addCommon：alloc/free/abi_version + 响应数据段 + memory 导出。
func addCommon(m *Module) {
	alloc := m.Func([]byte{I32}, []byte{I32}, I32Const(Scratch))
	m.ExportFunc("alloc", alloc)
	free := m.Func([]byte{I32}, nil, nil)
	m.ExportFunc("free", free)
	abi := m.Func(nil, []byte{I32}, I32Const(1)) // ABIVersion == 1
	m.ExportFunc("abi_version", abi)
	m.Data(RespHdr, frameBuf([]byte(echoResp)))
	m.ExportMemory("memory")
}

// Echo：无 host 导入；handle 恒返回 {"ok":true,"data":"echo"} 帧。
func Echo() []byte {
	m := New()
	addCommon(m)
	h := m.Func([]byte{I32, I32}, []byte{I32}, I32Const(RespHdr))
	m.ExportFunc("handle", h)
	return m.Bytes()
}

// ZeroHandle：handle 返回 0 —— 显式失败语义（spec §3.2）。
func ZeroHandle() []byte {
	m := New()
	addCommon(m)
	h := m.Func([]byte{I32, I32}, []byte{I32}, I32Const(0))
	m.ExportFunc("handle", h)
	return m.Bytes()
}

// Hang：handle 死循环 —— 验证超时关闭与实例重建。
func Hang() []byte {
	m := New()
	addCommon(m)
	h := m.Func([]byte{I32, I32}, []byte{I32}, LoopForever)
	m.ExportFunc("handle", h)
	return m.Bytes()
}

// Malicious：导入 airpress.kv_set 并调用。未授予 kv 权限时实例化必须失败（权限=接口）。
func Malicious() []byte {
	m := New()
	addCommon(m)
	kvSet := m.ImportFunc("airpress", "kv_set", []byte{I32, I32, I32, I32}, []byte{I32})
	h := m.Func([]byte{I32, I32}, []byte{I32},
		instrs(I32Const(Key1), I32Const(1), I32Const(Val1), I32Const(1), CallF(kvSet), Drop,
			I32Const(RespHdr)))
	m.ExportFunc("handle", h)
	m.Data(Key1, []byte("k"))
	m.Data(Val1, []byte("v"))
	return m.Bytes()
}

// Kvsink：把 kv/option/log 各调一遍（含 get 未命中、del、list），结果 drop；
// host 侧副作用由测试从 DB 断言。option_set 写入 {"a":1}。
func Kvsink() []byte {
	m := New()
	addCommon(m)
	kvSet := m.ImportFunc("airpress", "kv_set", []byte{I32, I32, I32, I32}, []byte{I32})
	kvGet := m.ImportFunc("airpress", "kv_get", []byte{I32, I32}, []byte{I32})
	kvDel := m.ImportFunc("airpress", "kv_del", []byte{I32, I32}, []byte{I32})
	kvList := m.ImportFunc("airpress", "kv_list", []byte{I32, I32}, []byte{I32})
	optSet := m.ImportFunc("airpress", "option_set", []byte{I32, I32}, []byte{I32})
	logF := m.ImportFunc("airpress", "log", []byte{I32, I32, I32}, nil)
	body := instrs(
		I32Const(Key1), I32Const(1), I32Const(Val1), I32Const(1), CallF(kvSet), Drop,
		I32Const(Key1), I32Const(1), CallF(kvGet), Drop, // set 后 get 命中，返回值丢弃
		I32Const(Key1), I32Const(1), CallF(kvDel), Drop,
		I32Const(Key1), I32Const(1), CallF(kvGet), Drop, // 再 get：未命中 → 0
		I32Const(Key2), I32Const(2), I32Const(Val2), I32Const(2), CallF(kvSet), Drop,
		I32Const(Key1), I32Const(1), CallF(kvList), Drop,
		I32Const(OptJSON), I32Const(7), CallF(optSet), Drop,
		I32Const(0), I32Const(LogText), I32Const(2), CallF(logF),
		I32Const(RespHdr))
	h := m.Func([]byte{I32, I32}, []byte{I32}, body)
	m.ExportFunc("handle", h)
	m.Data(Key1, []byte("k"))
	m.Data(Val1, []byte("v"))
	m.Data(Key2, []byte("k2"))
	m.Data(Val2, []byte("v2"))
	m.Data(OptJSON, []byte(`{"a":1}`))
	m.Data(LogText, []byte("hi"))
	return m.Bytes()
}

// Clock：调用无需授权的 now/random，验证这两个导出的签名与可执行性。
func Clock() []byte {
	m := New()
	addCommon(m)
	now := m.ImportFunc("airpress", "now", nil, []byte{I64})
	rnd := m.ImportFunc("airpress", "random", []byte{I32, I32}, nil)
	body := instrs(CallF(now), Drop, I32Const(RandBuf), I32Const(16), CallF(rnd), I32Const(RespHdr))
	h := m.Func([]byte{I32, I32}, []byte{I32}, body)
	m.ExportFunc("handle", h)
	return m.Bytes()
}
```

- [ ] **Step 3: 写测试 `plugin/runtime/wasmfix/fixtures_test.go`**

```go
package wasmfix

import (
	"context"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
)

func compiles(t *testing.T, name string, bin []byte) {
	t.Helper()
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	if _, err := rt.CompileModule(ctx, bin); err != nil {
		t.Fatalf("%s does not compile: %v", name, err)
	}
}

func TestFixturesCompile(t *testing.T) {
	compiles(t, "Echo", Echo())
	compiles(t, "ZeroHandle", ZeroHandle())
	compiles(t, "Hang", Hang())
	compiles(t, "Malicious", Malicious())
	compiles(t, "Kvsink", Kvsink())
	compiles(t, "Clock", Clock())
}

func TestEchoInstantiatesWithoutHost(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	cm, err := rt.CompileModule(ctx, Echo())
	if err != nil {
		t.Fatal(err)
	}
	mod, err := rt.InstantiateModule(ctx, cm, wazero.NewModuleConfig().WithName("e").WithStartFunctions())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"alloc", "free", "handle", "abi_version"} {
		if mod.ExportedFunction(n) == nil {
			t.Fatalf("missing export %s", n)
		}
	}
	rets, err := mod.ExportedFunction("abi_version").Call(ctx)
	if err != nil || rets[0] != 1 {
		t.Fatalf("abi: %v %d", err, rets)
	}
}

// Kvsink 没有任何 host module 时实例化必须失败，且失败原因是链接 airpress.* 导入失败。
func TestLinkFailureWithoutHost(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	cm, err := rt.CompileModule(ctx, Kvsink())
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.InstantiateModule(ctx, cm, wazero.NewModuleConfig().WithName("k").WithStartFunctions())
	if err == nil {
		t.Fatal("expected link failure")
	}
	if !strings.Contains(err.Error(), "airpress") {
		t.Fatalf("want link error mentioning airpress, got %v", err)
	}
}

func TestFrameBuf(t *testing.T) {
	got := frameBuf([]byte("ab"))
	want := []byte{2, 0, 0, 0, 'a', 'b'}
	if string(got) != string(want) {
		t.Fatalf("frameBuf=%v", got)
	}
}
```

- [ ] **Step 4: 运行**

```bash
go test ./plugin/runtime/wasmfix/ -v
```
预期：全 PASS。若 wazero 对某夹具报校验错误，按报错定位 builder 编码（section id/LEB/index 是常见错点），修 builder 而不是改测试断言。

- [ ] **Step 5: Commit**

```bash
git add plugin/runtime/wasmfix/
git commit -m "test(plugin): 纯 Go 微型 wasm 构造器与 ABI 测试夹具"
```

---

### Task 7: host 包（权限门控的宿主能力）

**Files:**
- Create: `plugin/host/host.go`
- Test: `plugin/host/host_test.go`

- [ ] **Step 1: 写失败测试（仅纯函数部分；行为路径在 Task 8 用夹具端到端覆盖）**

创建 `plugin/host/host_test.go`：

```go
package host

import (
	"bytes"
	"testing"
)

func TestEncodeFrame(t *testing.T) {
	var buf bytes.Buffer
	if err := encodeFrame(&buf, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	want := []byte{2, 0, 0, 0, 'h', 'i'}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %v", buf.Bytes())
	}
}

func TestLevelToLog(t *testing.T) {
	if levelName(0) != "info" || levelName(1) != "warn" || levelName(2) != "error" || levelName(9) != "info" {
		t.Fatal("level mapping wrong")
	}
}
```

- [ ] **Step 2: 运行确认失败** → `undefined: encodeFrame`。

- [ ] **Step 3: 实现 `plugin/host/host.go`**

```go
// Package host 实现 spec §4.1 的 wazero host module：权限门控的宿主能力。
// 安全前提：host 函数是本系统真正的攻击面，一切来自 wasm 内存的 (ptr,len) 先做长度
// 上限校验再读取；返回值统一用整数错误码（0 成功），不跨边界 panic。
package host

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"

	"github.com/hipoint-airpress/airpress/plugin/manifest"
	"github.com/hipoint-airpress/airpress/plugin/store"
)

const ModuleName = "airpress"

// 返回码约定（spec §3.2 host 函数失败语义）
const (
	rcOK       uint32 = 0
	rcBadArg   uint32 = 1
	rcNotFound uint32 = 2 // kv_get/option_get 之外，调用方把 ptr==0 也视作未命中
	rcInternal uint32 = 3
)

const (
	maxKVKey   = 512
	maxKVValue = 1 << 20 // 1MiB
	maxOption  = 1 << 20
	maxList    = 500
	maxLogLen  = 8 << 10
	maxRandom  = 1024
)

// Deps 是每插件绑定的能力依赖；命名空间键在构造时固化，插件无法伪造（spec §4.1）。
type Deps struct {
	Repo       *store.Repo
	PluginID   int32
	PluginName string
	Logger     *zap.Logger
}

func (d Deps) optionKey() string { return manifest.OptionKey(d.PluginName) }

// Build 按授权集合构造并实例化本插件专属的 host module。
// 未授权的能力**根本不存在**，越权 import 在链接期失败——权限即接口。
// log/now/random 无需授权（spec §4.1 表）。
func Build(ctx context.Context, rt wazero.Runtime, d Deps, granted map[manifest.Permission]bool) error {
	b := rt.NewHostModuleBuilder(ModuleName)
	b.NewFunctionBuilder().WithFunc(makeLog(d)).Export("log")
	b.NewFunctionBuilder().WithFunc(func() uint64 { return uint64(time.Now().UnixMilli()) }).Export("now")
	b.NewFunctionBuilder().WithFunc(makeRandom(d)).Export("random")
	if granted[manifest.PermKV] {
		b.NewFunctionBuilder().WithFunc(makeKVSet(d)).Export("kv_set")
		b.NewFunctionBuilder().WithFunc(makeKVGet(d)).Export("kv_get")
		b.NewFunctionBuilder().WithFunc(makeKVDel(d)).Export("kv_del")
		b.NewFunctionBuilder().WithFunc(makeKVList(d)).Export("kv_list")
	}
	if granted[manifest.PermOptionSelf] {
		b.NewFunctionBuilder().WithFunc(makeOptionGet(d)).Export("option_get")
		b.NewFunctionBuilder().WithFunc(makeOptionSet(d)).Export("option_set")
	}
	_, err := b.Instantiate(ctx)
	return err
}

func levelName(level uint32) string {
	switch level {
	case 1:
		return "warn"
	case 2:
		return "error"
	default:
		return "info"
	}
}

func encodeFrame(w *bytes.Buffer, data []byte) error {
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// readChecked 校验长度上限后从 wasm 线性内存读取（超限/越界都返回 false）。
func readChecked(mod api.Module, ptr, size uint32, max int) ([]byte, bool) {
	if size == 0 || int(size) > max {
		return nil, false
	}
	return mod.Memory().Read(ptr, size)
}

// writeFrame 经插件导出的 alloc 在其堆内分配，写入 [u32LE len][data]，返回 ptr。
// 宿主在 host 函数内回调插件导出是 wazero 支持的重入（Extism 同款模式）。
func writeFrame(ctx context.Context, mod api.Module, data []byte) uint32 {
	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return 0
	}
	rets, err := alloc.Call(ctx, uint64(len(data)+4))
	if err != nil || len(rets) == 0 || rets[0] == 0 {
		return 0
	}
	ptr := uint32(rets[0])
	var buf bytes.Buffer
	if err := encodeFrame(&buf, data); err != nil {
		return 0
	}
	if !mod.Memory().Write(ptr, buf.Bytes()) {
		return 0
	}
	return ptr
}

func makeLog(d Deps) func(ctx context.Context, mod api.Module, level, p, l uint32) {
	return func(ctx context.Context, mod api.Module, level, p, l uint32) {
		msg, ok := readChecked(mod, p, l, maxLogLen)
		if !ok {
			return
		}
		d.Logger.Info("plugin:"+levelName(level),
			zap.String("plugin", d.PluginName), zap.String("msg", string(msg)))
	}
}

func makeRandom(d Deps) func(ctx context.Context, mod api.Module, p, l uint32) uint32 {
	return func(ctx context.Context, mod api.Module, p, l uint32) uint32 {
		if l == 0 || l > maxRandom {
			return rcBadArg
		}
		buf := make([]byte, l)
		if _, err := rand.Read(buf); err != nil {
			d.Logger.Error("random", zap.Error(err))
			return rcInternal
		}
		if !mod.Memory().Write(p, buf) {
			return rcBadArg
		}
		return rcOK
	}
}

func makeKVSet(d Deps) func(ctx context.Context, mod api.Module, kp, kl, vp, vl uint32) uint32 {
	return func(ctx context.Context, mod api.Module, kp, kl, vp, vl uint32) uint32 {
		key, ok := readChecked(mod, kp, kl, maxKVKey)
		if !ok {
			return rcBadArg
		}
		val, ok := readChecked(mod, vp, vl, maxKVValue)
		if !ok {
			return rcBadArg
		}
		if err := d.Repo.KVSet(ctx, d.PluginID, string(key), string(val)); err != nil {
			d.Logger.Error("kv_set", zap.Error(err))
			return rcInternal
		}
		return rcOK
	}
}

func makeKVGet(d Deps) func(ctx context.Context, mod api.Module, kp, kl uint32) uint32 {
	return func(ctx context.Context, mod api.Module, kp, kl uint32) uint32 {
		key, ok := readChecked(mod, kp, kl, maxKVKey)
		if !ok {
			return 0
		}
		val, err := d.Repo.KVGet(ctx, d.PluginID, string(key))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				d.Logger.Error("kv_get", zap.Error(err))
			}
			return 0 // 未命中与内部错误对插件都表现为"空"
		}
		return writeFrame(ctx, mod, []byte(val))
	}
}

func makeKVDel(d Deps) func(ctx context.Context, mod api.Module, kp, kl uint32) uint32 {
	return func(ctx context.Context, mod api.Module, kp, kl uint32) uint32 {
		key, ok := readChecked(mod, kp, kl, maxKVKey)
		if !ok {
			return rcBadArg
		}
		if err := d.Repo.KVDel(ctx, d.PluginID, string(key)); err != nil {
			d.Logger.Error("kv_del", zap.Error(err))
			return rcInternal
		}
		return rcOK
	}
}

func makeKVList(d Deps) func(ctx context.Context, mod api.Module, pp, pl uint32) uint32 {
	return func(ctx context.Context, mod api.Module, pp, pl uint32) uint32 {
		var prefix string
		if pl > 0 {
			p, ok := readChecked(mod, pp, pl, maxKVKey)
			if !ok {
				return 0
			}
			prefix = string(p)
		}
		rows, err := d.Repo.KVList(ctx, d.PluginID, prefix, maxList)
		if err != nil {
			d.Logger.Error("kv_list", zap.Error(err))
			return 0
		}
		out := make([][2]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, [2]string{r.Key, r.Value})
		}
		data, err := json.Marshal(out)
		if err != nil {
			return 0
		}
		return writeFrame(ctx, mod, data)
	}
}

func makeOptionGet(d Deps) func(ctx context.Context, mod api.Module) uint32 {
	return func(ctx context.Context, mod api.Module) uint32 {
		val, err := d.Repo.OptionGet(ctx, d.optionKey())
		if err != nil {
			return 0
		}
		return writeFrame(ctx, mod, []byte(val))
	}
}

func makeOptionSet(d Deps) func(ctx context.Context, mod api.Module, p, l uint32) uint32 {
	return func(ctx context.Context, mod api.Module, p, l uint32) uint32 {
		raw, ok := readChecked(mod, p, l, maxOption)
		if !ok {
			return rcBadArg
		}
		if !json.Valid(raw) {
			return rcBadArg
		}
		if err := d.Repo.OptionSet(ctx, d.optionKey(), string(raw)); err != nil {
			d.Logger.Error("option_set", zap.Error(err))
			return rcInternal
		}
		return rcOK
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./plugin/host/ -v` → PASS（行为端到端覆盖在 Task 8）。

- [ ] **Step 5: Commit**

```bash
git add plugin/host/
git commit -m "feat(plugin): 权限门控的宿主能力(kv/log/now/random/option.self)"
```

---

### Task 8: runtime 实例（握手/调用/超时/重建）

**Files:**
- Create: `plugin/runtime/instance.go`
- Test: `plugin/runtime/instance_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/runtime/instance_test.go`：

```go
package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/plugin/host"
	"github.com/hipoint-airpress/airpress/plugin/manifest"
	"github.com/hipoint-airpress/airpress/plugin/runtime/wasmfix"
	"github.com/hipoint-airpress/airpress/plugin/store"
)

func newDeps(t *testing.T) (host.Deps, *store.Repo) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Plugin{}, &entity.PluginKV{}, &entity.Option{}); err != nil {
		t.Fatal(err)
	}
	repo := store.NewRepo(db)
	return host.Deps{Repo: repo, PluginID: 7, PluginName: "fx", Logger: zap.NewNop()}, repo
}

func TestEchoRoundTrip(t *testing.T) {
	ctx := context.Background()
	deps, _ := newDeps(t)
	inst, err := NewInstance(ctx, Config{Name: "echo", Wasm: wasmfix.Echo(), Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(ctx)
	req, _ := NewRequest("r1", KindInit, "", nil, nil)
	resp, err := inst.Call(ctx, req, time.Second)
	if err != nil || !resp.OK || string(resp.Data) != `"echo"` {
		t.Fatalf("resp: %v %+v", err, resp)
	}
}

func TestMaliciousDeniedByLinking(t *testing.T) {
	ctx := context.Background()
	deps, _ := newDeps(t)
	if _, err := NewInstance(ctx, Config{Name: "m", Wasm: wasmfix.Malicious(), Deps: deps}); err == nil {
		t.Fatal("no-perm grant must fail at instantiation")
	}
	granted := map[manifest.Permission]bool{manifest.PermKV: true}
	inst, err := NewInstance(ctx, Config{Name: "m", Wasm: wasmfix.Malicious(), Deps: deps, Granted: granted})
	if err != nil {
		t.Fatalf("with kv grant: %v", err)
	}
	inst.Close(ctx)
}

func TestKvsinkSideEffects(t *testing.T) {
	ctx := context.Background()
	deps, repo := newDeps(t)
	granted := map[manifest.Permission]bool{manifest.PermKV: true, manifest.PermOptionSelf: true}
	inst, err := NewInstance(ctx, Config{Name: "fx", Wasm: wasmfix.Kvsink(), Deps: deps, Granted: granted})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(ctx)
	req, _ := NewRequest("r", KindEvent, "post.updated", nil, nil)
	resp, err := inst.Call(ctx, req, 2*time.Second)
	if err != nil || !resp.OK {
		t.Fatalf("call: %v %+v", err, resp)
	}
	// kvsink: set k=v → get → del k → get(未命中) → set k2=v2 → list → option_set
	if _, err := repo.KVGet(ctx, 7, "k"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("k must be deleted")
	}
	if v, err := repo.KVGet(ctx, 7, "k2"); err != nil || v != "v2" {
		t.Fatalf("k2: %v %q", err, v)
	}
	if v, err := repo.OptionGet(ctx, manifest.OptionKey("fx")); err != nil || v != `{"a":1}` {
		t.Fatalf("option: %v %q", err, v)
	}
}

func TestExplicitFailureNoRebuild(t *testing.T) {
	ctx := context.Background()
	deps, _ := newDeps(t)
	inst, err := NewInstance(ctx, Config{Name: "z", Wasm: wasmfix.ZeroHandle(), Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(ctx)
	req, _ := NewRequest("r", KindAPI, "x", nil, nil)
	for i := 0; i < 2; i++ {
		if _, err := inst.Call(ctx, req, time.Second); !errors.Is(err, ErrPluginFailure) {
			t.Fatalf("want ErrPluginFailure, got %v", err)
		}
		if inst.mod == nil {
			t.Fatal("explicit failure must NOT discard the instance (spec §3.2)")
		}
	}
}

func TestTimeoutClosesAndRebuilds(t *testing.T) {
	ctx := context.Background()
	deps, _ := newDeps(t)
	inst, err := NewInstance(ctx, Config{Name: "h", Wasm: wasmfix.Hang(), Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(ctx)
	req, _ := NewRequest("r", KindEvent, "", nil, nil)
	if _, err := inst.Call(ctx, req, 100*time.Millisecond); err == nil {
		t.Fatal("hang must time out")
	}
	if inst.mod != nil {
		t.Fatal("timeout must discard module for lazy rebuild")
	}
	// 第二次调用：重建同名模块不得报 "name in use"（证明旧模块已被释放）
	if _, err := inst.Call(ctx, req, 100*time.Millisecond); err == nil {
		t.Fatal("second call must time out too")
	} else if errors.Is(err, ErrMissingExport) {
		t.Fatalf("rebuild broke exports: %v", err)
	}
}

func TestClockNoPermsNeeded(t *testing.T) {
	ctx := context.Background()
	deps, _ := newDeps(t)
	inst, err := NewInstance(ctx, Config{Name: "c", Wasm: wasmfix.Clock(), Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(ctx)
	req, _ := NewRequest("r", KindEvent, "", nil, nil)
	if resp, err := inst.Call(ctx, req, time.Second); err != nil || !resp.OK {
		t.Fatalf("clock: %v %+v", err, resp)
	}
}
```

- [ ] **Step 2: 运行确认失败** → `undefined: NewInstance`。

- [ ] **Step 3: 实现 `plugin/runtime/instance.go`**

```go
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/hipoint-airpress/airpress/plugin/host"
	"github.com/hipoint-airpress/airpress/plugin/manifest"
)

var (
	ErrABI           = errors.New("plugin: abi_version mismatch")
	ErrPluginFailure = errors.New("plugin: handle returned explicit failure (res_ptr=0)")
	ErrMissingExport = errors.New("plugin: missing required export")
)

// Config 是一个插件实例的全部输入。
type Config struct {
	Name        string
	Wasm        []byte
	Granted     map[manifest.Permission]bool
	Deps        host.Deps
	MemoryPages uint32 // 0 → 2048（128MB，spec §3.1）
}

// Instance 每插件独占一个 wazero.Runtime：host module 名固定 "airpress"，
// 权限集合逐插件不同，同进程多插件必须各自成域。
// 单长驻实例 + Mutex 串行；trap/超时 → 丢弃模块，下次调用惰性重建（spec §3.1）。
type Instance struct {
	cfg Config
	rt  wazero.Runtime
	cm  api.CompiledModule

	mu  sync.Mutex
	mod api.Module // nil = 待重建
}

func NewInstance(ctx context.Context, cfg Config) (*Instance, error) {
	if cfg.MemoryPages == 0 {
		cfg.MemoryPages = 2048
	}
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithMemoryLimitPages(cfg.MemoryPages).
		WithCloseOnContextDone(true)) // 超时/取消能杀死执行中的 wasm（spec §3.4）
	if err := host.Build(ctx, rt, cfg.Deps, cfg.Granted); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("host module: %w", err)
	}
	cm, err := rt.CompileModule(ctx, cfg.Wasm)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("compile: %w", err)
	}
	i := &Instance{cfg: cfg, rt: rt, cm: cm}
	if err := i.instantiate(ctx); err != nil {
		_ = rt.Close(ctx)
		return nil, err
	}
	rets, err := callExport(ctx, i.mod, "abi_version")
	if err != nil {
		_ = rt.Close(ctx)
		return nil, err
	}
	if len(rets) == 0 || uint32(rets[0]) != ABIVersion {
		_ = rt.Close(ctx)
		return nil, ErrABI
	}
	return i, nil
}

// instantiate 不自动跑任何启动函数（reactor 语义），随后按需显式调 _initialize。
func (i *Instance) instantiate(ctx context.Context) error {
	mod, err := i.rt.InstantiateModule(ctx, i.cm, wazero.NewModuleConfig().
		WithName("plugin."+i.cfg.Name).
		WithStartFunctions())
	if err != nil {
		return fmt.Errorf("instantiate/link: %w", err)
	}
	for _, n := range []string{"alloc", "free", "handle", "abi_version"} {
		if mod.ExportedFunction(n) == nil {
			_ = mod.Close(ctx)
			return fmt.Errorf("%w: %s", ErrMissingExport, n)
		}
	}
	if fn := mod.ExportedFunction("_initialize"); fn != nil { // 官方 Go wasip1 c-shared 插件走这里（spec §1 修订）
		if _, err := fn.Call(ctx); err != nil {
			_ = mod.Close(ctx)
			return fmt.Errorf("_initialize: %w", err)
		}
	}
	i.mod = mod
	return nil
}

// Call 一次完整信封往返（spec §3.2 内存协议）。timeout 覆盖整个调用（含重建）。
func (i *Instance) Call(ctx context.Context, req *Request, timeout time.Duration) (*Response, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if i.mod == nil {
		if err := i.instantiate(cctx); err != nil {
			return nil, err
		}
	}
	rets, err := callExport(cctx, i.mod, "alloc", uint64(len(data)))
	if err != nil || len(rets) == 0 || rets[0] == 0 {
		i.discard(cctx)
		return nil, fmt.Errorf("alloc: err=%v", err)
	}
	reqPtr := uint32(rets[0])
	if !i.mod.Memory().Write(reqPtr, data) {
		i.discard(cctx)
		return nil, errors.New("request out of plugin memory")
	}
	res, err := callExport(cctx, i.mod, "handle", uint64(reqPtr), uint64(len(data)))
	if freeErr := callExportIgnore(cctx, i.mod, "free", uint64(reqPtr)); freeErr != nil {
		i.discard(cctx) // 模块多半已被超时关闭
		if err != nil {
			return nil, fmt.Errorf("handle: %w", err)
		}
		return nil, fmt.Errorf("free(req): %w", freeErr)
	}
	if err != nil { // trap/超时：实例可能脏，丢弃并惰性重建（spec §3.1）
		i.discard(cctx)
		return nil, fmt.Errorf("handle: %w", err)
	}
	resPtr := uint32(res[0])
	if resPtr == 0 { // 显式失败：等价 {ok:false}，不触发重建（spec §3.2）
		return nil, ErrPluginFailure
	}
	n, ok := i.mod.Memory().ReadUint32Le(resPtr)
	if !ok {
		i.discard(cctx)
		return nil, errors.New("bad response header")
	}
	body, ok := i.mod.Memory().Read(resPtr+4, n)
	if !ok {
		i.discard(cctx)
		return nil, errors.New("response out of memory")
	}
	if err := callExportIgnore(cctx, i.mod, "free", uint64(resPtr)); err != nil {
		i.discard(cctx)
		return nil, fmt.Errorf("free(res): %w", err)
	}
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		i.discard(cctx)
		return nil, fmt.Errorf("bad response json: %w", err)
	}
	return &resp, nil
}

func (i *Instance) discard(ctx context.Context) {
	if i.mod != nil {
		_ = i.mod.Close(ctx)
		i.mod = nil
	}
}

// Close 关闭整个运行时（编译缓存/内存全部释放）。
func (i *Instance) Close(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.discard(ctx)
	return i.rt.Close(ctx)
}

func callExport(ctx context.Context, mod api.Module, name string, args ...uint64) ([]uint64, error) {
	fn := mod.ExportedFunction(name)
	if fn == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExport, name)
	}
	return fn.Call(ctx, args...)
}

func callExportIgnore(ctx context.Context, mod api.Module, name string, args ...uint64) error {
	_, err := callExport(ctx, mod, name, args...)
	return err
}
```

注意实现细节：`manifest` 仅用于类型 `Permission`，`instance_test` 用到 `store`/`entity`；
若 `go vet` 提示 instance.go 里 `manifest` 未使用（Config.Granted 已用到），删多余 import 即可。

- [ ] **Step 4: 运行**

```bash
go test ./plugin/runtime/ -v -count=1
```
预期：全 PASS。`TestTimeoutClosesAndRebuilds` 若因 `WithCloseOnContextDone` 未关闭模块而失败，
检查是否误用了 `wazero.NewRuntime`（没有 config）——必须走 `NewRuntimeWithConfig`。

- [ ] **Step 5: Commit**

```bash
git add plugin/runtime/instance.go plugin/runtime/instance_test.go
git commit -m "feat(plugin): wazero 插件实例（握手/信封调用/超时重建）"
```

---

### Task 9: zipx 安全解包

**Files:**
- Create: `plugin/zipx/zipx.go`
- Test: `plugin/zipx/zipx_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/zipx/zipx_test.go`：

```go
package zipx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractHappy(t *testing.T) {
	data := mkZip(t, map[string][]byte{
		"plugin.yaml": []byte("kind: Plugin\n"),
		"main.wasm":   []byte("\x00asm\x01\x00\x00\x00"),
		"static/x.css": []byte("body{}"),
	})
	dest := t.TempDir()
	if err := Extract(data, dest); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin.yaml", "main.wasm", filepath.Join("static", "x.css")} {
		if _, err := os.Stat(filepath.Join(dest, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
}

func TestExtractRejectsTraversalAndAbsolute(t *testing.T) {
	dest := t.TempDir()
	for _, name := range []string{"../evil.txt", "/abs/evil.txt", "a/../../evil.txt"} {
		data := mkZip(t, map[string][]byte{name: []byte("x")})
		if err := Extract(data, dest); err == nil || !strings.Contains(err.Error(), "unsafe") {
			t.Fatalf("entry %q: want unsafe error, got %v", name, err)
		}
	}
}

func TestFindManifestAndReadFile(t *testing.T) {
	data := mkZip(t, map[string][]byte{"plugin.yaml": []byte("A: 1"), "main.wasm": []byte("WASM")})
	mf, err := FindManifest(data)
	if err != nil || string(mf) != "A: 1" {
		t.Fatalf("manifest: %v %q", err, mf)
	}
	w, err := ReadFile(data, "main.wasm")
	if err != nil || string(w) != "WASM" {
		t.Fatalf("readfile: %v %q", err, w)
	}
	if _, err := FindManifest(mkZip(t, map[string][]byte{"sub/plugin.yaml": []byte("x")})); err == nil {
		t.Fatal("manifest must be at root")
	}
	if _, err := ReadFile(data, "nope"); err == nil {
		t.Fatal("missing entry error")
	}
}
```

- [ ] **Step 2: 运行确认失败** → `undefined: Extract`。

- [ ] **Step 3: 实现 `plugin/zipx/zipx.go`**

```go
// Package zipx 插件 zip 的安全解包与读取（spec §2.4：解包前完成全部校验）。
package zipx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	MaxEntries  = 1000
	MaxTotal    = int64(50) << 20 // 解包总大小 50MB
	MaxOne      = int64(20) << 20 // 单文件 20MB
	MaxManifest = int64(1) << 20
)

func openReader(data []byte) (*zip.Reader, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("bad zip: %w", err)
	}
	if len(zr.File) > MaxEntries {
		return nil, errors.New("too many zip entries")
	}
	return zr, nil
}

func safePath(name string) (string, error) {
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("unsafe zip entry: NUL in %q", name)
	}
	clean := path.Clean(filepath.ToSlash(name))
	if clean == "." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("unsafe zip entry: %q", name)
	}
	return clean, nil
}

// Extract 解包到 dest。拒绝：路径穿越/绝对路径/超限（声明值与实际字节双重校验，防 zip 炸弹）。
func Extract(data []byte, dest string) error {
	zr, err := openReader(data)
	if err != nil {
		return err
	}
	destClean := filepath.Clean(dest)
	total := int64(0)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name, err := safePath(f.Name)
		if err != nil {
			return err
		}
		if int64(f.UncompressedSize64) > MaxOne {
			return fmt.Errorf("entry %s too large", name)
		}
		total += int64(f.UncompressedSize64)
		if total > MaxTotal {
			return errors.New("zip too large")
		}
		target := filepath.Join(destClean, filepath.FromSlash(name))
		if target != destClean && !strings.HasPrefix(target, destClean+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe zip entry (escaping dest): %q", name)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, MaxOne+1)) // 实际字节可谎报声明值：多读 1 即失败
		rc.Close()
		cerr := out.Close()
		if err != nil {
			return err
		}
		if cerr != nil {
			return cerr
		}
		if n > MaxOne {
			return fmt.Errorf("entry %s exceeds %d bytes", name, MaxOne)
		}
	}
	return nil
}

// FindManifest 读取 zip 根目录的 plugin.yaml。
func FindManifest(data []byte) ([]byte, error) {
	zr, err := openReader(data)
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == "plugin.yaml" && !f.FileInfo().IsDir() {
			return ReadFile(data, "plugin.yaml")
		}
	}
	return nil, errors.New("plugin.yaml not found at zip root")
}

// ReadFile 读取 zip 内指定文件的声明内容（预检编译用）。
func ReadFile(data []byte, name string) ([]byte, error) {
	zr, err := openReader(data)
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == name && !f.FileInfo().IsDir() {
			if int64(f.UncompressedSize64) > MaxOne {
				return nil, fmt.Errorf("entry %s too large", name)
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, MaxOne+1))
		}
	}
	return nil, fmt.Errorf("entry %q not found", name)
}
```

- [ ] **Step 4: 运行确认通过** → PASS。注意 `plugin.yaml` 若因 MaxManifest 未使用报 vet/lint，把它用于 `FindManifest` 前的 `f.UncompressedSize64` 检查或从常量里删除（保留校验语义：

```go
		if f.UncompressedSize64 > uint64(MaxManifest) {
			return nil, errors.New("manifest too large")
		}
```
放在 `return ReadFile(...)` 之前。）

- [ ] **Step 5: Commit**

```bash
git add plugin/zipx/
git commit -m "feat(plugin): 安全的插件 zip 解包（防穿越/炸弹）"
```

---

### Task 10: lifecycle 生命周期服务

**Files:**
- Create: `plugin/lifecycle/service.go`
- Create: `plugin/lifecycle/init.go`
- Test: `plugin/lifecycle/service_test.go`

- [ ] **Step 1: 写失败测试**

创建 `plugin/lifecycle/service_test.go`：

```go
package lifecycle

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/plugin/manifest"
	"github.com/hipoint-airpress/airpress/plugin/runtime"
	"github.com/hipoint-airpress/airpress/plugin/runtime/wasmfix"
	"github.com/hipoint-airpress/airpress/plugin/store"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Plugin{}, &entity.PluginKV{}, &entity.Option{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.AirPress.PluginDir = t.TempDir()
	return &Service{
		repo: store.NewRepo(db), cfg: cfg, logger: zap.NewNop(),
		instances: map[int32]*runtime.Instance{},
	}
}

// 注意：newSvc 里 &config.Config{} + cfg.AirPress.PluginDir 依赖 Task 1 的字段；
// 同包测试可直接初始化私有字段（repo/cfg/logger/instances）。

func mkPluginZip(t *testing.T, mf string, entry string, wasm []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("plugin.yaml")
	w.Write([]byte(mf))
	w2, _ := zw.Create(entry)
	w2.Write(wasm)
	zw.Close()
	return buf.Bytes()
}

const kvManifest = `
apiVersion: airpress/v1alpha1
kind: Plugin
metadata: {name: kv, title: KV, version: 1.0.0}
spec:
  entry: main.wasm
  permissions: [kv, option.self]
`

const echoManifest = `
apiVersion: airpress/v1alpha1
kind: Plugin
metadata: {name: echo, title: Echo, version: 1.0.0}
spec: {entry: main.wasm, permissions: []}
`

func TestInstallEnableDisableUninstall(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	zipData := mkPluginZip(t, kvManifest, "main.wasm", wasmfix.Kvsink())

	p, err := s.Install(ctx, zipData)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != StatusInactive {
		t.Fatalf("fresh install must be inactive, got %s", p.Status)
	}
	if _, err := s.repo.GetPluginByName(ctx, "kv"); err != nil {
		t.Fatal(err)
	}

	if err := s.Enable(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	// enable 的 init 调用执行了 kvsink handle：kv_set("k2","v2") + option_set
	if v, err := s.repo.KVGet(ctx, p.ID, "k2"); err != nil || v != "v2" {
		t.Fatalf("init side effects: %v %q", err, v)
	}
	if v, err := s.repo.OptionGet(ctx, manifest.OptionKey("kv")); err != nil || v != `{"a":1}` {
		t.Fatalf("option: %v %q", err, v)
	}
	if err := s.Disable(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Uninstall(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.repo.GetPluginByID(ctx, p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("row must be gone")
	}
	if _, err := s.repo.OptionGet(ctx, manifest.OptionKey("kv")); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("option must be gone")
	}
}

func TestUninstallActiveRejected(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	p, err := s.Install(ctx, mkPluginZip(t, echoManifest, "main.wasm", wasmfix.Echo()))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Enable(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Uninstall(ctx, p.ID); err == nil {
		t.Fatal("must refuse uninstall of active plugin")
	}
}

func TestEnableDeniedByPermissions(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	mf := `
apiVersion: airpress/v1alpha1
kind: Plugin
metadata: {name: evil, title: Evil, version: 1.0.0}
spec: {entry: main.wasm, permissions: []}
`
	p, err := s.Install(ctx, mkPluginZip(t, mf, "main.wasm", wasmfix.Malicious()))
	if err != nil {
		t.Fatal(err) // 预检只编译不链接，install 应成功
	}
	if err := s.Enable(ctx, p.ID); err == nil {
		t.Fatal("enable must fail on permission link error")
	}
	got, _ := s.repo.GetPluginByID(ctx, p.ID)
	if got.Status != StatusInactive {
		t.Fatal("failed enable must keep inactive")
	}
}

func TestUpdateSettingsNotify(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	p, err := s.Install(ctx, mkPluginZip(t, echoManifest, "main.wasm", wasmfix.Echo()))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Enable(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSettings(ctx, p.ID, []byte(`{"max":3}`)); err != nil {
		t.Fatal(err)
	}
	v, err := s.repo.OptionGet(ctx, manifest.OptionKey("echo"))
	if err != nil || v != `{"max":3}` {
		t.Fatalf("settings: %v %q", err, v)
	}
	if err := s.UpdateSettings(ctx, p.ID, []byte(`[1,2]`)); err == nil {
		t.Fatal("non-object settings rejected")
	}
}

func TestUpgradeSameNameBumpsVersion(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	if _, err := s.Install(ctx, mkPluginZip(t, kvManifest, "main.wasm", wasmfix.Kvsink())); err != nil {
		t.Fatal(err)
	}
	mf2 := `
apiVersion: airpress/v1alpha1
kind: Plugin
metadata: {name: kv, title: KV, version: 1.0.1}
spec: {entry: main.wasm, permissions: [kv]}
`
	p2, err := s.Install(ctx, mkPluginZip(t, mf2, "main.wasm", wasmfix.Kvsink()))
	if err != nil {
		t.Fatal(err)
	}
	if p2.Version != "1.0.1" {
		t.Fatalf("version %s", p2.Version)
	}
	rows, _ := s.repo.ListPlugins(ctx)
	if len(rows) != 1 {
		t.Fatalf("upgrade must reuse row, got %d", len(rows))
	}
}
```

- [ ] **Step 2: 运行确认失败** → `undefined: Service`。

- [ ] **Step 3: 实现 `plugin/lifecycle/service.go`**

```go
// Package lifecycle 编排插件的安装/启用/停用/卸载/设置（spec §2.4 状态机）。
package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tetratelabs/wazero"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/plugin/host"
	"github.com/hipoint-airpress/airpress/plugin/manifest"
	"github.com/hipoint-airpress/airpress/plugin/runtime"
	"github.com/hipoint-airpress/airpress/plugin/store"
	"github.com/hipoint-airpress/airpress/plugin/zipx"
)

const (
	StatusInactive = "inactive"
	StatusActive   = "active"

	initTimeout     = 5 * time.Second
	shutdownTimeout = 3 * time.Second
	settingsTimeout = 5 * time.Second
	closeTimeout    = 5 * time.Second
)

type Service struct {
	repo   *store.Repo
	cfg    *config.Config
	logger *zap.Logger

	mu        sync.Mutex
	instances map[int32]*runtime.Instance
}

// NewService 是 fx 构造器；OnStop 广播 shutdown（spec §3.4）。
// M1 已知行为：应用重启后 active 行仍在库但实例不自动拉起——自动拉起属 M2 事件桥接。
func NewService(db *gorm.DB, cfg *config.Config, logger *zap.Logger, lc fx.Lifecycle) *Service {
	s := &Service{
		repo: store.NewRepo(db), cfg: cfg, logger: logger,
		instances: map[int32]*runtime.Instance{},
	}
	lc.Append(fx.Hook{OnStop: func(ctx context.Context) error {
		s.ShutdownAll(ctx)
		return nil
	}})
	return s
}

func (s *Service) Install(ctx context.Context, zipData []byte) (*entity.Plugin, error) {
	mfRaw, err := zipx.FindManifest(zipData)
	if err != nil {
		return nil, err
	}
	m, err := manifest.Parse(mfRaw)
	if err != nil {
		return nil, err
	}
	if err := preflightCompile(ctx, zipData, m.Spec.Entry); err != nil {
		return nil, err
	}

	dir := filepath.Join(s.cfg.AirPress.PluginDir, m.Metadata.Name, m.Metadata.Version)
	sha := hex.EncodeToString(shaBytes(zipData))

	old, err := s.repo.GetPluginByName(ctx, m.Metadata.Name)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if old != nil && old.Status == StatusActive { // 热升级：先停（spec §2.4）
		if err := s.Disable(ctx, old.ID); err != nil {
			return nil, err
		}
	}
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := zipx.Extract(zipData, dir); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(m.Spec.Entry))); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("entry %q missing in zip", m.Spec.Entry)
	}

	row := &entity.Plugin{
		Name: m.Metadata.Name, Title: m.Metadata.Title, Version: m.Metadata.Version,
		Status: StatusInactive, InstallPath: dir, Sha256: sha, Manifest: string(mfRaw),
	}
	if old != nil {
		row.ID = old.ID
		if err := s.repo.UpdatePlugin(ctx, row); err != nil {
			return nil, err
		}
	} else if err := s.repo.CreatePlugin(ctx, row); err != nil {
		return nil, err
	}
	if old != nil && old.Status == StatusActive {
		if err := s.Enable(ctx, row.ID); err != nil {
			return nil, fmt.Errorf("installed but re-enable failed: %w", err)
		}
	}
	s.logger.Info("plugin installed", zap.String("name", row.Name), zap.String("version", row.Version))
	return s.repo.GetPluginByID(ctx, row.ID)
}

// preflightCompile 上传即试编译（快速失败）；链接与启动在 Enable。
func preflightCompile(ctx context.Context, zipData []byte, entry string) error {
	wasm, err := zipx.ReadFile(zipData, entry)
	if err != nil {
		return fmt.Errorf("entry %q: %w", entry, err)
	}
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	if _, err := rt.CompileModule(ctx, wasm); err != nil {
		return fmt.Errorf("wasm rejected by compiler: %w", err)
	}
	return nil
}

func (s *Service) Enable(ctx context.Context, id int32) error {
	p, err := s.repo.GetPluginByID(ctx, id)
	if err != nil {
		return err
	}
	if p.Status == StatusActive {
		return errors.New("plugin already active")
	}
	m, err := manifest.Parse([]byte(p.Manifest))
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(filepath.Join(p.InstallPath, filepath.FromSlash(m.Spec.Entry)))
	if err != nil {
		return err
	}
	deps := host.Deps{Repo: s.repo, PluginID: p.ID, PluginName: p.Name, Logger: s.logger}
	inst, err := runtime.NewInstance(ctx, runtime.Config{
		Name: p.Name, Wasm: wasm, Granted: m.Granted(), Deps: deps,
	})
	if err != nil {
		return fmt.Errorf("instantiate: %w", err)
	}
	req, _ := runtime.NewRequest(uuid.NewString(), runtime.KindInit, "", s.readSettings(ctx, p.Name), nil)
	if _, err := inst.Call(ctx, req, initTimeout); err != nil {
		_ = inst.Close(ctx)
		return fmt.Errorf("init: %w", err)
	}
	p.Status = StatusActive
	if err := s.repo.UpdatePlugin(ctx, p); err != nil {
		_ = inst.Close(ctx)
		return err
	}
	s.mu.Lock()
	s.instances[id] = inst
	s.mu.Unlock()
	s.logger.Info("plugin enabled", zap.String("name", p.Name))
	return nil
}

func (s *Service) Disable(ctx context.Context, id int32) error {
	p, err := s.repo.GetPluginByID(ctx, id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	inst := s.instances[id]
	delete(s.instances, id)
	s.mu.Unlock()
	if inst != nil {
		req, _ := runtime.NewRequest(uuid.NewString(), runtime.KindShutdown, "", nil, nil)
		if _, err := inst.Call(ctx, req, shutdownTimeout); err != nil {
			s.logger.Warn("shutdown call failed (ignored)", zap.String("plugin", p.Name), zap.Error(err))
		}
		cctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()
		_ = inst.Close(cctx) // 用独立 ctx：HTTP 侧可能已取消
	}
	if p.Status != StatusInactive {
		p.Status = StatusInactive
		return s.repo.UpdatePlugin(ctx, p)
	}
	return nil
}

func (s *Service) Uninstall(ctx context.Context, id int32) error {
	p, err := s.repo.GetPluginByID(ctx, id)
	if err != nil {
		return err
	}
	if p.Status == StatusActive {
		return errors.New("disable before uninstall")
	}
	if err := os.RemoveAll(p.InstallPath); err != nil {
		return err
	}
	if err := s.repo.KVDeleteByPlugin(ctx, p.ID); err != nil {
		return err
	}
	if err := s.repo.OptionDel(ctx, manifest.OptionKey(p.Name)); err != nil {
		return err
	}
	return s.repo.DeletePlugin(ctx, p.ID)
}

func (s *Service) List(ctx context.Context) ([]*entity.Plugin, error) {
	return s.repo.ListPlugins(ctx)
}

func (s *Service) readSettings(ctx context.Context, name string) json.RawMessage {
	v, err := s.repo.OptionGet(ctx, manifest.OptionKey(name))
	if err != nil || v == "" {
		return json.RawMessage("{}")
	}
	return json.RawMessage(v)
}

func (s *Service) GetSettings(ctx context.Context, id int32) (json.RawMessage, error) {
	p, err := s.repo.GetPluginByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.readSettings(ctx, p.Name), nil
}

// UpdateSettings 先持久化再通知活动实例；通知失败只记日志（spec §3.4：设置不回滚）。
func (s *Service) UpdateSettings(ctx context.Context, id int32, raw []byte) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("settings must be a JSON object: %w", err)
	}
	p, err := s.repo.GetPluginByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.OptionSet(ctx, manifest.OptionKey(p.Name), string(raw)); err != nil {
		return err
	}
	s.mu.Lock()
	inst := s.instances[id]
	s.mu.Unlock()
	if inst != nil {
		req, _ := runtime.NewRequest(uuid.NewString(), runtime.KindSettingsChanged, "", json.RawMessage(raw), nil)
		if _, err := inst.Call(ctx, req, settingsTimeout); err != nil {
			s.logger.Warn("settings_changed call failed", zap.String("plugin", p.Name), zap.Error(err))
		}
	}
	return nil
}

func (s *Service) ShutdownAll(ctx context.Context) {
	s.mu.Lock()
	all := s.instances
	s.instances = map[int32]*runtime.Instance{}
	s.mu.Unlock()
	for _, inst := range all {
		req, _ := runtime.NewRequest(uuid.NewString(), runtime.KindShutdown, "", nil, nil)
		_, _ = inst.Call(ctx, req, shutdownTimeout)
		cctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		_ = inst.Close(cctx)
		cancel()
	}
}

func shaBytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
```

- [ ] **Step 4: 实现 `plugin/lifecycle/init.go`**

```go
package lifecycle

import "github.com/hipoint-airpress/airpress/injection"

func init() {
	injection.Provide(NewService)
}
```

- [ ] **Step 5: 运行**

```bash
go test ./plugin/lifecycle/ -v
```
预期：全 PASS。若 `TestUpdateSettingsNotify` 里 echo 插件实例重建（settings_changed 走 handle 恒 OK）无碍；
若 `Enable` 报 `_initialize` 相关错误，说明误用了带默认启动函数的配置——回 Task 8 检查 `WithStartFunctions()`。

- [ ] **Step 6: Commit**

```bash
git add plugin/lifecycle/
git commit -m "feat(plugin): 生命周期服务（安装/启停/卸载/设置/停机广播）"
```

---

### Task 11: 管理端点与路由接线

**Files:**
- Create: `plugin/dto/plugin.go`
- Create: `handler/admin/plugin.go`
- Test: `handler/admin/plugin_test.go`
- Modify: `handler/admin/init.go`（provide 列表）
- Modify: `handler/server.go`（Server 字段约 :47、ServerParams 约 :96、NewServer 赋值约 :170——三处都紧跟 `PostHandler` 同名字段之后插入）
- Modify: `handler/router.go`（optionRouter 块之后，约 :175）

- [ ] **Step 1: 写失败测试 `handler/admin/plugin_test.go`**

```go
package admin

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/plugin/lifecycle"
	"github.com/hipoint-airpress/airpress/plugin/runtime"
	"github.com/hipoint-airpress/airpress/plugin/runtime/wasmfix"
)

func newHandler(t *testing.T) *PluginHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Plugin{}, &entity.PluginKV{}, &entity.Option{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.AirPress.PluginDir = t.TempDir()
	svc := &lifecycle.TestableService(db, cfg) // 见 Step 3 说明：真实实现用 NewService 的 db/cfg/logger 变体
	return NewPluginHandler(svc)
}

func zipOf(t *testing.T, mf string, wasm []byte) []byte {
	var buf bytes.Buffer
	// 复用 lifecycle 测试同款构造
	zw := newZipWriter(&buf)
	writeZipEntry(t, zw, "plugin.yaml", []byte(mf))
	writeZipEntry(t, zw, "main.wasm", wasm)
	zw.Close()
	return buf.Bytes()
}

func TestUploadListEnableDisable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHandler(t)
	ctx, resp := gin.CreateTestContext(httptest.NewRecorder())
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, _ := mw.CreateFormField("file")
	fw.Write(zipOf(t,
		"apiVersion: airpress/v1alpha1\nkind: Plugin\nmetadata: {name: echo, title: E, version: 1.0.0}\nspec: {entry: main.wasm}\n",
		wasmfix.Echo()))
	mw.Close()
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/plugins/upload", body)
	ctx.Request.Header.Set("Content-Type", mw.FormDataContentType())
	out, err := h.Upload(ctx)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	id := out.(*pluginDtoOut).ID

	ctx2, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx2.Params = gin.Params{{Key: "id", Value: "1"}}
	_ = id
	if _, err := h.Enable(ctx2); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, err := h.Disable(ctx2); err != nil {
		t.Fatalf("disable: %v", err)
	}
	ctx3, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx3.Params = gin.Params{{Key: "id", Value: "1"}}
	if _, err := h.Delete(ctx3); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_ = context.Background()
}
```

> **执行者注意（两处需按真实 API 微调，微调不算偏离计划）**：
> ① Step 1 里 `lifecycle.TestableService` 与 `pluginDtoOut`/`newZipWriter` 是**占位伪名**——
> 落地时：`TestableService` 换成直接构造（`lifecycle` 包导出一个
> `NewServiceForTest(db *gorm.DB, cfg *config.Config) *Service` 供 handler 测试，
> 见 Step 3 最后一段），`pluginDtoOut` 换成 `*dto.PluginResp`，
> `newZipWriter/writeZipEntry` 换成 `archive/zip` 标准写法（Task 9 测试里的 `mkZip` 模式，
> 本包内重新实现即可，勿跨包引用测试辅助函数）。
> ② `h.Upload` 的返回类型以 Step 3 实现为准。

- [ ] **Step 2: 运行确认失败** → `undefined: NewPluginHandler`。

- [ ] **Step 3: 实现 DTO、handler、接线**

`plugin/dto/plugin.go`：

```go
// Package dto 插件管理 API 的出参。
package dto

import (
	"time"

	"github.com/hipoint-airpress/airpress/model/entity"
)

type PluginResp struct {
	ID          int32     `json:"id"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	InstallPath string    `json:"install_path"`
	Sha256      string    `json:"sha256"`
	CreateTime  time.Time `json:"create_time"`
}

func PluginFromEntity(p *entity.Plugin) *PluginResp {
	return &PluginResp{
		ID: p.ID, Name: p.Name, Title: p.Title, Version: p.Version, Status: p.Status,
		InstallPath: p.InstallPath, Sha256: p.Sha256, CreateTime: p.CreateTime,
	}
}
```

`handler/admin/plugin.go`：

```go
package admin

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/plugin/dto"
	"github.com/hipoint-airpress/airpress/plugin/lifecycle"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

const maxUploadBytes = 51 << 20 // zipx 内部限额更严，这里只挡失控请求体

type PluginHandler struct {
	PluginService *lifecycle.Service
}

func NewPluginHandler(pluginService *lifecycle.Service) *PluginHandler {
	return &PluginHandler{PluginService: pluginService}
}

// Upload godoc
// @Summary  上传插件 zip
// @Description 解包→manifest 校验→试编译，入库为 inactive
// @Tags     Admin.Plugin
// @Accept   multipart/form-data
// @Produce  json
// @Security AdminApiKey
// @Success  200 {object} dto.BaseDTO{data=dto.PluginResp}
// @Failure  400 {object} dto.BaseDTO
// @Router   /admin/plugins/upload [post]
func (h *PluginHandler) Upload(ctx *gin.Context) (interface{}, error) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("multipart field `file` required")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUploadBytes))
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	p, err := h.PluginService.Install(ctx, data)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(err.Error())
	}
	return dto.PluginFromEntity(p), nil
}

// List godoc
// @Summary      插件列表
// @Tags         Admin.Plugin
// @Produce      json
// @Security     AdminApiKey
// @Success      200 {object} dto.BaseDTO{data=[]dto.PluginResp}
// @Router       /admin/plugins [get]
func (h *PluginHandler) List(ctx *gin.Context) (interface{}, error) {
	rows, err := h.PluginService.List(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]*dto.PluginResp, 0, len(rows))
	for _, r := range rows {
		list = append(list, dto.PluginFromEntity(r))
	}
	return list, nil
}

func pluginID(ctx *gin.Context) (int32, error) {
	raw := ctx.Param("id")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("bad plugin id")
	}
	return int32(id), nil
}

// Enable godoc
// @Summary  启用插件
// @Tags     Admin.Plugin
// @Security AdminApiKey
// @Success  200 {object} dto.BaseDTO
// @Router   /admin/plugins/{id}/enable [post]
func (h *PluginHandler) Enable(ctx *gin.Context) (interface{}, error) {
	id, err := pluginID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.PluginService.Enable(ctx, id); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(err.Error())
	}
	return nil, nil
}

// Disable godoc
// @Summary  停用插件
// @Tags     Admin.Plugin
// @Security AdminApiKey
// @Success  200 {object} dto.BaseDTO
// @Router   /admin/plugins/{id}/disable [post]
func (h *PluginHandler) Disable(ctx *gin.Context) (interface{}, error) {
	id, err := pluginID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.PluginService.Disable(ctx, id); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(err.Error())
	}
	return nil, nil
}

// Delete godoc
// @Summary  卸载插件（须先停用；删行/删 KV/删设置/删目录）
// @Tags     Admin.Plugin
// @Security AdminApiKey
// @Success  200 {object} dto.BaseDTO
// @Router   /admin/plugins/{id} [delete]
func (h *PluginHandler) Delete(ctx *gin.Context) (interface{}, error) {
	id, err := pluginID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.PluginService.Uninstall(ctx, id); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(err.Error())
	}
	return nil, nil
}

// GetSettings godoc
// @Summary  读插件设置（JSON）
// @Tags     Admin.Plugin
// @Security AdminApiKey
// @Success  200 {object} dto.BaseDTO
// @Router   /admin/plugins/{id}/settings [get]
func (h *PluginHandler) GetSettings(ctx *gin.Context) (interface{}, error) {
	id, err := pluginID(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := h.PluginService.GetSettings(ctx, id)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	return raw, nil
}

// UpdateSettings godoc
// @Summary  写插件设置并通知活动实例
// @Tags     Admin.Plugin
// @Accept   json
// @Security AdminApiKey
// @Success  200 {object} dto.BaseDTO
// @Router   /admin/plugins/{id}/settings [put]
func (h *PluginHandler) UpdateSettings(ctx *gin.Context) (interface{}, error) {
	id, err := pluginID(ctx)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	if err := h.PluginService.UpdateSettings(ctx, id, body); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg(err.Error())
	}
	return nil, nil
}
```

在 `plugin/lifecycle/service.go` 底部追加供 handler 测试用的构造（生产路径仍用 NewService）：

```go
// NewServiceForTest 跳过 fx 生命周期注入，等价于生产构造但不注册 OnStop。
func NewServiceForTest(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{
		repo: store.NewRepo(db), cfg: cfg, logger: zap.NewNop(),
		instances: map[int32]*runtime.Instance{},
	}
}
```

`handler/admin/init.go` 的 `injection.Provide(...)` 列表追加 `NewPluginHandler,`。

`handler/server.go` 三处（均紧跟 PostHandler 对应行之后）：
- Server 字段：`	PluginHandler             *admin.PluginHandler`
- ServerParams 字段：`	PluginHandler             *admin.PluginHandler`
- NewServer 赋值：`		PluginHandler:             param.PluginHandler,`

`handler/router.go` 在 optionRouter 块（`optionRouter := authRouter.Group("/options")`
所在的 `{...}`）之后插入：

```go
				{
					pluginRouter := authRouter.Group("/plugins")
					pluginRouter.POST("/upload", s.wrapHandler(s.PluginHandler.Upload))
					pluginRouter.GET("", s.wrapHandler(s.PluginHandler.List))
					pluginRouter.POST("/:id/enable", s.wrapHandler(s.PluginHandler.Enable))
					pluginRouter.POST("/:id/disable", s.wrapHandler(s.PluginHandler.Disable))
					pluginRouter.DELETE("/:id", s.wrapHandler(s.PluginHandler.Delete))
					pluginRouter.GET("/:id/settings", s.wrapHandler(s.PluginHandler.GetSettings))
					pluginRouter.PUT("/:id/settings", s.wrapHandler(s.PluginHandler.UpdateSettings))
				}
```

注意：`handler/admin` 包 import `plugin/lifecycle` → 其 `init.go` 的 `injection.Provide(NewService)`
随包加载自动生效，`main.go` 不需要改动；`plugin/lifecycle` import `plugin/runtime` →
`plugin/host` → `plugin/store`，fx 需要的 `*gorm.DB`/`*config.Config`/`*zap.Logger` 均已在
`main.go` 提供。

- [ ] **Step 4: 按 Step 3 的真实名字修正 Step 1 测试**（`lifecycle.NewServiceForTest(db, cfg)`、
`*dto.PluginResp`、archive/zip 直接构造），运行：

```bash
go test ./handler/admin/ -run Plugin -v
```
预期：PASS。

- [ ] **Step 5: Commit**

```bash
git add plugin/dto/ handler/admin/ handler/server.go handler/router.go
git commit -m "feat(plugin): /api/admin/plugins 管理端点与 fx 接线"
```

---

### Task 12: 全量验证与收尾

- [ ] **Step 1: 静态检查与全量测试**

```bash
cd /mnt/d/codes/github/hipoint-airpress/airpress
gofmt -l plugin/ model/entity/ handler/admin/plugin.go config/   # 期望：无输出（有则 gofmt -w）
go vet ./plugin/... ./handler/... ./config/ ./dal/ ./model/...
go test ./plugin/... ./handler/... ./config/ ./dal/ -count=1
go build ./...
```
预期：vet 无告警；测试全绿；build 通过。若 sqlite 报 cgo 相关错误，确认本机 CGO_ENABLED=1
（与主程序同栈，此前 airpress.db 能跑即证明可用）。

- [ ] **Step 2: 回归护栏（现有测试不受影响）**

```bash
go test ./template/ -run "TestPrecompileBuiltinTemplates|TestRenderThemeTemplates" -count=1
```
预期：PASS（M1 不触碰模板层）。

- [ ] **Step 3: 手工冒烟（可选，推荐）**

启动 dev 实例（`go run . -config conf/config.dev.yaml`），用登录后的 JWT：

```bash
# 1) 生成一个 echo 插件 zip（用测试构造器）：go run ./scripts/plugindev 或临时 main
curl -H "Admin-Authorization: Bearer $TOKEN" http://localhost:8080/api/admin/plugins   # 期望 []
```
预期：空列表 200；登录/权限中间件正常。若嫌生成 zip 麻烦，跳过——单测已覆盖等价行为。

- [ ] **Step 4: 最终提交与状态标注**

```bash
git status --short   # 应只剩既有无关改动（主题等）；本 M1 全部已提交
```
本计划完成的定义（M1 验收，对应 spec §10）：
1. `POST upload`（zip：manifest 校验 + 试编译 + 安全解包）→ inactive 行；重名安装=升级换版本；
2. `enable`：host module 按权限构造、实例化（含 `_initialize` 兼容）、abi 握手、init 信封调用；
   越权 import → 启用失败且保持 inactive；
3. `disable`：shutdown 信封（尽力而为）+ 实例与 runtime 释放；
4. `uninstall`：目录/KV/设置 option/行 全清；active 拒绝；
5. `settings`：读写 option + settings_changed 通知；
6. 信封/内存协议/trap 超时重建/显式失败不重建，全部由夹具测试证明；
7. 应用退出 OnStop 广播 shutdown。

**M1 明确不做（留给 M2/M3/M4）**：事件桥接进 `event.Bus`（`Spec.Events` 校验存在但无派发）、
filter 链、`/api/plugins/*`、page-ticket、static 挂载、http.fetch/post.write/attachment.write
的 host 实现（guard 纯函数已就位待接）、SDK 仓库与 examples。

---

## 计划自查记录（writing-plans 完成后）

- **spec 覆盖**：spec §10 M1 行 = Task 1–12 全项；§2.4 状态机 → Task 10；§3.2 协议 → Task 5–8；
  §4.1 host 表（M1 子集）→ Task 7；§9 测试策略（echo/kvsink/malicious/zerohandle/hang + guard +
  manifest + store）→ Task 2/3/4/6/8/10/11；`spec.api/menu/timeouts/settings/fetch` 的
  校验（Task 2）先于其运行时用途（M3）。§4.2 的 3–7 条（拨号后校验/限额/UA/限流）在
  M3 接 `http_fetch` 时落地，M1 只交付其纯判定内核（guard）。
- **一致性**：`OptionKey/Granted/Build/NewInstance/Config/Deps/RC 码` 在各任务间同名；
  `manifest.OptionKey` 在 host（Task 7）与 lifecycle（Task 10）一致使用。
- **占位符**：Task 11 Step 1 有两处**声明式占位**（TestableService/pluginDtoOut/newZipWriter），
  已在 Step 4 给出确定修正（NewServiceForTest/*dto.PluginResp/archive/zip），其余步骤无 TBD。
- **环境风险**：sqlite 驱动=CGO（与主程序一致）；wazero v1.12 已缓存；不依赖 TinyGo/外网。
