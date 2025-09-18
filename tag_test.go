package xtract

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func mustParse(doc string) *html.Node {
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		panic(err)
	}
	return node
}

func TestXpathTagSearch(t *testing.T) {
	doc := mustParse(`
	<div>
		<span class="field">1</span>
		<span class="field">2</span>
	</div>`)

	tests := []struct {
		name,
		tag string
		expected []string
		wantErr  bool
	}{
		{
			"single",
			`//span[@class="field"][1]`,
			[]string{"1"},
			false,
		},
		{
			"mutiple",
			`//span[@class="field"]`,
			[]string{"1", "2"},
			false,
		},
		{
			"Not found",
			`//span[@class="none"]`,
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xtag := newXpathTag(tt.tag)
			texts, err := xtag.Search(doc)
			if !reflect.DeepEqual(texts, tt.expected) {
				t.Errorf("XpathTag.Search() = %v, expected %v", texts, tt.expected)
				return
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("XpathTag.Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
