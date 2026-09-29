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
writeFileSync(at('assets/app.js'), await minifyJs(read('src/argontheme.js')));

console.log('已产出 assets/vendor.js, assets/vendor.css, assets/app.js');
