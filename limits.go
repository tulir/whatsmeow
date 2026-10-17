// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mau.fi/whatsmeow/types"
)

const (
	queryNewChatMessageCappingInfo = "24503548349331633"
	queryAccountReachoutTimelock   = "23983697327930364"
)

type respGetNewChatMessageCappingInfo struct {
	MessageCappingInfo *types.NewChatMessageCappingInfo `json:"xwa2_message_capping_info"`
}

type respGetAccountReachoutTimelock struct {
	ReachoutTimelock *types.AccountReachoutTimelock `json:"xwa2_fetch_account_reachout_timelock"`
}

// GetNewChatMessageCappingInfo fetches raw MEX capping info for caller-invoked
func (cli *Client) GetNewChatMessageCappingInfo(ctx context.Context) (*types.NewChatMessageCappingInfo, error) {
	data, err := cli.sendMexIQ(ctx, queryNewChatMessageCappingInfo, map[string]any{
		"input": map[string]any{
			"type": "INDIVIDUAL_NEW_CHAT_MSG",
		},
	})
	var respData respGetNewChatMessageCappingInfo
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		} else if err == nil && respData.MessageCappingInfo == nil {
			err = fmt.Errorf("mex unexpected null response for new chat message capping info")
		}
	}
	return respData.MessageCappingInfo, err
}

// GetAccountReachoutTimelock fetches raw MEX reachout timelock info
func (cli *Client) GetAccountReachoutTimelock(ctx context.Context) (*types.AccountReachoutTimelock, error) {
	data, err := cli.sendMexIQ(ctx, queryAccountReachoutTimelock, map[string]any{})
	var respData respGetAccountReachoutTimelock
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		} else if err == nil && respData.ReachoutTimelock == nil {
			err = fmt.Errorf("mex unexpected null response for fetching reachout timelock")
		}
	}
	return respData.ReachoutTimelock, err
}
