import { defineConfig } from 'vite';

// Vite 负责主题自有样式：src/style.scss → assets/app.css（sass 编译 + 压缩）。
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
  plugins: [
    {
      name: 'strip-vite-css-marker',
      enforce: 'post',
      // Vite 8 finalizeCss() 会在 CSS 末尾追加 /*$vite$:1*/ 内部哈希标记，
      // lib 模式下内置 generateBundle 的清除步骤未生效，需手动清除以保证产出纯净。
      generateBundle(_opts, bundle) {
        for (const asset of Object.values(bundle)) {
          if (asset.type !== 'asset' || !asset.fileName.endsWith('.css')) continue;
          if (typeof asset.source === 'string') {
            asset.source = asset.source.replace(/\/\*\$vite\$:\d+\*\//g, '');
          }
        }
      },
    },
  ],
});
