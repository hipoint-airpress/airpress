# theme-argon 资源管线 vite 化 实现计划

> **路径更新（2026-09-30，阶段 E 之后）**：本文是阶段 A/B 的计划快照，其中的文件位置此后有变化，读正文时按此对照：
> - `src/argontheme.js` → `src/scripts/main.js`、`src/style.scss` → `src/styles/main.scss`（正文已按新位置更新）；
> - `src/css/` → `src/styles/vendor/`（正文中形如 `src/css/...` 的写法保留为当时的位置）；
> - `src/vendor/**`、`scripts/split-merged.mjs`、`scripts/build-vendor.mjs` 已在阶段 C 删除（第三方库改由 CDN 提供），
>   正文里这些路径只作历史记录；
> - 未带 `src/` 前缀的 `style.css`/`argontheme.js` 指当时位于主题根目录的构建产物。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除手工拼接的 `assets/argon_css_merged.css` / `assets/argon_js_merged.js`，改为从 merged 按标记拆分出第三方源、由「拼接+压缩」管线产出固定名的 `vendor.js`/`vendor.css`，主题自有的 `style.scss`/`argontheme.js` 由 Vite/esbuild 产出 `app.css`/`app.js`，并压缩 CSS。

**Architecture:** 经典脚本（jQuery/Bootstrap 等 UMD 全局库 + 依赖全局 `argonConfig` 的 `argontheme.js`）**不能走 ESM 打包**，否则全局作用域被破坏；因此用「顺序拼接 + esbuild 压缩」保留原语义。主题自有的 `src/styles/main.scss` 走 Vite 的 sass 管线并压缩。产物全部落在 `assets/`，采用固定文件名（无 hash）以适配 Halo 模板的 `theme_base/assets/...?version=` 引用。

**Tech Stack:** Vite 8（+ sass 插件）、esbuild（Vite 内置引擎）、Node ESM 脚本、Halo 主题模板（`{{ }}`/`{% %}`/`{# #}`）。

**工作分支：** `refactor/vite-asset-pipeline`（已创建并含 spec 提交）。所有任务在此分支执行。

**参考 spec：** `docs/superpowers/specs/2026-09-29-theme-argon-asset-pipeline-design.md`

---

## 文件结构（本计划创建/修改/删除）

| 动作 | 路径 | 职责 |
|------|------|------|
| 新建 | `scripts/split-merged.mjs` | 一次性：按标记把 merged 拆成 `src/vendor/**` 独立源文件 |
| 新建 | `scripts/build-vendor.mjs` | 构建期：拼接+压缩 → `assets/vendor.js`/`vendor.css`/`app.js` |
| 新建 | `src/entries/style.js` | Vite 入口，仅 `import '../styles/main.scss'` → `assets/app.css` |
| 新建 | `src/vendor/**`（拆分产物） | 第三方库独立源文件 |
| 新建 | `src/vendor/argon-ds/argon.min.js` | 由 `assets/js/argon.min.js` 迁移而来（Argon DS） |
| 修改 | `vite.config.js` | 改为 lib 入口 `src/entries/style.js`，输出固定名到 `assets/` |
| 修改 | `package.json` | 脚本合并为 `build`；清理未用的 `dependencies`；加 `esbuild` devDep |
| 修改 | `module/head.tmpl` | CSS/JS 引用改为 `assets/vendor.*` / `assets/app.css` |
| 修改 | `module/scripts.tmpl` | `argontheme.js` → `assets/app.js` |
| 删除 | `assets/argon_css_merged.css`、`assets/argon_js_merged.js` | 使命终结 |
| 删除 | `assets/js/argon.min.js`、根 `style.css`、根 `argontheme.js` | 旧产物/已迁移 |
| 删除 | `src/vendor/headindex.js`（若确认冗余） | 与拆分出的 headindex 去重 |
| 保留 | `assets/vendor/font-awesome/fonts/**`、`assets/vendor/highlight/styles/**` | 运行时静态资源（字体 + 动态高亮主题） |

---

## Task 1: 编写并按标记拆分 merged 文件

**Files:**
- Create: `scripts/split-merged.mjs`
- Create（运行产物）: `src/vendor/**`、`src/css/argon.min.css`

- [ ] **Step 1: 写拆分脚本**

创建 `scripts/split-merged.mjs`：

```js
// 一次性脚本：按 merged 文件内的分库标记，拆分为 src/ 下的独立源文件。
// merged 结构：`/* assets/<原路径> */\n<该库压缩代码>\n/* assets/<原路径> */\n...`
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const MARKER = /\/\* (assets\/[^*]+?) \*\//g;

// 审计结论：sharejs 全库零调用，精简删除（见 spec §4.4）
const DROP = new Set(['assets/vendor/sharejs/sharejs.min.js']);

function split(srcRel) {
  const text = readFileSync(join(root, srcRel), 'utf8');
  const matches = [...text.matchAll(MARKER)];
  const written = [];
  for (let i = 0; i < matches.length; i++) {
    const m = matches[i];
    const start = m.index + m[0].length;
    const end = i + 1 < matches.length ? matches[i + 1].index : text.length;
    const body = text.slice(start, end).trim();
    const srcPath = m[1];
    if (DROP.has(srcPath)) { console.log('跳过(精简):', srcPath); continue; }
    if (!body) continue;
    const outPath = join(root, 'src', srcPath.replace(/^assets\//, '')); // assets/vendor/x → src/vendor/x
    mkdirSync(dirname(outPath), { recursive: true });
    writeFileSync(outPath, body + '\n');
    written.push(outPath.replace(root + '/', ''));
  }
  return written;
}

console.log('== JS 片段 ==');
console.log(split('assets/argon_js_merged.js').join('\n'));
console.log('== CSS 片段 ==');
console.log(split('assets/argon_css_merged.css').join('\n'));
```

- [ ] **Step 2: 运行拆分**

Run: `node scripts/split-merged.mjs`
Expected: 列出 20 个 JS 片段（`src/vendor/jquery/jquery.min.js` … `src/vendor/jquery.easing/jquery.easing.min.js`，**不含 sharejs**）与 5 个 CSS 片段（`src/css/argon.min.css`、`src/vendor/font-awesome/css/font-awesome-min.css`、`src/vendor/izitoast/css/iziToast.css`、`src/vendor/pickr/themes/monolith.min.css`、`src/vendor/fancybox/jquery.fancybox.min.css`），并打印 `跳过(精简): assets/vendor/sharejs/sharejs.min.js`。

- [ ] **Step 3: 校验拆分完整性（总量守恒）**

Run:
```bash
python3 - <<'PY'
import glob, os
tot = sum(os.path.getsize(f) for f in glob.glob('src/vendor/**/*.js', recursive=True))
print('src/vendor JS 总字节:', tot)
PY
```
Expected: 约 560KB~590KB 量级（与 `assets/argon_js_merged.js` 的 584932 字节同量级，差额仅 sharejs 片段）。若明显偏小说明拆分丢失内容。

- [ ] **Step 4: 提交**

```bash
git add scripts/split-merged.mjs src/vendor src/css
git commit -m "refactor: split merged vendor assets into src/vendor sources"
```

---

## Task 2: 审计复核（保留项 / 删除项）

**Files:**
- Test: 无（grep 校验）

- [ ] **Step 1: 复核「保留」的库确有调用**

Run:
```bash
grep -q "new ClipboardJS" src/scripts/main.js && echo "clipboard OK"
grep -q "new Pickr" src/scripts/main.js && echo "pickr OK"
grep -qE "easeOutExpo|easeOutCirc" src/scripts/main.js && echo "jquery.easing OK"
grep -q "noUiSlider" src/scripts/main.js && echo "nouislider OK"
grep -q "iziToast" src/scripts/main.js && echo "izitoast OK"
grep -q "\$.pjax" src/scripts/main.js && echo "pjax OK"
grep -q "Headroom" src/scripts/main.js && echo "headroom OK"
grep -q "pangu" src/scripts/main.js && echo "pangu OK"
grep -q "tippy(" src/scripts/main.js && echo "tippy OK"
```
Expected: 9 行 `... OK` 全部输出。

- [ ] **Step 2: 复核「删除」的 sharejs 确为零调用**

Run: `grep -rn "ShareJS\|sharejs" src/scripts/main.js module/ layouts/ *.tmpl | wc -l`
Expected: `0`

- [ ] **Step 3: 确认拆分源中不含 sharejs**

Run: `ls src/vendor/sharejs 2>/dev/null && echo "FAIL: sharejs 仍存在" || echo "OK: sharejs 已剔除"`
Expected: `OK: sharejs 已剔除`

---

## Task 3: 迁移 Argon DS 脚本并去重 headindex

**Files:**
- Create: `src/vendor/argon-ds/argon.min.js`（由 `assets/js/argon.min.js` 复制）
- Delete: `src/vendor/headindex.js`（若无引用）

- [ ] **Step 1: 迁移 Argon DS 脚本到 src/vendor**

Run:
```bash
mkdir -p src/vendor/argon-ds
git mv assets/js/argon.min.js src/vendor/argon-ds/argon.min.js
ls -l src/vendor/argon-ds/argon.min.js
```
Expected: 文件存在于 `src/vendor/argon-ds/argon.min.js`（约 3241 字节）。

- [ ] **Step 2: 检查旧 `src/vendor/headindex.js` 是否被引用**

Run: `grep -rn "vendor/headindex\|headindex" src/scripts/main.js | head`
Expected: 无引用（`src/scripts/main.js` 无 import）。若确无引用则执行 Step 3；若有引用，改为保留该文件并在 Task 4 的列表中加入它、同时从拆分列表移除 `headindex/headindex.js`。

- [ ] **Step 3: 删除冗余的 headindex 副本**

Run:
```bash
git rm src/vendor/headindex.js
```
Expected: 删除成功。（权威 headindex 来自 Task 1 拆出的 `src/vendor/headindex/headindex.js`。）

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "refactor: move argon DS script into src/vendor, dedupe headindex"
```

---

## Task 4: 编写拼接+压缩构建脚本

**Files:**
- Create: `scripts/build-vendor.mjs`

- [ ] **Step 1: 写构建脚本**

创建 `scripts/build-vendor.mjs`：

```js
// 构建脚本：把经典脚本（UMD 全局库 + 依赖全局 argonConfig 的 argontheme.js）
// 顺序拼接后用 esbuild 压缩，保留全局作用域语义（不能用 ESM bundle）。
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import esbuild from 'esbuild';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const at = (p) => join(root, p);
const read = (p) => readFileSync(at(p), 'utf8');

// 顺序沿用 merged 原顺序（popper 去重、sharejs 剔除）；Argon DS 置于末尾。
const VENDOR_JS = [
  'src/vendor/jquery/jquery.min.js',
  'src/vendor/bootstrap/bootstrap.min.js',
  'src/vendor/popper/popper.min.js',
  'src/vendor/headindex/headindex.js',
  'src/vendor/headroom/headroom.min.js',
  'src/vendor/nprogress/nprogress.js',
  'src/vendor/izitoast/js/iziToast.min.js',
  'src/vendor/lazyload/jquery.lazyload.min.js',
  'src/vendor/zoomify/zoomify.js',
  'src/vendor/pickr/pickr.es5.min.js',
  'src/vendor/nouislider/js/nouislider.min.js',
  'src/vendor/pangu/pangu.min.js',
  'src/vendor/clipboard/clipboard.min.js',
  'src/vendor/highlight/highlight.pack.js',
  'src/vendor/highlight/highlightjs-line-numbers.min.js',
  'src/vendor/jquery-pjax-plus/jquery.pjax.plus.js',
  'src/vendor/clamp/clamp.min.js',
  'src/vendor/fancybox/jquery.fancybox.min.js',
  'src/vendor/tippy.js/dist/tippy.umd.min.js',
  'src/vendor/jquery.easing/jquery.easing.min.js',
  'src/vendor/argon-ds/argon.min.js', // Argon DS，依赖上面的 $/Headroom/noUiSlider
];

// 顺序沿用 merged 原顺序。字体 url('vendor/font-awesome/fonts/...') 相对 assets/，
// 输出 assets/vendor.css 后无需改写。
const VENDOR_CSS = [
  'src/css/argon.min.css',
  'src/vendor/font-awesome/css/font-awesome-min.css',
  'src/vendor/izitoast/css/iziToast.css',
  'src/vendor/pickr/themes/monolith.min.css',
  'src/vendor/fancybox/jquery.fancybox.min.css',
];

const concatJs = (files) => files.map(read).join('\n;\n');

// 注意：esbuild transform 不指定 format，保留顶层作用域（不包裹模块）。
const minifyJs = async (code) =>
  (await esbuild.transform(code, { minify: true, loader: 'js' })).code;
const minifyCss = async (code) =>
  (await esbuild.transform(code, { minify: true, loader: 'css' })).code;

writeFileSync(at('assets/vendor.js'), await minifyJs(concatJs(VENDOR_JS)));
writeFileSync(at('assets/vendor.css'), await minifyCss(concatJs(VENDOR_CSS)));
writeFileSync(at('assets/app.js'), await minifyJs(read('src/scripts/main.js')));

console.log('已产出 assets/vendor.js, assets/vendor.css, assets/app.js');
```

- [ ] **Step 2: 把 esbuild 加入 devDependencies 并安装**

在 `package.json` 的 `devDependencies` 加入 `"esbuild": "^0.25.0"`（版本以 `node_modules/esbuild/package.json` 中的实际版本为准，运行 `node -p "require('esbuild/package.json').version"` 获取后填对应 `^` 范围）。

Run: `npm install && node -p "require('esbuild/package.json').version"`
Expected: 安装成功并打印版本号。

- [ ] **Step 3: 单独运行构建脚本验证**

Run: `node scripts/build-vendor.mjs && ls -l assets/vendor.js assets/vendor.css assets/app.js`
Expected: 三个文件存在且非空；`vendor.js` 约 400~550KB、`vendor.css` 约 250~340KB、`app.js` 约 50~70KB（均为压缩后单行）。

- [ ] **Step 4: 校验关键全局存在于产物中**

Run:
```bash
for s in "jQuery" "Headroom" "iziToast" "noUiSlider" "Tippy" "ClipboardJS" "Pickr"; do
  grep -q "$s" assets/vendor.js && echo "vendor.js 含 $s" || echo "缺失 $s"
done
grep -q "ShareJS" assets/vendor.js && echo "FAIL: 仍含 sharejs" || echo "OK: 无 sharejs"
grep -q "argonConfig" assets/app.js && echo "app.js 含 argontheme 逻辑"
```
Expected: 7 行「含/缺失」全部为「含 …」；`OK: 无 sharejs`；`app.js 含 argontheme 逻辑`。

- [ ] **Step 5: 提交**

```bash
git add scripts/build-vendor.mjs package.json package-lock.json
git commit -m "build: add concat+minify vendor build script"
```

---

## Task 5: 改造 Vite 配置与入口（产出 app.css）

**Files:**
- Create: `src/entries/style.js`
- Modify: `vite.config.js`
- Modify: `package.json`（scripts）

- [ ] **Step 1: 新建 Vite 入口**

创建 `src/entries/style.js`：

```js
// Vite 入口：仅编译主题自有样式（style.scss → assets/app.css，压缩）。
import '../styles/main.scss';
```

- [ ] **Step 2: 重写 vite.config.js**

将 `vite.config.js` 整体替换为：

```js
import { defineConfig } from 'vite';

// Vite 负责主题自有样式：src/styles/main.scss → assets/app.css（sass 编译 + 压缩）。
// 经典脚本（vendor.js/app.js）由 scripts/build-vendor.mjs 用 esbuild 拼接+压缩产出，
// 因为 UMD 全局库与依赖全局 argonConfig 的 argontheme.js 不能被 ESM 打包。
export default defineConfig({
  build: {
    outDir: 'assets',
    emptyOutDir: false, // 保留 assets/vendor/font-awesome、assets/vendor/highlight/styles 等静态资源
    cssMinify: true,
    lib: {
      entry: 'src/entries/style.js',
      formats: ['iife'],
      name: 'ArgonTheme',
      fileName: () => 'app.js',
    },
    rollupOptions: {
      output: {
        // 固定文件名（无 hash），适配 Halo 模板的 ?version= 引用
        assetFileNames: (info) => (info.name && info.name.endsWith('.css') ? 'app.css' : '[name][extname]'),
      },
    },
  },
});
```

- [ ] **Step 3: 更新 package.json 脚本**

把 `scripts` 改为：

```json
"scripts": {
  "build": "vite build && node scripts/build-vendor.mjs",
  "build:vendor": "node scripts/build-vendor.mjs",
  "watch": "vite build --watch"
}
```

（删除 `build:js`、`build:css`；sass 由 Vite 接管，不再单独调用 `sass` 命令。）

- [ ] **Step 4: 运行完整构建**

Run: `npm run build`
Expected: Vite 构建成功，随后打印 `已产出 assets/vendor.js, assets/vendor.css, assets/app.js`。注意 Vite 先生成空的 `assets/app.js`，随后被 build-vendor 覆盖为经典脚本。

- [ ] **Step 5: 校验 app.css 已压缩且为固定名**

Run:
```bash
ls -l assets/app.css && wc -l assets/app.css && grep -c "/\*" assets/app.css
```
Expected: 文件存在；`wc -l` 为 `1`（压缩为单行）；`grep -c "/*"` 为 `0`（无注释残留）。若 `wc -l` 大于 1 说明未压缩，检查 `cssMinify`。

- [ ] **Step 6: 提交**

```bash
git add src/entries/style.js vite.config.js package.json
git commit -m "build: vite emits fixed-name app.css, drop separate sass step"
```

---

## Task 6: 更新模板引用

**Files:**
- Modify: `module/head.tmpl:19`, `module/head.tmpl:20`, `module/head.tmpl:37-38`
- Modify: `module/scripts.tmpl:2`

- [ ] **Step 1: 改 head.tmpl 的 CSS 引用**

把 `module/head.tmpl` 第 19、20 行改为：

```html
<link rel="stylesheet" href="{{ theme_base }}/assets/vendor.css?version={{ theme.Version }}">
<link rel="stylesheet" href="{{ theme_base }}/assets/app.css?version={{ theme.Version }}">
```

- [ ] **Step 2: 改 head.tmpl 的 JS 引用并删除 argon.min.js**

把第 37 行改为 `vendor.js`，并删除第 38 行（`assets/js/argon.min.js`，已并入 vendor.js）：

```html
<script src="{{ theme_base }}/assets/vendor.js?version={{ theme.Version }}"></script>
```

（第 38 行整行删除。）

- [ ] **Step 3: 改 scripts.tmpl 引用**

把 `module/scripts.tmpl` 第 2 行改为：

```html
<script src="{{ theme_base }}/assets/app.js?version={{ theme.Version }}"></script>
```

（第 13 行动态高亮主题 `assets/vendor/highlight/styles/...` **保持不变**。）

- [ ] **Step 4: 校验模板已无旧引用**

Run:
```bash
grep -rn "argon_css_merged\|argon_js_merged\|assets/js/argon.min.js\|\"style.css\|/argontheme.js" module/ layouts/ *.tmpl | grep -v node_modules | wc -l
```
Expected: `0`

- [ ] **Step 5: 提交**

```bash
git add module/head.tmpl module/scripts.tmpl
git commit -m "refactor: point templates at vendor.*/app.* fixed-name assets"
```

---

## Task 7: 删除旧产物与清理依赖

**Files:**
- Delete: `assets/argon_css_merged.css`、`assets/argon_js_merged.js`、根 `style.css`、根 `argontheme.js`
- Modify: `package.json`（清理未用 `dependencies`）
- Modify: `.gitignore`（可选）

- [ ] **Step 1: 删除旧产物与旧引用文件**

Run:
```bash
git rm assets/argon_css_merged.css assets/argon_js_merged.js style.css argontheme.js
```
Expected: 4 个文件删除成功。（`style.css`、`argontheme.js` 是根目录旧构建产物。）

- [ ] **Step 2: 清理 package.json 中未被取源使用的依赖**

由于第三方库现由 `src/vendor/**`（从 merged 拆出）提供，`dependencies` 中的 `@fancyapps/fancybox`、`bootstrap`、`clamp-js`、`headroom.js`、`highlight.js`、`highlightjs-line-numbers.js`、`izitoast`、`jquery`、`jquery-pjax`、`nouislider`、`nprogress`、`popper.js`、`tippy.js` 均未被打包引用，删除整个 `dependencies` 字段。

Run: `node -e "const p=require('./package.json'); delete p.dependencies; require('fs').writeFileSync('package.json', JSON.stringify(p,null,2)+'\n')"`
Expected: `package.json` 不再有 `dependencies`；`devDependencies` 保留 `sass`、`vite`、`esbuild`。

- [ ] **Step 3: 重新安装并确认构建仍通过**

Run: `rm -rf node_modules package-lock.json && npm install && npm run build`
Expected: 安装与构建均成功；`assets/vendor.js`、`assets/vendor.css`、`assets/app.js`、`assets/app.css` 四个文件齐全。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "chore: remove merged files, root artifacts and unused npm deps"
```

---

## Task 8: 全量验证（构建产物 + 运行预览）

**Files:**
- Test: 构建产物断言 + Halo 预览

- [ ] **Step 1: 断言四个固定名产物齐全且体积达标**

Run:
```bash
ls -l assets/vendor.js assets/vendor.css assets/app.js assets/app.css
python3 -c "import os; print('CSS 合计', os.path.getsize('assets/vendor.css')+os.path.getsize('assets/app.css'))"
```
Expected: 四文件存在；CSS 合计 < 514000 字节（原 `argon_css_merged.css` 358106 + `style.css` 156668 的未压缩总量）。

- [ ] **Step 2: 断言产物为压缩态**

Run:
```bash
for f in assets/vendor.css assets/app.css assets/vendor.js assets/app.js; do
  echo "$f 行数=$(wc -l < $f)";
done
```
Expected: 每个文件行数为 `1`（压缩为单行）。

- [ ] **Step 3: 启动 Halo 预览并逐项检查**

启动本地 Halo（或使用既有预览环境）加载该主题，打开首页与一篇含代码块的文章，检查：
1. 页面样式正常（无 FOUC / 无裸 HTML）。
2. 控制台**无** 404、无 `jQuery is not defined`、无 `Headroom is not defined`、无 `iziToast is not defined`。
3. 夜间模式切换正常。
4. 代码高亮主题正常（验证 `assets/vendor/highlight/styles/<theme>.css` 可访问）。
5. 站内搜索 / pjax 翻页 / fancybox 灯箱 / 复制按钮 / 主题色取色器（Pickr）正常。
6. 字体图标（`fa fa-*`）正常显示（验证 `assets/vendor/font-awesome/fonts/*` 可访问）。

Expected: 6 项全部通过。任何一项失败 → 回到对应 Task 修正（例如全局缺失在 Task 4 的列表补入/调整顺序）。

- [ ] **Step 4: 记录验证结果**

在 `docs/superpowers/plans/2026-09-29-theme-argon-asset-pipeline.md` 末尾追加一节 `## 验证记录`，粘贴 Step 1–2 的实际输出与 Step 3 的手动检查结论。

- [ ] **Step 5: 提交**

```bash
git add docs/superpowers/plans/2026-09-29-theme-argon-asset-pipeline.md
git commit -m "test: record asset pipeline build & preview verification"
```

---

## Task 9: 收尾

**Files:**
- Modify: `.gitignore`（如需要）

- [ ] **Step 1: 确认 .gitignore 不误伤产物**

Run: `git check-ignore -v assets/vendor.js assets/vendor.css assets/app.js assets/app.css || echo "OK: 产物未被忽略"`
Expected: `OK: 产物未被忽略`（若被忽略，从 `.gitignore` 移除对应规则后重试）。

- [ ] **Step 2: 确认待提交文件清单**

Run: `git status --short`
Expected: 仅包含本计划涉及的文件；不包含 `handler/router.go`、`airpress_test.exe`、`resources/template/theme/anatole` 等无关改动。

- [ ] **Step 3: 最终提交并总结**

Run: `git log --oneline refactor/vite-asset-pipeline -10`
Expected: 可见本计划各任务的提交。向用户汇报：删除的 merged 文件、四个固定名产物、CSS 压缩比例、验证结论。

---

## 自检

**Spec 覆盖：**
- §3 目标产物结构（固定名 vendor.*/app.*）→ Task 4/5/6。
- §4 审计后精简（删 sharejs，保留其余）→ Task 1（DROP）、Task 2（复核）。
- §4.2 本地库抽取到 src/vendor → Task 1。
- §4.3 动态高亮主题保留静态目录 → Task 6 Step 3（保留）+ Task 8 Step 3.4。
- §5 构建机制（经典脚本拼接+esbuild；Vite 管 scss）→ Task 4 + Task 5。
- §6 模板改动 → Task 6。
- §7 删除项与 .gitignore → Task 7 + Task 9。
- §8 验证标准 → Task 8。
- §9 风险（全局缺失）→ Task 4 Step 4 + Task 8 Step 3。

**占位符扫描：** 无 TBD/TODO；所有步骤含可执行命令或完整代码；esbuild 版本要求先以实际安装版本回填（Task 4 Step 2 给出获取命令）。

**类型/命名一致性：** 产物名统一为 `vendor.js`/`vendor.css`/`app.js`/`app.css`；入口 `src/entries/style.js`；脚本 `scripts/split-merged.mjs`、`scripts/build-vendor.mjs` 全篇一致。

---

## 验证记录（执行后补录）

**执行日期：** 2026-09-29
**分支：** `refactor/vite-asset-pipeline`
**提交链：** `b76dc03`(拆分) → `275db24`(迁移Argon DS) → `360010e`(构建脚本) → `1944c61`(Vite配置) → `b32d3fb`(模板引用) → `bcbcd06`(清理依赖)

### Task 8 Step 1–2 构建产物断言（实际输出）

```
assets/app.css      129918 B   行数=1   无 /* / 无 $vite$ 标记
assets/app.js        77091 B   行数=16  含 argonConfig（全局作用域保留）
assets/vendor.css   348115 B   行数=1   无 /*
assets/vendor.js    548554 B   行数=76  含 jQuery/Headroom/iziToast/noUiSlider/tippy/ClipboardJS/Pickr；无 sharejs
CSS 合计 478033 字节  < 514000（原 argon_css_merged 358106 + style.css 156668 未压缩总量）
```

**说明：**
- `app.css`/`vendor.css` 为压缩单行（CSS 无多行模板字面量）；`app.js`/`vendor.js` 因源码含多行 HTML 字面量保留换行，属正确行为（代码空白仍压缩）。
- **CSS 压缩目标达成**：压缩态 478KB vs 原未压缩态 515KB。

### Task 8 Step 3 运行时 Halo 预览（本环境无法启动，需用户确认）

1. 页面样式正常、无 404、无 `jQuery is not defined`/`Headroom is not defined`/`iziToast is not defined`
2. 夜间模式、pjax、fancybox、代码高亮、复制、Pickr 正常
3. `fa fa-*` 图标正常（`assets/vendor/font-awesome/fonts/*` 可访问）
4. 代码高亮主题正常（`assets/vendor/highlight/styles/<theme>.css` 可访问）

### 关键发现：`.gitignore` 模式（修正 Task 9 Step 1 预期）

父级 `resources/template/theme/.gitignore` 含 `*` 规则，所有主题文件默认被忽略，需 `git add -f`；旧 `style.css`/`argontheme.js`/`argon_*.css/js` **从未被跟踪**。**构建产物被忽略是既有模式、符合预期**，非缺陷。

### Dev 环境注意：DrvFs 重命名权限

`/mnt/d`（WSL DrvFs）上 `npm install` 剪枝目录偶发 `EACCES`（重命名）。规避：用 `npm install --package-lock-only` 重建锁文件；纯构建不受影响。

### 其他技术结论

- **`strip-vite-css-marker` 插件确属必要**：无插件时 `app.css` 含 `/*$vite$:1*/`（grep 计 1），带插件为 0；源码 `node_modules/vite/dist/node/chunks/node.js:30355` 的 `finalizeCss()` 无条件追加该标记。
- **esbuild 非 Vite 8 自带**（Vite 8 用 Rolldown），已在 devDependencies 声明 `^0.28.0`。
- **sharejs 已精简删除**（零调用）。
