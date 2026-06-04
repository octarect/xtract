package xtract

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

const maxDisplayedSourceLineLength = 120

type UnmarshalError struct {
	Err        error
	XPath      string
	LineNumber int
	Context    []SourceLine
}

func (e *UnmarshalError) Error() string {
	if e.LineNumber > 0 && len(e.Context) > 0 {
		var b strings.Builder
		width := len(fmt.Sprintf("%d", e.Context[len(e.Context)-1].Number))
		fmt.Fprintf(&b, "Error: %v\n", e.Err)
		fmt.Fprintf(&b, "  XPath: %q\n", e.XPath)
		for _, line := range e.Context {
			prefix := " "
			if line.Number == e.LineNumber {
				prefix = ">"
			}
			fmt.Fprintf(&b, "%s %*d | %s\n", prefix, width, line.Number, truncateSourceLine(line.Text))
		}
		return strings.TrimRight(b.String(), "\n")
	}
	if e.XPath != "" {
		return fmt.Sprintf("Error: %v\n  XPath: %q", e.Err, e.XPath)
	}
	return e.Err.Error()
}

type SourceLine struct {
	Number int
	Text   string
}

type sourceLocator struct {
	lines        []string
	elementLines map[*html.Node]int
}

func newSourceLocator(source []byte, root *html.Node) *sourceLocator {
	locator := &sourceLocator{
		lines:        splitSourceLines(source),
		elementLines: map[*html.Node]int{},
	}
	locator.index(root, source)
	return locator
}

func (l *sourceLocator) Location(node *html.Node) (int, []SourceLine, bool) {
	for current := node; current != nil; current = current.Parent {
		lineNumber, ok := l.elementLines[current]
		if !ok {
			continue
		}
		if lineNumber <= 0 || lineNumber > len(l.lines) {
			return 0, nil, false
		}
		return lineNumber, l.context(lineNumber, 2), true
	}

	return 0, nil, false
}

func (l *sourceLocator) context(lineNumber, radius int) []SourceLine {
	start := max(1, lineNumber-radius)
	end := min(len(l.lines), lineNumber+radius)
	lines := make([]SourceLine, 0, end-start+1)
	for i := start; i <= end; i++ {
		lines = append(lines, SourceLine{
			Number: i,
			Text:   l.lines[i-1],
		})
	}
	return lines
}

func (l *sourceLocator) index(root *html.Node, source []byte) {
	tokens := collectElementTokens(source)
	if len(tokens) == 0 {
		return
	}

	i := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || i >= len(tokens) {
			return
		}

		if node.Type == html.ElementNode && node.Data == tokens[i].name {
			l.elementLines[node] = tokens[i].line
			i++
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(root)
}

type elementToken struct {
	name string
	line int
}

func collectElementTokens(source []byte) []elementToken {
	tokenizer := html.NewTokenizer(bytes.NewReader(source))
	line := 1
	var tokens []elementToken

	for {
		tokenType := tokenizer.Next()
		raw := append([]byte(nil), tokenizer.Raw()...)
		if tokenType == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				break
			}
			return tokens
		}

		if tokenType == html.StartTagToken || tokenType == html.SelfClosingTagToken {
			name, _ := tokenizer.TagName()
			tokens = append(tokens, elementToken{
				name: string(name),
				line: line,
			})
		}

		line += bytes.Count(raw, []byte{'\n'})
	}

	return tokens
}

func splitSourceLines(source []byte) []string {
	rawLines := bytes.Split(source, []byte("\n"))
	lines := make([]string, len(rawLines))
	for i, rawLine := range rawLines {
		lines[i] = strings.TrimRight(string(rawLine), "\r")
	}
	return lines
}

func truncateSourceLine(s string) string {
	runes := []rune(s)
	if len(runes) <= maxDisplayedSourceLineLength {
		return s
	}
	if maxDisplayedSourceLineLength <= 3 {
		return string(runes[:maxDisplayedSourceLineLength])
	}
	return string(runes[:maxDisplayedSourceLineLength-3]) + "..."
}
