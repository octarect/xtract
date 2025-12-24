package xtract

import (
	"fmt"

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
	doc *html.Node
}

func newSearchContext(doc *html.Node) *searchContext {
	return &searchContext{
		doc: doc,
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
		ret[i] = &searchContext{doc: node}
	}

	return ret, nil
}

// Text returns the text matched by the given xpath.
// If xpath is empty, it returns the whole content of the current context.
func (ctx *searchContext) Text(xpath string) (string, error) {
	if xpath == "" {
		return htmlquery.InnerText(ctx.doc), nil
	}

	expr, err := xpathlib.Compile(xpath)
	if err != nil {
		return "", err
	}

	v := expr.Evaluate(htmlquery.CreateXPathNavigator(ctx.doc))
	switch v.(type) {
	case *xpathlib.NodeIterator:
		ctxs, err := ctx.Search(xpath)
		if err != nil {
			return "", err
		}
		if len(ctxs) == 0 {
			return "", nil
		}
		return htmlquery.InnerText(ctxs[0].doc), nil
	}

	return v.(string), nil
}
