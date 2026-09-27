package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchArgs struct {
	Query string `json:"query" jsonschema:"required,Inventory item name or search phrase"`
}

type offer struct {
	SKU               string  `json:"sku"`
	Name              string  `json:"name"`
	UnitPrice         float64 `json:"unitPrice"`
	Currency          string  `json:"currency"`
	AvailableQuantity int     `json:"availableQuantity"`
	ShippingCost      float64 `json:"shippingCost"`
	DeliveryDays      int     `json:"deliveryDays"`
}

var offers = []offer{
	{SKU: "LAMP-LOW", Name: "Desk lamp", UnitPrice: 12, Currency: "USD", AvailableQuantity: 0, ShippingCost: 5, DeliveryDays: 3},
	{SKU: "LAMP-READY", Name: "Desk lamp", UnitPrice: 15, Currency: "USD", AvailableQuantity: 5, ShippingCost: 4, DeliveryDays: 2},
	{SKU: "LAMP-EXPRESS", Name: "Desk lamp", UnitPrice: 18, Currency: "USD", AvailableQuantity: 8, ShippingCost: 0, DeliveryDays: 1},
	{SKU: "BULB-BASIC", Name: "LED bulb", UnitPrice: 4, Currency: "USD", AvailableQuantity: 10, ShippingCost: 3, DeliveryDays: 4},
	{SKU: "BULB-FAST", Name: "LED bulb", UnitPrice: 5, Currency: "USD", AvailableQuantity: 4, ShippingCost: 1, DeliveryDays: 2},
}

func search(query string) []offer {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := make([]offer, 0)
	if query == "" {
		return matches
	}
	for _, item := range offers {
		if strings.Contains(strings.ToLower(item.Name+" "+item.SKU), query) {
			matches = append(matches, item)
		}
	}
	return matches
}

func newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "supplier-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name: "search_offers", Description: "List matching supplier offers with price, stock, shipping, and delivery facts. Offers are not ranked or selected.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input searchArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(input.Query) == "" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "query is required"}}}, nil, nil
		}
		result := map[string]any{"offers": search(input.Query)}
		data, _ := json.Marshal(result)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, result, nil
	})
	return s
}

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "supplier-mcp takes no arguments")
		os.Exit(2)
	}
	if err := newServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
