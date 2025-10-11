package types

import "time"

type OrderDetails struct {
	ID        string         `json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	CatalogID string         `json:"catalog_id,omitempty"`
	Price     OrderPrice     `json:"price"`
	Products  []OrderProduct `json:"products"`
}

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

type VariantInfo struct {
	Properties string `json:"properties,omitempty"`
}

type OrderPrice struct {
	Subtotal    int64  `json:"subtotal"`
	Total       int64  `json:"total"`
	Currency    string `json:"currency"`
	PriceStatus string `json:"price_status,omitempty"`
}
