package text

import "strings"

func BulletIndent(bullet string, text string) string {
	indentBytes := make([]byte, len(bullet), len(bullet))
	for i, _ := range bullet {
		indentBytes[i] = ' '
	}
	indentText := string(indentBytes)
	text = Indent(text, indentText)
	text = strings.TrimPrefix(text, indentText)
	return bullet + text
}

func Indent(text, indent string) string {
	if len(text) != 0 && text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}
