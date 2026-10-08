# theme-argon 资源管线全面 vite 化

> **路径更新（2026-09-30，阶段 E 之后）**：本文是 2026-09-29 的设计快照，文件位置此后有变化：
> `src/argontheme.js` → `src/scripts/main.js`、`src/style.scss` → `src/styles/main.scss`（正文已按新位置更新）；
> 另 `src/vendor/**` 已在阶段 C 删除（第三方库改由 CDN 提供），正文里该路径只作历史记录。
> 未带 `src/` 前缀的 `style.css`/`argontheme.js` 指当时位于主题根目录的构建产物。

- 日期：2026-09-29
- 范围：`airpress/resources/template/theme/theme-argon`
- 目标：用 vite 完整接管 CSS+JS 资源管线，删除手工拼接的 `argon_css_merged.css` / `argon_js_merged.js`，将第三方库与项目自有代码分离、分别产出固定文件名并可压缩。

## 1. 背景与现状

theme-argon 是从 WordPress 的 argon 主题移植到 Halo 的主题。当前资源管线处于「半迁移」状态：

| 资源 | 大小 | 来源 | 由谁构建 | 性质 |
|------|------|------|----------|------|
| `style.css`（根目录） | 156KB | `src/styles/main.scss` | sass（`--style=expanded`，**未压缩**） | 项目自有 CSS |
| `argontheme.js`（根目录） | 87KB | `src/scripts/main.js` | vite（`minify:false`，lib 模式，用全局 `$`） | 项目自有 JS 逻辑 |
| `assets/js/argon.min.js` | 3.2KB | 手写静态 | 无 | 项目头部小脚本 |
| `assets/argon_css_merged.css` | 358KB | **手工拼接** | 无 | 第三方 CSS（bootstrap、font-awesome…） |
| `assets/argon_js_merged.js` | 584KB | **手工拼接** | 无 | 第三方 JS（jquery、bootstrap、fancybox、highlight…共 20+ 库） |
| `assets/vendor/font-awesome/fonts` + `highlight/styles` | — | 静态 | 无 | 字体 + 动态高亮主题 |

问题：

1. `package.json` 声明了第三方依赖，但构建流程**根本没有打包它们**——只是摆设；merged 文件是 WordPress 时代手工拼接的遗留物。
2. merged 文件**不可维护、无法压缩、无法 tree-shaking、版本与 npm 脱节**。
3. SCSS 用 `--style=expanded` 编译，**未压缩**，产物体积偏大。
4. 既有 `vite.config.js` 是 lib 单入口模式，仅产出 `argontheme.js` 到根目录，不覆盖 CSS 与第三方库。

## 2. 结论：merged 文件「内容必要、形式多余」

`argon_css_merged.css` / `argon_js_merged.js` 承载的第三方库功能必须保留，但它们作为**手工拼接产物**应被 vite 从 npm + 本地 vendor 打包出的等价产物取代。审计后确认：主题实际用到的库是 merged 里的子集，存在未使用代码。

## 3. 目标产物结构（固定文件名，Halo 友好）

Halo 模板以 `theme_base/assets/...?version={{ theme.Version }}` 的固定相对路径 + 版本号引用资源。vite 默认输出带 hash 的文件名，与这种引用不兼容，因此配置 vite **输出固定文件名、不带 hash**，模板引用基本不动。

| 产物（输出到 `assets/`） | 内容 | 取代 |
|------|------|------|
| `assets/vendor.js` | 第三方库，全局暴露 `$`/`jQuery`/`Headroom`/`iziToast`/`tippy`/`noUiSlider`/`hljs` 等 | `assets/argon_js_merged.js` |
| `assets/vendor.css` | 第三方 CSS（bootstrap、font-awesome 等） | `assets/argon_css_merged.css`（部分） |
| `assets/app.js` | 项目自有 JS（`src/scripts/main.js` + `assets/js/argon.min.js`） | 根目录 `argontheme.js` |
| `assets/app.css` | 项目自有 CSS（`src/styles/main.scss`，**压缩**） | 根目录 `style.css` |

配置要点：

- `build.outDir: 'assets'`，`build.emptyOutDir: false`（避免冲掉 `assets/vendor/font-awesome/fonts` / `highlight/styles` 等仍需保留的静态目录，以及图片等）。
- `rollupOptions.output.entryFileNames: '[name].js'`、`chunkFileNames: '[name].js'`、`assetFileNames: '[name][extname]'`——强制固定名、无 hash。
- `build.minify: 'esbuild'`（JS 压缩，取代当前 `minify:false`）。
- `build.cssMinify: true`（**CSS 压缩**，取代 sass `--style=expanded`）。

## 4. 第三方库审计与处置（审计后精简）

对 merged 中的约 20 个库逐一核对调用点（全局变量 / jQuery 插件 / CSS 类名）。保留在用、删除未用。

**关键发现（修订取源策略）：** 第三方库的独立源文件**不存在**——它们仅以拼接形式存在于 `assets/argon_js_merged.js` / `assets/argon_css_merged.css` 内，且二者**含分库标记**（JS 如 `/* assets/vendor/jquery/jquery.min.js */`，CSS 如 `/* assets/css/argon.min.css */`）。因此取源策略改为：**从 merged 按标记精确拆分**为 `src/vendor/` 下独立文件，再由构建管线打包，保留原版本、零行为风险。`package.json` 中版本不匹配（见 4.1）的 npm 依赖在重构后清理。

审计发现三类特殊情形：

### 4.1 版本陷阱（不能无脑 npm 升级）

- **jquery**：merged 实际是 **3.2.1**，而 `package.json` 写的是 `^4.0.0`。v4 与依赖 v3 的旧代码不兼容 → 锁定到 3.2.1（npm 安装 `jquery@3.2.1` 或保留本地副本）。
- **font-awesome**：merged 用的是 **FA4**（`fa fa-xxx` 类名 + `fontawesome-webfont` 字体）。npm 的 `@fortawesome/fontawesome-free` 是 FA6，类名体系不同，升级会**破坏图标** → 保持 FA4，由 vite 接管 CSS 并将字体 emit 到固定路径，删除 `assets/vendor/font-awesome/fonts` 静态副本（改由构建产物提供）。

### 4.2 本地专有 / 非 npm 库

`zoomify`、`sharejs`、`jquery-pjax-plus`、`jquery.easing`、`lazyload`、`pangu`、`pickr`、`headindex` 是 WordPress 时代本地副本，不在 npm 或无从升级 → 抽到 `src/vendor/` 作为本地源，由 vite 一并打包进 `vendor.js`（CSS 部分若有的也进 `vendor.css`）。

### 4.3 动态高亮主题

`assets/vendor/highlight/styles/{{code_theme}}.css` 由设置 `settings.code_theme` 在运行时动态选择，**无法**静态打包进 `vendor.css` → **保留该静态目录**，并把 highlight 相关样式从 `vendor.css` 剥离（仅把 `hljs` JS 纳入 `vendor.js`）。

### 4.4 审计结论（实现阶段逐库最终确认）

| 库 | merged 内形态 | 处置 |
|------|------|------|
| jquery 3.2.1 | 本地 | 锁定 3.2.1，进 vendor（已确认源码大量使用 `$`/`jQuery`） |
| bootstrap (CSS+JS) | 本地 | 进 vendor（CSS 已确认引用，JS 与 `data-` 属性交互） |
| font-awesome 4 | 本地 | 进 vendor（FA4，保字体；已确认 `fa fa-xxx` 使用） |
| popper.js | 本地 | 进 vendor（bootstrap 依赖） |
| headroom.js | 本地 | 进 vendor（已确认 `new Headroom`） |
| iziToast | 本地/npm | 进 vendor（已确认 `iziToast.show`） |
| tippy.js | 本地/npm | 进 vendor（已确认 `tippy()`） |
| noUiSlider | 本地/npm | 进 vendor（已确认 `noUiSlider.create`） |
| fancybox | 本地/npm | 进 vendor（已确认 `.fancybox`） |
| pangu | 本地/npm | 进 vendor（已确认 pangu 处理） |
| pjax (`jquery-pjax-plus`) | 本地 | 进 vendor（已确认 `$.pjax`，6 处引用） |
| zoomify | 本地 | 进 vendor（已确认 zoomify 调用） |
| NProgress | 本地/npm | 进 vendor（已确认 pjax 进度条） |
| clamp-js | 本地/npm | 进 vendor（已确认 clamp 调用） |
| hljs + line-numbers | 本地 | JS 进 vendor；**样式剥离**（动态主题保留静态目录） |
| headindex | 本地 | 进 vendor（已确认调用） |
| clipboard.js | 本地 | 进 vendor（已确认 `new ClipboardJS(...)`，`src/scripts/main.js:2693`） |
| pickr | 本地 | 进 vendor（已确认 `new Pickr(...)`，`src/scripts/main.js:2483`） |
| jquery.easing | 本地 | 进 vendor（已确认 `.animate(..., 'easeOutExpo'/'easeOutCirc')`，jQuery 核心无此命名缓动） |
| **sharejs** | 本地 | **删除**（全库零调用：`grep` 命中 0） |
| Argon DS JS（`assets/js/argon.min.js`） | 独立文件 | 进 vendor（Argon Design System，Creative Tim；依赖 `$`/`Headroom`/`noUiSlider`） |

> `navigator.clipboard` 是浏览器原生 API，与 `clipboard.js` 库无关，二者并存。
> 审计已完成（本表即最终结论）；实现阶段再对每个「保留」项与「删除」项做一次 grep 复核作为验证步骤。

## 5. 构建机制（关键约束：经典脚本不能走 ESM 打包）

**约束：** jQuery/Bootstrap/popper 等是 UMD 全局脚本；`src/scripts/main.js` 顶部用 `var argonConfig`（依赖全局作用域），且大量使用全局 `$`/`jQuery`。若交给 Vite 以 ESM 打包：UMD 库会走 CommonJS 分支、不再挂到 `window`；argontheme 的 `var argonConfig` 会变成模块作用域、与 `head.tmpl` 注入的全局 `window.argonConfig` 脱节，导致主题配置全部失效。因此**经典脚本必须「拼接顺序 + 压缩」，不能 ESM bundle**。

机制（已确认）：

- **经典脚本（vendor.js / app.js）**：由 `scripts/build-vendor.mjs` 读取 `src/vendor/` 下按固定顺序拆出的源文件，**顺序拼接**后用 esbuild 压缩（`transform(code, { minify: true, loader: 'js' }`）输出固定名文件。esbuild 是 Vite 的内置引擎，此处直接用于保留全局作用域语义。顺序沿用 merged 原顺序（jquery → popper → bootstrap → …，去重，剔除 sharejs），Argon DS 的 `argon.min.js` 置于末尾。
- **第三方 CSS（vendor.css）**：同源由 `build-vendor.mjs` 拼接 5 个 CSS 片段（argon.min.css → font-awesome → iziToast → pickr → fancybox）后 esbuild 压缩，输出 `assets/vendor.css`。字体 `url('vendor/font-awesome/fonts/...')` 在 merged 中**已是相对 `assets/` 的路径**，落到 `assets/vendor.css` 后无需改写，字体目录 `assets/vendor/font-awesome/fonts/` 仍作静态资源保留。
- **主题自有 CSS（app.css）**：由 Vite 经 sass 编译 `src/styles/main.scss` 并压缩（体现「Vite 管理主题 scss」），输出固定名 `assets/app.css`。

`vite.config.js` 改造：

- `build.lib` 入口服从 `src/entries/style.js`（`import '../styles/main.scss'`），`formats: ['iife']`、`fileName: 'app'`、`outDir: 'assets'`、`emptyOutDir: false`、启用 CSS 压缩 → 产出 `assets/app.js`（空壳，将被 `build-vendor.mjs` 覆盖）+ `assets/app.css`。
- `build.rollupOptions.output.assetFileNames: '[name][extname]'` 强制固定名、无 hash。
- 删除原 `lib`/`copy-to-root` 逻辑（产物直接落在 `assets/`）。
- `package.json` 脚本改为 `"build": "vite build && node scripts/build-vendor.mjs"`、`"watch": "vite build --watch"`，移除 `build:js`/`build:css` 拆分（sass 由 Vite 接管）。
- 清理 `package.json` 中未被取源使用的 npm 依赖（重构后以实际拆分来源为准核对）。

## 6. 模板改动

- `module/head.tmpl`：
  - 第 19 行 `assets/argon_css_merged.css` → `assets/vendor.css`
  - 第 20 行 `style.css` → `assets/app.css`
  - 第 37 行 `assets/argon_js_merged.js` → `assets/vendor.js`
  - 顺序保证 `vendor.js` 在 `vendor.css`/`app.css` 之后、`app.js` 之前加载（全局库先就绪）。
- `module/scripts.tmpl`：
  - 第 2 行 `argontheme.js` → `assets/app.js`
  - 第 13 行动态高亮主题 `assets/vendor/highlight/styles/...` → **保留不变**。
- `head.tmpl` 第 38 行 `assets/js/argon.min.js`：已并入 `vendor.js`（作为 Argon DS 第三方脚本置于末尾），删除该静态文件与旧引用。

## 7. 删除项与 `.gitignore`

- **删除**：`assets/argon_css_merged.css`、`assets/argon_js_merged.js`、`assets/js/argon.min.js`（已并入 app.js）。
- `.gitignore` 当前忽略 `dist/`、`node_modules/` 等；新产物 `assets/vendor.*` / `assets/app.*` 需**提交**给 Halo 运行环境，确认 `.gitignore` 未排除 `assets/`（当前未排除，保留即可）。`dist/` 不再使用可保留忽略或移除（建议保留 `dist/` 忽略以兼容历史）。

## 8. 验证标准

1. `npm install` 后 `npm run build` 成功，产出 `assets/vendor.js`、`assets/vendor.css`、`assets/app.js`、`assets/app.css` 四个**固定名**文件。
2. `vendor.css` + `app.css` 总体积 < 原 `argon_css_merged.css`(358KB) + `style.css`(156KB)，且 CSS 为压缩单行（无多余空白/注释）。
3. 本地起 Halo 预览，首页与文章页：样式正常、夜间模式切换、pjax 翻页、fancybox 灯箱、代码高亮、iziToast 提示、tooltip 均正常。
4. 控制台无 404 静态资源、`jQuery is not defined`、`Headroom is not defined` 等报错。
5. 确认 `assets/vendor/font-awesome` 字体与 `assets/vendor/highlight/styles` 仍可访问（或已由新机制覆盖）。

## 9. 风险与回退

- **版本不兼容**：jquery 4 → 3.2.1、FA4 → FA6 是主要风险点，已通过对锁定版本与保留 FA4 规避。
- **全局变量缺失**：若 `argontheme.js` 依赖的某个全局未正确挂到 `window`，表现为运行期报错。通过 `vendor.js` 末尾集中挂载 + 验证标准第 4 条兜底。
- **回退**：保留本次改动前的 `argon_css_merged.css` / `argon_js_merged.js` 于 git 历史，出问题可 `git revert` 对应文件恢复旧引用。
