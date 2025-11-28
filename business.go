// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// GetOrderDetails fetches the details of a specific order using its ID and token.
// Both token and orderID is found in the OrderMessage.
func (cli *Client) GetOrderDetails(ctx context.Context, orderID, tokenBase64 string) (*types.OrderDetails, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "fb:thrift_iq",
		Type:      iqGet,
		SMaxID:    "5",
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "order",
			Attrs: waBinary.Attrs{
				"op": "get",
				"id": orderID,
			},
			Content: []waBinary.Node{
				{
					Tag: "image_dimensions",
					Content: []waBinary.Node{
						{Tag: "width", Content: []byte("100")},
						{Tag: "height", Content: []byte("100")},
					},
				},
				{Tag: "token", Content: []byte(tokenBase64)},
			},
		}},
		Context: ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send order IQ: %w", err)
	}

	orderNode, ok := resp.GetOptionalChildByTag("order")
	if !ok {
		return nil, &ElementMissingError{Tag: "order", In: "response to order query"}
	}

	return parseOrderDetailsNode(orderNode)
}

// Helper to get the string content of a child node.
func getStringChild(node waBinary.Node, tag string) string {
	child, ok := node.GetOptionalChildByTag(tag)
	if !ok {
		return ""
	}
	content, _ := child.Content.([]byte)
	return string(content)
}

func parseOrderDetailsNode(orderNode waBinary.Node) (*types.OrderDetails, error) {
	ag := orderNode.AttrGetter()
	details := &types.OrderDetails{
		ID:        ag.String("id"),
		CreatedAt: ag.UnixTime("creation_ts"),
	}
	if err := ag.Error(); err != nil {
		return nil, err
	}

	// Parse Price
	priceNode, ok := orderNode.GetOptionalChildByTag("price")
	if ok {
		subtotal, _ := strconv.ParseInt(getStringChild(priceNode, "subtotal"), 10, 64)
		total, _ := strconv.ParseInt(getStringChild(priceNode, "total"), 10, 64)
		details.Price = types.OrderPrice{
			Subtotal:    subtotal,
			Total:       total,
			Currency:    getStringChild(priceNode, "currency"),
			PriceStatus: getStringChild(priceNode, "price_status"),
		}
	}

	// Parse Catalog ID
	catalogNode, ok := orderNode.GetOptionalChildByTag("catalog")
	if ok {
		details.CatalogID = getStringChild(catalogNode, "id")
	}

	// Parse Products
	for _, productNode := range orderNode.GetChildrenByTag("product") {
		price, _ := strconv.ParseInt(getStringChild(productNode, "price"), 10, 64)
		quantity, _ := strconv.Atoi(getStringChild(productNode, "quantity"))

		product := types.OrderProduct{
			ID:       getStringChild(productNode, "id"),
			Price:    price,
			Currency: getStringChild(productNode, "currency"),
			Name:     getStringChild(productNode, "name"),
			Quantity: quantity,
		}

		// Parse Product Image
		if imageNode, ok := productNode.GetOptionalChildByTag("image"); ok {
			product.ImageID = getStringChild(imageNode, "id")
			product.ImageURL = getStringChild(imageNode, "url")
		}

		// Parse Variant Info
		if variantNode, ok := productNode.GetOptionalChildByTag("variant_info"); ok {
			product.VariantInfo.Properties = getStringChild(variantNode, "properties")
		}

		details.Products = append(details.Products, product)
	}

	return details, nil
}
