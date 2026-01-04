# xtract

[![Go Reference](https://pkg.go.dev/badge/github.com/octarect/xtract.svg)](https://pkg.go.dev/github.com/octarect/xtract)
[![Go Report Card](https://goreportcard.com/badge/github.com/octarect/xtract)](https://goreportcard.com/report/github.com/octarect/xtract)

A Go library for extracting data from HTML/XML documents into structs using XPath expressions via struct tags.

## Installation

```bash
go get github.com/octarect/xtract
```

## Quick Start

Run the following code:

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/octarect/xtract"
)

type Product struct {
	Name  string   `xpath:"//h1[@class='name']/text()"`
	Price float64  `xpath:"//span[@class='price']/@content"`
	Tags  []string `xpath:"//ul[@class='tags']/li/text()"`
}

func main() {
	html := `
		<html>
			<body>
				<h1 class="name">Effective XPath</h1>
				<span class="price" content="99.99">$99.99</span>
				<ul class="tags">
					<li>Programming</li>
					<li>Web Scraping</li>
					<li>XML</li>
				</ul>
			</body>
		</html>
	`

	var product Product
	if err := xtract.Unmarshal([]byte(html), &product); err != nil {
		log.Fatal(err)
	}

	b, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}
	fmt.Println(string(b))
}
```

Output:

```json
{
  "Name": "Effective XPath",
  "Price": 99.99,
  "Tags": [
    "Programming",
    "Web Scraping",
    "XML"
  ]
}
```

## Getting Started

### Define struct

You can use a struct tag to specify which value will be decoded into each field. Only exported fields will be decoded, just like encoding/json.

```go
type Product struct {
	Name  string  `xpath:"//*[@class='name']"`
	Price float32 `xpath:"//*[@class='price']"`
}
```

### Unmarshal

```go
html := `<html>...</html>`

var p Product
err := xtract.Unmarshal([]byte(html), &p)
```

### Nested Structure

Nested structures inherit context from their parent when the parent field has a struct tag.

#### Struct

```html
<div>
  <div class="name">Smartphone</div>
  <div class="manufacturer">
    <div class="name">Unnamed Company</div>
    <div class="country">US</div>
  </div>
</div>
```

```go
type Product struct {
	Name         string `xpath:"//div[@class='name']"`
	Manufacturer struct {
		Name    string `xpath:"//div[@class='name']"`
		Country string `xpath:"//div[@class='country']"`
	} `xpath:"//div[@class='manufacturer']"`
}

// The above struct can be also written redunduntly as:
// type Product struct {
//   Name string `xpath:"//div[@class='name']"`
//   Manufacturer struct {
//     Name    string `xpath:"//div[@class='manufacturer']//div[@class='name']"`
//     Name    string `xpath:"//div[@class='manufacturer']//div[@class='country']"`
//   }
// }
```

#### Slice & Map

```html
<table>
  <thead>
    <tr>
      <th class="name">Name</th>
      <th class="price">Price</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="name">ItemA</td>
      <td class="price">10</td>
    </tr>
    <tr>
      <td class="name">ItemB</td>
      <td class="price">20</td>
    </tr>
  </tbody>
</table>
```

```go
type Product struct {
  Name  string  `xpath:"//*[@class='name']"`
  Price float32 `xpath:"//*[@class='price']"`
}

// Slice
type ProductList struct {
  Products []Product `xpath:"//tbody/tr"`
}

// Map
type ProductListMap struct {
  Products map[string]Product `xpath:"//tbody/tr/td[@class='name']/text();//tbody/tr"`
}
```

### Custom Unmarshaler

It is useful to implement xtract.Unmarshaler interface on your types when you need to handle complex parsing logic.

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/octarect/xtract"
)

// UnixTime represents Unix timestamp
type UnixTime int64

// UnmarshalXPath implements xtract.Unmarshaler interface
func (ut *UnixTime) UnmarshalXPath(data []byte) error {
	t, err := time.Parse("2006-01-02 15:04:05", string(data))
	if err != nil {
		return err
	}

	*ut = UnixTime(t.Unix())

	return nil
}

type User struct {
	Email     string   `xpath:"//*[@class='email']"`
	CreatedAt UnixTime `xpath:"//*[@class='created-at']"`
}

func main() {
	html := `
		<span class="email">user@example.com</span>
		<span class="created-at">1970-01-01 01:23:45</span>
	`

	var data User
	if err := xtract.Unmarshal([]byte(html), &data); err != nil {
		log.Fatal(err)
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}
	fmt.Println(string(b))
}
```

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
