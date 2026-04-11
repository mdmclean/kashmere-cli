// internal/mcp/prices.go
package mcp

import (
	"context"
	"net/url"
	"strings"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPriceTools(server *sdkmcp.Server, c *api.Client) {
	// get_prices
	type getPricesInput struct {
		Tickers []string `json:"tickers,omitempty" jsonschema:"Optional list of ticker symbols to filter (e.g. VCN, VFV). Omit to get all tracked prices."`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_prices",
		Description: "Get current prices for tracked tickers. Optionally filter by specific tickers.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getPricesInput) (*sdkmcp.CallToolResult, any, error) {
		path := "/prices"
		if len(in.Tickers) > 0 {
			params := url.Values{}
			params.Set("tickers", strings.Join(in.Tickers, ","))
			path += "?" + params.Encode()
		}
		var prices []api.TickerPrice
		if err := c.Get(path, &prices); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(prices), nil, nil
	})

	// get_ticker_price
	type getTickerPriceInput struct {
		Ticker   string  `json:"ticker" jsonschema:"Ticker symbol (e.g. VCN)"`
		Exchange *string `json:"exchange,omitempty" jsonschema:"Optional exchange (e.g. TSX, NYSE)"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_ticker_price",
		Description: "Get the current price for a single ticker symbol",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getTickerPriceInput) (*sdkmcp.CallToolResult, any, error) {
		path := "/prices/" + url.PathEscape(in.Ticker)
		if in.Exchange != nil {
			params := url.Values{}
			params.Set("exchange", *in.Exchange)
			path += "?" + params.Encode()
		}
		var price api.TickerPrice
		if err := c.Get(path, &price); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(price), nil, nil
	})

	// test_ticker_price
	type testTickerPriceInput struct {
		Ticker   string  `json:"ticker" jsonschema:"Ticker symbol (e.g. VCN)"`
		Exchange *string `json:"exchange,omitempty" jsonschema:"Optional exchange (e.g. TSX, NYSE)"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "test_ticker_price",
		Description: "Test if a ticker price is available in cache or scheduled for fetching",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in testTickerPriceInput) (*sdkmcp.CallToolResult, any, error) {
		path := "/prices/test/" + url.PathEscape(in.Ticker)
		if in.Exchange != nil {
			params := url.Values{}
			params.Set("exchange", *in.Exchange)
			path += "?" + params.Encode()
		}
		var result any
		if err := c.Get(path, &result); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(result), nil, nil
	})
}
