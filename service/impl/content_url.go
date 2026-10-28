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

// extractContentTargets 提取正文中 HTML 属性与 Markdown 链接里的 URL，按出现顺序去重。
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
func replaceContentTargets(content string, replacements map[string]string) string {
	content = htmlURLAttrPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := htmlURLAttrPattern.FindStringSubmatch(match)
		attr, sep, value := sub[1], sub[2], sub[4]
		quote := `"`
		if value == "" {
			value = sub[5]
			quote = `'`
		}
		replacement, ok := replacements[value]
		if !ok {
			return match
		}
		return attr + sep + quote + html.EscapeString(replacement) + quote
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
