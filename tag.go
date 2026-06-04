package xtract

import (
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	xpathlib "github.com/antchfx/xpath"
	"golang.org/x/net/html"
)

type xpathTag struct {
	Xpath string
}

func newXpathTag(tag string) *xpathTag {
	return &xpathTag{
		Xpath: tag,
	}
}

type searchContext struct {
	doc    *html.Node
	source *html.Node
}

func newSearchContext(doc *html.Node) *searchContext {
	return &searchContext{
		doc:    doc,
		source: doc,
	}
}

func (ctx *searchContext) Search(xpath string) ([]*searchContext, error) {
	nodes, err := htmlquery.QueryAll(ctx.doc, xpath)
	if err != nil {
		return nil, fmt.Errorf("invalid xpath found in struct tag. tag=%s", xpath)
	}
	if nodes == nil {
		return nil, nil
	}

	ret := make([]*searchContext, len(nodes))
	for i, node := range nodes {
		ret[i] = &searchContext{
			doc:    node,
			source: resolveSourceNode(ctx.doc, xpath, node),
		}
	}

	return ret, nil
}

// Text returns the text matched by the given xpath.
// If xpath is empty, it returns the whole content of the current context.
func (ctx *searchContext) Text(xpath string) (string, error) {
	text, _, err := ctx.TextNode(xpath)
	return text, err
}

// TextNode returns the text matched by the given xpath and the node that best
// represents the source location for that match.
func (ctx *searchContext) TextNode(xpath string) (string, *html.Node, error) {
	if xpath == "" {
		return htmlquery.InnerText(ctx.doc), ctx.source, nil
	}

	expr, err := xpathlib.Compile(xpath)
	if err != nil {
		return "", nil, err
	}

	v := expr.Evaluate(htmlquery.CreateXPathNavigator(ctx.doc))
	switch v.(type) {
	case *xpathlib.NodeIterator:
		ctxs, err := ctx.Search(xpath)
		if err != nil {
			return "", nil, err
		}
		if len(ctxs) == 0 {
			return "", nil, nil
		}
		return htmlquery.InnerText(ctxs[0].doc), ctxs[0].source, nil
	}

	return v.(string), ctx.source, nil
}

func resolveSourceNode(ctxDoc *html.Node, xpath string, node *html.Node) *html.Node {
	if node == nil {
		return ctxDoc
	}
	if node.Parent != nil {
		return node
	}

	ownerXPath, ok := attributeOwnerXPath(xpath)
	if !ok {
		return ctxDoc
	}

	nodes, err := htmlquery.QueryAll(ctxDoc, ownerXPath)
	if err != nil || len(nodes) == 0 {
		return ctxDoc
	}

	return nodes[0]
}

func attributeOwnerXPath(xpath string) (string, bool) {
	idx := strings.LastIndex(xpath, "/@")
	if idx < 0 {
		return "", false
	}

	owner := strings.TrimSpace(xpath[:idx])
	if owner == "" {
		return "", false
	}

	return owner, true
}
