// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import "time"

// OrderDetails contains the metadata, price, and products in a WhatsApp order.
type OrderDetails struct {
	ID        string         `json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	CatalogID string         `json:"catalog_id,omitempty"`
	Price     OrderPrice     `json:"price"`
	Products  []OrderProduct `json:"products"`
}

// OrderProduct contains a single product entry in a WhatsApp order.
type OrderProduct struct {
	ID          string      `json:"id"`
	ImageID     string      `json:"image_id,omitempty"`
	ImageURL    string      `json:"image_url,omitempty"`
	Price       int64       `json:"price"`
	Currency    string      `json:"currency"`
	Name        string      `json:"name"`
	Quantity    int         `json:"quantity"`
	VariantInfo VariantInfo `json:"variant_info,omitempty"`
}

// VariantInfo contains selected variant metadata for an order product.
type VariantInfo struct {
	Properties string `json:"properties,omitempty"`
}

// OrderPrice contains the subtotal and total price metadata for a WhatsApp order.
type OrderPrice struct {
	Subtotal    int64  `json:"subtotal"`
	Total       int64  `json:"total"`
	Currency    string `json:"currency"`
	PriceStatus string `json:"price_status,omitempty"`
}
