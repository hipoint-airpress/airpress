# 全局第三方静态资源(vendor)

所有主题共享的第三方 JS/CSS 资源目录,由 `handler/router.go` 挂载到站点根路径 `/vendor/`
(`common` 命名空间即全站公共,主题无需各自携带副本,避免漏放导致运行时报错;
同时兼容内网等外部 CDN 不可达的部署环境)。

## 当前清单

| 文件 | 版本 | 来源 | 许可证 | SHA256 |
|---|---|---|---|---|
| `vue/2.6.14/vue.min.js` | 2.6.14 | https://cdn.jsdelivr.net/npm/vue@2.6.14/dist/vue.min.js | MIT | `9174c425c445377df4562ad9165ea08fdf9433a808296d7de5f619791df10e17` |
| `halo-comment/1.3.2/halo-comment.min.js` | 1.3.2 | https://cdn.jsdelivr.net/npm/halo-comment@1.3.2/dist/halo-comment.min.js | MIT | `d65f2ff42494f25d56d5fde033a67143d795351b6712388bb39486e7ceb7b5e0` |

## 约定

- 按 `<库名>/<版本号>/<文件名>` 组织;引用方(如 `resources/template/common/macro/comment.tmpl`)写死版本化路径,升级时新增目录并改引用,新旧共存互不影响。
- `halo-comment.min.js` 依赖全局 Vue **2**(内部使用 `Vue.extend`),不能与 `airpress_comment.tmpl` 加载的 Vue 3 混用。
- 本目录下的文件不参与 pongo2 模板编译(加载器仅识别 `*.tmpl`)。
