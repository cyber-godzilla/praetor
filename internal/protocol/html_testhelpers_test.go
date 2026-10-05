package protocol

func ParseHTML(input string) HTMLResult {
	return ParseHTMLWithIndent(input, 0)
}
