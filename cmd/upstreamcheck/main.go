// upstreamcheck only reads supplier connection, catalog, prices and inventory.
// It has no code path that creates a purchase or consumes a merchant balance.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/upstream"
)

func main() {
	adapter, err := upstream.NewAdapter(&siteconnectiondomain.Connection{
		BaseURL: os.Getenv("UPSTREAM_BASE_URL"), ApiKey: os.Getenv("UPSTREAM_API_KEY"), ApiSecret: os.Getenv("UPSTREAM_API_SECRET"), Protocol: constants.ConnectionProtocolACGFaka,
	}, "")
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	account, err := adapter.Ping(ctx)
	if err != nil {
		fail(err)
	}
	catalog, err := adapter.ListProducts(ctx, upstream.ListProductsOpts{Page: 1, PageSize: 100})
	if err != nil {
		fail(err)
	}
	type product struct {
		ID     uint   `json:"id"`
		Name   any    `json:"name"`
		Cost   string `json:"cost_cny"`
		Active bool   `json:"supported"`
		Stock  int    `json:"stock"`
	}
	products := []product{}
	for _, item := range catalog.Items {
		stock := 0
		if len(item.SKUs) > 0 {
			stock = item.SKUs[0].StockQuantity
		}
		products = append(products, product{item.ID, item.Title, item.PriceAmount, item.IsActive, stock})
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"site": account.SiteName, "merchant_id": account.UserID, "balance": account.Balance, "currency": account.Currency, "total": catalog.Total, "products": products, "purchases_created": 0}); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
