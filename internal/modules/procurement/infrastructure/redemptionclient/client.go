// Package redemptionclient connects paid supplier fulfillment to the independent
// PAIPLUS issuer. It never buys supplier stock or submits an account recharge.
package redemptionclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/procurement/contract"
	"github.com/dujiao-next/internal/modules/procurement/domain"
)

type Client struct {
	origin       string
	token        string
	products     map[uint]bool
	connectionID uint
	orderFloor   uint
	paidAfter    time.Time
	http         *http.Client
}

func New(origin, token string, products []uint, connectionID, orderFloor uint, paidAfter time.Time) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u == nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid PAIPLUS origin")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return nil, errors.New("PAIPLUS requires HTTPS or loopback HTTP")
	}
	if len(token) < 32 || len(products) == 0 || connectionID == 0 || orderFloor == 0 || paidAfter.IsZero() {
		return nil, errors.New("PAIPLUS requires token, product scope and future-order bounds")
	}
	selected := make(map[uint]bool)
	for _, product := range products {
		if product == 0 {
			return nil, errors.New("invalid PAIPLUS product ID")
		}
		selected[product] = true
	}
	return &Client{origin: strings.TrimRight(origin, "/"), token: token, products: selected, connectionID: connectionID, orderFloor: orderFloor, paidAfter: paidAfter,
		http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("PAIPLUS redirect rejected") }}}, nil
}

func FromEnvironment() (contract.DeliveryRewriter, error) {
	if os.Getenv("PAIPLUS_ENABLED") != "true" {
		return nil, nil
	}
	parse := func(name string) (uint, error) {
		n, err := strconv.ParseUint(os.Getenv(name), 10, 32)
		return uint(n), err
	}
	connection, err := parse("PAIPLUS_CONNECTION_ID")
	if err != nil {
		return nil, errors.New("invalid PAIPLUS_CONNECTION_ID")
	}
	floor, err := parse("PAIPLUS_ORDER_ID_FLOOR")
	if err != nil {
		return nil, errors.New("invalid PAIPLUS_ORDER_ID_FLOOR")
	}
	after, err := time.Parse(time.RFC3339, os.Getenv("PAIPLUS_PAID_AFTER"))
	if err != nil {
		return nil, errors.New("invalid PAIPLUS_PAID_AFTER")
	}
	var products []uint
	for _, text := range strings.Split(os.Getenv("PAIPLUS_PRODUCT_IDS"), ",") {
		n, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
		if err != nil {
			return nil, errors.New("invalid PAIPLUS_PRODUCT_IDS")
		}
		products = append(products, uint(n))
	}
	return New(os.Getenv("PAIPLUS_ORIGIN"), os.Getenv("PAIPLUS_INTEGRATION_TOKEN"), products, connection, floor, after)
}

func (c *Client) RequiresRewrite(order *domain.LocalOrder, connectionID uint) bool {
	if order == nil || connectionID != c.connectionID || order.ID < c.orderFloor {
		return false
	}
	if order.PaidAt != nil && order.PaidAt.Before(c.paidAfter) {
		return false
	}
	for _, item := range order.Items {
		if c.products[item.ProductID] {
			return true
		}
	}
	return false
}

func (c *Client) Rewrite(order *domain.LocalOrder, connectionID uint, f *contract.Fulfillment) (*contract.Fulfillment, error) {
	if order == nil || f == nil {
		return nil, errors.New("missing redemption delivery input")
	}
	if !c.RequiresRewrite(order, connectionID) {
		return f, nil
	}
	if order.PaidAt == nil {
		return nil, errors.New("redemption delivery requires a paid order")
	}
	if len(order.Items) != 1 || order.Items[0].Quantity < 1 || order.Items[0].Quantity > 100 || f.Type != "auto" || len(f.DeliveryData) > 0 {
		return nil, errors.New("unsupported redemption delivery; preserve supplier payload for review")
	}
	request := struct {
		Reference string `json:"reference"`
		Payload   string `json:"payload"`
		Quantity  int    `json:"quantity"`
	}{
		fmt.Sprintf("shop:%d:order:%d", connectionID, order.ID), f.Payload, order.Items[0].Quantity,
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, errors.New("encode redemption delivery failed")
	}
	req, err := http.NewRequest(http.MethodPost, c.origin+"/api/integration/deliver", bytes.NewReader(encoded))
	if err != nil {
		return nil, errors.New("create redemption delivery request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("redemption issuer unavailable; retry the same delivery reference")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("redemption issuer rejected delivery; preserve source and retry only the same reference")
	}
	var result struct {
		OK   bool             `json:"ok"`
		Data deliveryResponse `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil || !result.OK {
		return nil, errors.New("incomplete redemption delivery response")
	}
	payload, err := result.Data.customerPayload(request.Quantity, c.origin)
	if err != nil {
		return nil, err
	}
	copy := *f
	copy.Payload = payload
	copy.InstructionsOverride = customerInstructions(c.origin)
	return &copy, nil
}
