package xtract

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func mustParse(doc string) *searchContext {
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		panic(err)
	}
	return newSearchContext(node)
}

func TestXpathTagSearch(t *testing.T) {
	sc := mustParse(`
	<div>
		<span class="field">foo</span>
		<span class="field">bar</span>
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
			[]string{"foo"},
			false,
		},
		{
			"mutiple",
			`//span[@class="field"]`,
			[]string{"foo", "bar"},
			false,
		},
		{
			"Not found",
			`//span[@class="none"]`,
			[]string{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xtag := newXpathTag(tt.tag)
			scs, err := sc.Search(xtag.Xpath)

			texts := make([]string, 0, len(scs))
			for _, sc0 := range scs {
				text, err := sc0.Text("")
				if err != nil {
					t.Fatal(err)
				}

				texts = append(texts, text)
			}

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

func TestXpathTagText(t *testing.T) {
	doc := `
	<address>
		<div class="address1">1-1 Chiyoda, Chiyoda-ku</div>
		<div class="address2"><span class="city">Tokyo</span>, <span class="country">Japan</span></div>
	</address>
	`
	sc := mustParse(doc)

	tests := []struct {
		name,
		xpath,
		want string
		wantErr bool
	}{
		{
			"empty",
			"",
			"\n\t\t1-1 Chiyoda, Chiyoda-ku\n\t\tTokyo, Japan\n\t\n\t",
			false,
		},
		{
			"invalid xpath",
			"//a[id=']",
			"",
			true,
		},
		{
			"matched multiple nodes",
			"//div[@class='address2']",
			"Tokyo, Japan",
			false,
		},
		{
			"top-level function",
			"lower-case(//span[@class='country'])",
			"japan",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sc.Text(tt.xpath)
			if !reflect.DeepEqual(got, tt.want) {
				fmt.Printf("%#v\n", got)
				t.Errorf("SearchContext.Text() = %v, want %v", got, tt.want)
			}
			if tt.wantErr != (err != nil) {
				t.Errorf("SearchContext.Text() unexpected error status: %v", err)
			}
		})
	}
}
