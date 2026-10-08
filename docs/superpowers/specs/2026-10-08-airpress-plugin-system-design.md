# airpress 插件系统设计（WASM / wazero）

- 日期：2026-10-08
- 状态：待评审
- 目标仓库：`hipoint-airpress/airpress`

## 1. 需求与决策摘要

**需求**：WordPress 式的完整插件生命周期（后台上传 zip、启用/停用/卸载、声明式配置），
首个真实用例是「链接导入」插件：输入 URL → 服务端抓取第三方页面 → 解析正文与图片
URL → 图片转存为本站附件 → 生成 post。该用例要求插件能发起受限的网络访问。

**关键决策**（选型过程见附录 A）：

| 决策点 | 结论 |
|---|---|
| 运行时机理 | **WASM（wazero，纯 Go 无 cgo）**——唯一同时满足"跨语言 + 同进程钩子直调 + 能力沙箱"的方案；插件碰不到网络/文件，抓取必须走宿主注入的 `http_fetch`（权限声明 + SSRF 防线） |
| ABI | 自研极小 JSON ABI：单分发器 `handle` + 4 导出；不采用 Extism（核心依赖 Rust/cgo，Windows 减分，省下的层恰好是最快写完的层），不采用逐钩子类型化导出（跨语言成本最高） |
| 权限模型 | manifest 声明权限 → 启用时按权限**动态构造 host module**，越权 import 直接实例化失败（权限即接口，无检查旁路） |
| filter 时机 | 一期只挂**保存路径**（库内即最终内容，渲染零开销）；渲染期 filter 二期 |
| 后台 UI | 核心做全机制（API/页面/票据/静态资源）；Vue SPA 是编译产物不在本仓库，其两处小改动列为**外部依赖工作项**，不阻塞 |
| 插件作者语言 | **2026-10-08 二次修订（用户确认）**：**官方 Go ≥1.24 已支持 WASI Reactor 常驻插件**——`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared` 生成 `_initialize`（而非 `_start`）后保持活跃（reactor→`_initialize` 是 [WASI Preview 1](https://github.com/WebAssembly/WASI/blob/main/legacy/preview1/docs.md) 标准语义；`-buildmode=c-shared` 在 wasip1 上构建 reactor/library 见 [Go cmd/go 文档](https://go.dev/cmd/go/)）；`//go:wasmexport` 导出函数（参数/返回类型按 [Go 官方类型表](https://go.dev/cmd/compile/)：bool/int32/uint32→i32、int64/uint64→i64、float32→f32、float64→f64、unsafe.Pointer→i32、pointer→i32（受限）、string→(i32,i32) 仅作参数不能作返回；其余类型编译器拒绝；导出函数不导方法）；初始化放 `init()` 不放 `main()`（Reactor 不调 main）；`GOOS=js` 是独立 port 不适用于此（[Go 1.24 Release Notes](https://go.dev/doc/go1.24)：`go:wasmexport` 与 wasip1 reactor 均为 Go 1.24 引入，wasip1 本身 Go 1.21 引入）。Higress、阿里云 AI Gateway 等已用官方 1.24 编译迁移、不再依赖 TinyGo（第三方生产可用性声明，非官方文档佐证）。Go 1.28 实验性 `-buildmode=plugin`+WIT 绑定是后续演进。**结论：SDK 主目标 = 官方 Go（airpress 本机已是 1.27，直接可用）**；TinyGo 降为备选。M1 测试夹具仍用测试内纯 Go 微型构造器（零编译依赖，见 §9）；核心运行时需兼容 reactor 形态：实例化后若存在 `_initialize` 则调用一次（M1 Task 8 已含） |

## 2. 插件包格式与生命周期

### 2.1 包结构

```
linkscraper-1.0.0.zip
├─ plugin.yaml      # manifest（必需）
├─ main.wasm        # 唯一入口，文件名由 spec.entry 指定
└─ static/          # 可选：插件页面的 css/js 等静态资源
```

### 2.2 manifest（`plugin.yaml`）

```yaml
apiVersion: airpress/v1alpha1
kind: Plugin                    # 硬校验必须等于 Plugin（配合 apiVersion 拦截"传错 YAML"）
metadata:
  name: linkscraper             # [a-z][a-z0-9-]*，全局唯一
  title: 链接导入
  version: 1.0.0
  author: ryan
  description: 输入 URL 抓取内容生成文章
spec:
  entry: main.wasm
  permissions:                  # 枚举校验；取值见 §4.1
    [kv, log, option.self, http.fetch, post.read, post.write, attachment.write]
  events:  [post.updated, comment.new]      # 订阅的核心事件（action，异步）
  filters: [post.content.saved]             # 参与的 filter 链（同步）
  fetch:
    allowedHosts: ["*"]                      # 域名白名单，支持 *.example.com 通配
    allowHTTP: false                         # 默认仅 https
  menu:
    - { title: 链接导入, path: admin/scrape, icon: link }
  api:                          # METHOD+path 白名单（页面路由与 API 路由共用，未声明=核心 404）
    - "GET admin/scrape"        # 页面（OnPage）
    - "POST admin/scrape"       # 表单提交（OnAPI）
    - "GET admin/history"
  timeouts:                     # 可选覆盖 §3.4 默认值；上限 api/event/filter=120s、shutdown=10s
    api: 60s                    # 抓取类插件的页面+多图耗时远超默认 30s，示例给出放宽
  settings:                     # 字段规范逐字对齐主题 settings.yaml（复用 schema→表单机制）
    - name: default_status
      type: select
      options:
        - { label: 草稿, value: draft }
        - { label: 发布, value: publish }
      default: draft
    - { name: max_images, type: number, default: 10 }
```

### 2.3 数据表

沿用现有双轨约定：建表 SQL 追加到 `scripts/table.sql`（MySQL DDL，供生产部署），实体注册进 `dal/dal.go` 的 `AutoMigrate`（供开发/测试）。
插件 `plugin`/`plugin_kv` 两表**手写实体**（`model/entity/plugin.go`、`pluginkv.go`，非 `.gen.go`），因仓储层用原生 gorm API、不依赖 dal gen 层。

- `plugin`：`id`(自增，**即 filter 链排序键**)、`name`(唯一)、`title`、`version`、
  `status`(`inactive|active`)、`install_path`、`sha256`、`manifest_json`、`installed_at`、`updated_at`
- `plugin_kv`：`id`、`plugin_id`、`key`、`value`(TEXT)、`UNIQUE(plugin_id, key)`

插件设置值不新建机制：写现有 `option` 表，键 `plugin.{name}.settings`（JSON 快照），
自动复用 OptionUpdateEvent。manifest 里未声明 `spec.settings` 的插件无此键。

### 2.4 安装目录与生命周期 API

安装根：`config` 新增 `plugin_dir`（默认 `plugins/`），布局 `plugins/<name>/<version>/`。

管理端点（全部在 `AuthMiddleware` 之后，前缀 `/api/admin`）：

| 端点 | 行为 |
|---|---|
| `POST /plugins/upload` | multipart zip：解包→manifest 校验→wasm 试编译→写表（`inactive`）。重名同版本拒绝（v1 不支持覆盖安装为同版本） |
| `GET /plugins` | 列表（含状态、已声明钩子摘要） |
| `POST /plugins/:id/enable` | 实例化（host module 按权限构造）→ 调 `init` 信封；失败则回滚为 inactive 并报错 |
| `POST /plugins/:id/disable` | 取消在途调用，宽限 2s 后销毁实例 |
| `DELETE /plugins/:id` | 卸载（要求当前 inactive）：删 `plugin_kv`、`option` 键、磁盘目录、表行 |
| `GET/PUT /plugins/:id/settings` | schema 校验读写；PUT 成功后广播 `settings_changed` |

状态机仅两态 `inactive ⇄ active`。升级 = 上传同名更高版本、覆盖安装目录；
若目标 active，先 disable 再 enable（实例热切换）。**v1 不做旧版本目录保留/回滚**（已知取舍）。

## 3. 运行时与 ABI

### 3.1 运行时

- **每插件一个独立 `wazero.Runtime`**（非全局单例）：host module 名固定 `airpress`，而 wazero 同一 runtime 内 host module 名唯一，多插件必须各自成域；每插件编译一次并缓存 `CompiledModule`。
- 每插件**一个长驻实例 + 每插件 Mutex**：调用串行化（wasm 本就不能并行，并发上限=1 即语义）。
- **插件是 reactor 形态**：官方 Go 用 `GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`（Go ≥1.24，`//go:wasmexport`），产出导出 `_initialize` 而非 `_start`。核心实例化时
  `WithStartFunctions()`（不自动跑任何启动函数），随后若存在导出 `_initialize` 则显式调用一次（失败=启用失败）。
  TinyGo(`-target=wasm-unknown`)插件无 `_initialize`，同样成立。测试夹具（纯 Go 构造）两者皆无。
- 实例 trap/panic → 本次调用返回错误，实例**销毁重建**（下次调用前惰性重建），防脏状态。
- `MemoryLimitPages` 限内存：默认 **2048 页（128MB）**。依据：`http_fetch` 单响应上限 10MB，
  插件侧解析 HTML 还要在 wasm 堆里建 DOM/中间结构，32MB 会贴着上限跑；128MB 对博客负载宽裕。
  manifest 不提供扩大入口（出现真实需求再开）。

### 3.2 导出（插件必须实现的 4 项）

```
alloc(len u32) -> ptr u32     # 宿主写请求前调用，在插件堆中预留
free(ptr u32)                 # 宿主读完响应后释放
handle(ptr u32, len u32) -> res_ptr u32
    # res_ptr 指向 [len: u32 LE][bytes]，宿主按此读取线性内存；
    # res_ptr=0 视为插件显式失败（等价 {ok:false}），与 trap 同等处理但不触发实例重建
abi_version() -> u32          # 握手常量（当前 1），不匹配拒绝启用
```

跨边界字符串一律 UTF-8 JSON。内存借用协议：宿主 `alloc` → 写入 → `handle` → 读结果 → `free`。

### 3.3 信封

请求：

```jsonc
{ "id": "<uuid>", "kind": "init|shutdown|settings_changed|event|filter|api",
  "hook": "<钩子名>", "config": { /* 设置快照 */ }, "payload": { } }
```

响应：

```jsonc
{ "ok": true,  "data": <按 kind 定义> }
{ "ok": false, "error": "<人类可读>" }
```

`data` 约定：`filter` = 改写后的值；`api` = `{status, headers, body_b64}`——
请求与响应的 HTTP body **一律 base64**（二进制安全，无第二种编码路径）；其余 kind 宿主忽略 `data`。

### 3.4 调用语义与超时

| kind | 同步性 | 超时 | 失败处理 |
|---|---|---|---|
| `event` | 异步（每插件 buffered chan=256） | 5s/次 | 队列满或超时→丢弃+日志，不回压请求线程 |
| `filter` | 同步（保存路径） | 2s/插件、整链 5s | **fail-open**：跳过该插件，值取上一状态 |
| `api` | 同步（HTTP 请求内） | 30s | 返回 502 JSON 错误 |
| `init` | 同步（enable/重建时） | 5s | enable 失败；重建失败→插件标记 inactive |
| `shutdown` | 同步（disable/应用退出 fx.OnStop） | 3s | 超时直接销毁 |

默认值进 `config`；manifest `spec.timeouts` 可按 kind 覆盖（上限：api/event/filter 120s、
shutdown 10s）。**已知代价**：`api` 是同步 HTTP——重 IO 插件（如 linkscraper 抓一页+多图）
须在 manifest 放宽超时，且浏览器请求会挂到实际完成；后台异步任务框架（提交→轮询）记为二期候选。

## 4. 宿主能力与权限

### 4.1 host 函数清单

模块名 `airpress`。启用时按 `spec.permissions` **只把被授权的函数**挂进 host module，
再实例化插件——插件 import 了未授权函数则实例化直接失败，权限无法绕过。

| 函数 | 所需权限 | 说明 |
|---|---|---|
| `kv_set(key,val)` `kv_get(key)->val` `kv_del(key)` `kv_list(prefix)->keys[]` | `kv` | plugin_id 由宿主按调用方硬绑定，天然命名空间隔离 |
| `log(level, msg)` | 无需授权 | 进 zap，带 `plugin/<name>` 字段 |
| `now() -> ms` `random(buf,len)` | 无需授权 | |
| `option_get() -> json` `option_set(json)` | `option.self` | 仅能读写自己的 `plugin.{name}.settings` 一个键 |
| `post_get(id) -> dto` | `post.read` | 稳定 DTO（id/title/slug/status/content/创建时间…），非实体 dump |
| `post_create(dto) -> {id}` | `post.write` | 映射 `PostService.Create(*param.Post)`；v1 只写不改不删 |
| `attachment_create(name, mime, bytes) -> {id, url}` | `attachment.write` | 走现有 storage 抽象（本地/OSS/MinIO 插件无感）；大小限沿用现有上传限制；不要求 multipart |
| `http_fetch(req) -> resp` | `http.fetch` | 见 §4.2 |

host 函数失败一律返回**非零错误码**（写类：`rcBadArg/rcInternal` 等 uint32）或 **0 ptr**（读类：未命中/内部错误对插件都表现为"空"，返回 `0` 而非 frame），绝不跨边界 panic/trap；host 函数自身全部短超时。
**host 函数是本系统真正的攻击面**：全部入参（key 长度、URL、body 大小、DTO 字段）必须校验，信封解析纳入 fuzz 目标。

### 4.2 `http_fetch` SSRF 防线（安全核心）

抓取插件 = 服务端访问任意 URL，是经典 SSRF 面（云 metadata `169.254.169.254` 为首要靶点）。七层防线：

1. **域名白名单**：URL host 匹配 manifest `spec.fetch.allowedHosts`（`*.example.com` 通配）；插件运行时不可扩大。
2. **scheme**：默认仅 `https`；manifest `allowHTTP: true` 才放开 http（私网校验不豁免）。
3. **拨号后校验**：自定义 `Transport.DialContext`，连接建立后检查 **peer IP**，拒绝回环/私网(`10/8,172.16/12,192.168/16`)/链路本地(`169.254/16`)/运营商 NAT(`100.64/10`)/IPv6 对应段。校验发生在已连接 socket 上，**DNS rebinding 无窗口**。
4. **重定向每跳重复 1-3**，≤5 跳。
5. **限额**：响应 ≤10MB、请求体 ≤5MB、单请求 15s。
6. UA 固定 `AirPressPluginBot/<coreVersion>`，不透传用户 Cookie。
7. **每插件限流 30 req/min**（常量，v1 不可配）。

响应仅回传 `{status, headers(白名单: content-type/content-length 等), body}`；无 CookieJar（每调用独立）。

## 5. 钩子与 filter 链

### 5.1 事件桥接

插件启用时按 `spec.events` 往现有 `event.Bus` 注册包装 listener：Go 事件 → **稳定 DTO**
（每事件一个显式小结构，禁止 `entity` JSON 直出，防内部字段变成对外契约）→ 推入插件队列。

事件目录（snake_case 为对外名）：

| 插件事件名 | 核心事件 | payload(DTO 字段) |
|---|---|---|
| `startup` | StartEvent | `{}` |
| `post.created` / `post.updated` | **新增** PostCreatedEvent / PostUpdateEvent | `{post_id}` |
| `comment.new` / `comment.reply` | CommentNew/CommentReplyEvent | `{comment_id, post_id, parent_id, status}` |
| `user.updated` | UserUpdateEvent | `{user_id}` |
| `option.updated` | OptionUpdateEvent | `{}` |
| `theme.activated` / `theme.updated` / `theme.file.updated` | 对应事件 | `{}` |
| `attachment.uploaded` | **新增** | `{attachment_id, name, type, url}` |

核心改动：`PostService.Create` 补发 `PostCreatedEvent`（与 Update 分离）；
`AttachmentService.Upload` 成功后补发事件。各约 5 行。

### 5.2 filter 链

v1 仅一个点：`post.content.saved`，插在 `PostService.Create/Update` 校验后、写库前。

- 顺序：`plugin` 表 `id` 升序（确定、可复现，不引入 priority 字段）。
- 调用：`{kind:"filter", hook:"post.content.saved", payload:{value: html, ctx:{title, slug, is_new}}}`；
  返回 `data` 替换 value 传链。
- fail-open（§3.4）；注册表带原子计数，无订阅时开销≈0。
- 二期候选：`post.content.rendered`、摘要/标题类 filter——明确不在本期。

## 6. 后台集成

原则：**核心做全机制，SPA 只做最小补充**（Vue 包为编译产物、不在本仓库）。

1. **插件 API 路由**：`/api/plugins/{name}/{path...}`（AuthMiddleware 后）→ `kind:"api"` 信封
   （`payload:{method, path, query, headers, body_b64}`）→ 插件返回 `{status,headers,body}` 原样写回。
   manifest 的 `spec.api`（默认空）声明 `METHOD path` 白名单，未声明的路径核心直接 404。
2. **管理页面**：`/plugins/{name}/admin/{path}`（非 `/api` 前缀，浏览器直接导航）→
   同一 `api` 信封（`payload.ctx.mode=page`），插件返回 HTML。**页面路由同样受
   `spec.api` 白名单约束**（如 `GET admin/scrape`）。
   **认证用短效 ticket**：SPA 调 `POST /api/admin/plugins/:id/page-ticket`（带 header JWT）→
   返回 302 目标 `/plugins/{name}/admin/{path}?ticket=...`（单次使用、60s、存内存 LRU）。
   页面 JS 需调 API 时同源读 localStorage 的 JWT，fetch 带 header——不引入 cookie 体系改造。
3. **静态资源**：`/plugins/{name}/static/...` 直读安装目录，无认证（等同 WP）。
4. **设置表单**：`GET/PUT settings` 已覆盖；核心附一个通用服务端渲染表单页
   （复用 schema 规范）作兜底，不依赖 SPA 改动。

> **外部依赖工作项（SPA 仓库，另行排期，不阻塞 M1–M4）**：
> ① 侧栏数据化"插件"菜单区（消费 `GET /api/admin/plugins/menus`，即各插件 `spec.menu` 聚合）；
> ② 插件管理页（列表/上传/启停/设置，消费 §2.4 API）。

## 7. 参考插件「链接导入」（端到端验收件）

- manifest：`permissions [kv, log, http.fetch, post.write, post.read, attachment.write]`、
  `menu [{title:链接导入, path:admin/scrape}]`、`allowedHosts ["*"]`、
  `settings [default_status, max_images]`、`api ["GET admin/scrape"(页面), "POST admin/scrape", "GET admin/history"]`、
  `timeouts {api: 60s}`。
- 数据流（在 `admin/scrape` 的 api handler 内）：

```
表单 URL → kv_get("seen:"+sha256(url)) 查重
        → http_fetch(url)                         # 宿主代发，七层防线
        → wasm 内解析 HTML（golang.org/x/net/html，纯 Go，wasip1 可用）
        → 提取 title / 正文容器 / 图片 URL 候选（og:image、正文 img src）
        → 逐个 http_fetch(图片)（≤max_images，content-type 为 image/*）
        → attachment_create(name, mime, bytes) → {id, url}
        → 重写正文 img src 为本地 url
        → post_create({title, content, status: default_status})
        → kv_set("seen:"+sha256(url), postID)
        ← JSON {post_id, url, images: n}；页面展示结果
```

- `admin/history`（GET）：`kv_list("seen:")` + `post_get` 补标题——同时验收读类 API。
- 该插件同时是 SDK 的 dogfood 与开发文档的样板。

## 8. SDK 与开发文档

- 独立仓库 `airpress-plugin-go`（官方 Go wasip1 reactor SDK，`-buildmode=c-shared`；见 §1 决策表二次修订）：封装 `alloc/free/handle` 生成、
  信封编组、host 函数绑定；作者代码形如：

```go
func main() {
    p := plugin.New()
    p.OnAPI("POST admin/scrape", handleScrape)
    p.OnPage("admin/scrape", renderForm)
    p.OnEvent("post.created", onPost)
    p.OnFilter("post.content.saved", rewriteContent)
    p.Run()
}
```

- 本仓库 `docs/plugin-dev-guide.md`：打包格式、manifest 字段表、权限语义、构建命令
  （`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm`）、SSRF 约束说明、linkscraper 走读。
- `examples/linkscraper/`：参考插件源码，独立 `go.mod`（与主模块隔离），产物不入库。

## 9. 测试策略

**单测（纯 Go）**
- `manifest`：错误 `kind`、缺 `entry`、非法 name/version、未知权限名、`spec.timeouts` 超上限 → 拒绝。
- `hooks`：事件→DTO 快照；队列满丢弃计数；无订阅快速路径。
- `hooks.ApplyFilter`：链式、id 升序、超时跳过、坏返回 fail-open（表驱动）。
- `guard`：IP 段判定表驱动；通配域匹配；重定向策略；httptest 起 302→`127.0.0.1` 的假服务断言拒绝。
- `host`：权限→host module 构造表；信封 fuzz。

**集成测试（wasm fixture 由测试内纯 Go 构造器生成，零外部工具链；**2026-10-08 修订**，替代原"预编译提交"方案）**
- `echo`：协议往返（alloc/handle/free/abi 握手）。
- `malicious`：import 未授权函数 → 实例化失败。
- `kvsink`/`optsink`：真实调用 host 函数（kv_set/kv_get/kv_del/kv_list/option_set/log），断言宿主侧副作用。
- `zerohandle`：handle 返回 0（显式失败语义）。
- `hang`：handle 死循环 → 超时关闭与实例重建（event 丢弃 / filter 跳过 / api 502 在 M2/M3 复用）。
- runtime：trap 后重建、Mutex 串行、MemoryLimitPages 触发。

**端到端**：linkscraper 手工验收清单（httptest 假目标页→导入→文章/附件/防重检查）。

**目标**：`plugin/runtime`、`plugin/hooks`、`plugin/guard` 覆盖率 ≥80%。
回归：SSRF 清单（metadata、rebinding、超限、重定向循环）每次发版执行；
现有模板护栏 `go test ./template/` 不受影响（一期不触碰模板层）。

## 10. 里程碑与代码落位

| 里程碑 | 交付 | 验收 |
|---|---|---|
| M1 | `plugin/manifest store runtime host guard service` + 两张表 + 生命周期 API + ABI + host: kv/log/now/random/option.self + echo/malicious fixture | 单测 + 启停状态机 |
| M2 | 事件桥接 + DTO + `post.content.saved` + 核心补发 `post.created`/`attachment.uploaded` + slow fixture | hooks/guard 单测 |
| M3 | `/api/plugins/*` + page-ticket + static + settings 读写 + 通用表单页 + `attachment_create/post_create/post_get/http_fetch` | ticket 流程 E2E + SSRF 集成测试 |
| M4 | SDK 仓库 + `examples/linkscraper` + 开发文档 | §7 手工验收清单 |

```
plugin/
├─ manifest/  store/  runtime/  host/  guard/  hooks/  service/
handler/admin/plugin.go      # /api/admin/plugins/*
handler/pluginserve.go       # /api/plugins/*、/plugins/{name}/admin|static、ticket
service/post.go 等           # ApplyFilter 插入 + 补发事件（各 ~5 行）
main.go / injection          # fx 接线；OnStop 广播 shutdown
scripts/table.sql            # plugin、plugin_kv；实体走 gen
conf / config                # plugin_dir、插件系统总开关、超时/内存默认值
docs/plugin-dev-guide.md     examples/linkscraper/
```

## 11. 已知取舍与不做清单

- 官方 Go ≥1.24 的 wasip1 reactor（`-buildmode=c-shared`+`//go:wasmexport`，用户确认已生产可用）为 SDK 主目标；限制：导出参数/返回仅 wasm 标量、初始化在 `init()`、`main()` 不被调用。纯 JS-as-plugin 不支持（如需要，二期评估 QuickJS-as-wasm 套娃，明确不做）。
- 单实例串行：重计算插件不得放到同步路径（filter/api 超时即 fail/skip 的保护在此）。
- wasm 沙箱强，但 host 函数是攻击面 → 入参校验 + fuzz 是 M1–M3 的完成定义一部分。
- v1 不做：升级回滚目录、签名校验、依赖关系、多插件实例并行、渲染期 filter、前台路由、
  模板注入、`post.update/delete` 等细粒度权限、per-plugin 资源配置、i18n。
- 评论链路（halo-comment）插件化涉及前台，明确排除在二期之外（二期候选仅列模板注入机制本身）。

## 附录 A：选型对比记录

| | WP 本体（PHP） | **A. WASM(wazero)** | B. 独立进程+RPC | C. 脚本(Lua/JS) |
|---|---|---|---|---|
| 语言 | 仅 PHP | 多语言→wasm | 多语言 | 仅脚本 |
| 钩子 | 进程内直调 | 进程内（沙箱） | 跨进程开销大 | 进程内 |
| 安装包 | zip 文本 | zip=yaml+wasm 平台无关 | **按平台编译，分发痛** | zip 文本 |
| 隔离 | 无 | 强（能力式，无 ABI 则碰不到网络） | 最强 | API 收口 |
| Windows | — | ✓ | ✓ | ✓ |
| 成本 | — | 高 | 中 | 低-中 |

排除项：Go 原生 `plugin` 包（不支持 Windows + 依赖树版本锁死，第三方分发不可行）；
Extism（生命周期/钩子/后台/SSRF 策略仍须自写，其省下的一层是本项目最快写完的一层，
且现版本核心为 Rust/cgo，Windows 部署减分）。
用户决策轨迹：完整生命周期 → 类 WordPress 且跨语言 → 三约束唯一交集 WASM → 同意自研 ABI。
