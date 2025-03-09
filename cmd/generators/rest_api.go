package generators

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIGenerator struct {
	url      string
	client   *http.Client
	page     int
	pageSize int
	hasMore  bool
}

func NewAPIGenerator(url string, pageSize int, client *http.Client) func() ([]map[string]any, bool, error) {
	if client == nil {
		client = http.DefaultClient
	}
	gen := &APIGenerator{
		url:      url,
		client:   client,
		page:     0,
		pageSize: pageSize,
		hasMore:  true,
	}
	return gen.fetch
}

func (g *APIGenerator) fetch() ([]map[string]any, bool, error) {
	if !g.hasMore {
		return nil, false, nil
	}
	g.page++
	resp, err := g.client.Get(fmt.Sprintf("%s?page=%d&pageSize=%d", g.url, g.page, g.pageSize))
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	var data []map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, err
	}
	g.hasMore = len(data) == g.pageSize
	return data, g.hasMore, nil
}

// func main() {
// 	apiGen := NewAPIGenerator("https://api.example.com/data", 10, nil)
// 	for {
// 		data, hasMore, err := apiGen()
// 		if err != nil {
// 			fmt.Println("Error:", err)
// 			break
// 		}
// 		if !hasMore {
// 			break
// 		}
// 		fmt.Println("Fetched:", data)
// 	}
// }
