package impl

import (
	"html"
	"regexp"
	"strings"
)

var (
	// htmlURLAttrPattern 匹配 HTML 属性中的 URL，兼容单双引号；
	// 捕获完整属性名以保留 data-src 这类前缀，避免重写时破坏属性。
	htmlURLAttrPattern = regexp.MustCompile(`(?i)\b([\w-]*(?:src|href))(\s*=\s*)("([^"]*)"|'([^']*)')`)
	// markdownURLPattern 匹配 Markdown 链接/图片目标。
	markdownURLPattern = regexp.MustCompile(`(!?\[[^\]]*\]\(\s*<?)([^)\s>]+)(>?\s*[^)]*\))`)
)

// extractContentTargets 提取正文中 HTML 属性与 Markdown 链接里的 URL 并去重。
// 扫描顺序固定为：先扫描全部 HTML 属性，再扫描全部 Markdown 链接，因此返回
// 顺序并非 URL 在文档中的真实出现顺序。返回值为属性/链接目标原文，可能包含
// HTML 实体（如 &amp;），调用方需要时自行 unescape。
// 该实现基于正则，不解析文档结构：仅处理带引号的 HTML 属性，Markdown 目标
// 不支持含 ")" 的文件名（正则会在第一个 ")" 处截断）。
func extractContentTargets(content string) []string {
	seen := make(map[string]struct{})
	targets := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		targets = append(targets, value)
	}

	for _, match := range htmlURLAttrPattern.FindAllStringSubmatch(content, -1) {
		value := match[4]
		if value == "" {
			value = match[5]
		}
		add(value)
	}
	for _, match := range markdownURLPattern.FindAllStringSubmatch(content, -1) {
		add(match[2])
	}
	return targets
}

// replaceContentTargets 只替换 HTML 属性值与 Markdown 链接目标，正文文本不受影响。
// HTML 属性值查表前会裁剪首尾空白，与 extractContentTargets 保持一致；替换后的
// 输出不保留原有首尾空白。
func replaceContentTargets(content string, replacements map[string]string) string {
	content = htmlURLAttrPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := htmlURLAttrPattern.FindStringSubmatch(match)
		quoted := sub[3]
		value := sub[4]
		if value == "" {
			value = sub[5]
		}
		value = strings.TrimSpace(value)
		replacement, ok := replacements[value]
		if !ok {
			return match
		}
		quote := string(quoted[0])
		return sub[1] + sub[2] + quote + html.EscapeString(replacement) + quote
	})

	content = markdownURLPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := markdownURLPattern.FindStringSubmatch(match)
		replacement, ok := replacements[sub[2]]
		if !ok {
			return match
		}
		return sub[1] + replacement + sub[3]
	})
	return content
}
