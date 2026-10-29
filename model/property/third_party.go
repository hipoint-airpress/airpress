package property

import "reflect"

// ThirdPartyJSCDN 是第三方 JS 资源（编辑器扩展、评论组件等）的 CDN 根地址。
// 各调用方在此地址后拼接自己的包路径，默认为兼容 jsdelivr 的 https://cdn.jsdelivr.net。
var ThirdPartyJSCDN = Property{
	KeyValue:     "third_party_js_cdn",
	DefaultValue: "https://cdn.jsdelivr.net",
	Kind:         reflect.String,
}