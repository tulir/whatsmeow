// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package whatsmeow implements a client for interacting with the WhatsApp web multidevice API.
package whatsmeow

import (
	"encoding/xml"
	"bytes"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)


type OrderDetailType struct {
	Order struct {
		CreationTs string `xml:"creation_ts,attr"`
		ID         string `xml:"id,attr"`
		Product    []struct {
			ID         string `xml:"id"`
			RetailerID string `xml:"retailer_id"`
			ImageUrl   string `xml:"image>url"`
			Price    string `xml:"price"`
			Currency string `xml:"currency"`
			Name     string `xml:"name"`
			Quantity string `xml:"quantity"`
		} `xml:"product"`
		Catalog struct {
			ID   string `xml:"id"`
		} `xml:"catalog"`
		Price struct {
			Subtotal    string `xml:"subtotal"`
			Currency    string `xml:"currency"`
			Total       string `xml:"total"`
			PriceStatus string `xml:"price_status"`
		} `xml:"price"`
	} `xml:"order"`
} 

func (cli *Client) GetOrderDetails(orderId, tokenBase64 string) (*OrderDetailType, error) {

	detailsNode, err := cli.sendIQ(infoQuery{
		Namespace: "fb:thrift_iq",
		Type:      "get",
		To:        types.ServerJID,
		SmaxId:    "5",
		Content: []waBinary.Node{
			{
				Tag: "order",
				Attrs: waBinary.Attrs{
					"op": "get",
					"id": orderId,
				},
				Content: []waBinary.Node{
					{
						Tag:   "image_dimensions",
						Attrs: nil,
						Content: []waBinary.Node{
							{
								Tag:     "width",
								Attrs:   nil,
								Content: []byte("100"),
							}, {
								Tag:     "height",
								Attrs:   nil,
								Content: []byte("100"),
							},
						},
					},
					{
						Tag:     "token",
						Attrs:   nil,
						Content: []byte(tokenBase64),
					},
				},
			},
		},
	})
	
	OrderDetail := &OrderDetailType{}
	d := xml.NewDecoder(bytes.NewReader([]byte(detailsNode.XMLString())))
	d.Strict = false
	err = d.Decode(&OrderDetail)
	if err != nil {
		cli.Log.Infof("Order Details response Error: %v",err)
	}

	return OrderDetail, err
}
