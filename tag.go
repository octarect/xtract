package xtract

import (
	"fmt"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type xpathTag struct {
	xpath string
}

func newXpathTag(s string) *xpathTag {
	return &xpathTag{
		xpath: s,
	}
}

// Search the HTML document using the provided XPath tag and returns the inner texts of the matched nodes.
func (t *xpathTag) Search(doc *html.Node) ([]string, error) {
	nodes, err := htmlquery.QueryAll(doc, t.xpath)
	if err != nil {
		return nil, fmt.Errorf("invalid xpath found in struct tag. tag=%s", t.xpath)
	}
	if nodes == nil {
		return nil, nil
	}

	ret := make([]string, len(nodes))
	for i, node := range nodes {
		ret[i] = htmlquery.InnerText(node)
	}

	return ret, nil
}
