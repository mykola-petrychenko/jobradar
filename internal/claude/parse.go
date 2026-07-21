package claude

import (
	"regexp"
	"strings"
)

// tagPattern matches any HTML tag: <p>, </li>, <a href="...">
var tagPattern = regexp.MustCompile(`<[^>]*>`)

// entities we actually see in arbeitnow postings
var entityReplacer = strings.NewReplacer(
	"&#x26;", "&",
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&#39;", "'",
	"&nbsp;", " ",
)

// CleanHTML strips tags and decodes entities from posting description.
func CleanHTML(html string) string {
	text := tagPattern.ReplaceAllString(html, " ")
	text = entityReplacer.Replace(text)
	text = strings.ReplaceAll(text, `\n`, " ")
	text = strings.Join(strings.Fields(text), " ")
	return text
}
