// Copyright (c) 2026 Zhang Haoran
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types"
)

const mutationGroupSuspensionAppeal = "25946115325088226"

// ReqGroupSuspensionAppeal contains the optional parameters for SubmitGroupSuspensionAppeal.
type ReqGroupSuspensionAppeal struct {
	Reason string
	// DebugInfo is a JSON-encoded string. An empty value defaults to "{}".
	DebugInfo string
}

// SubmitGroupSuspensionAppeal submits an appeal for a suspended group.
// A nil error means the appeal was submitted or already exists, not that the group was unsuspended.
// If the server rejects the appeal, both the response and an error are returned.
func (cli *Client) SubmitGroupSuspensionAppeal(ctx context.Context, jid types.JID, req ReqGroupSuspensionAppeal) (*types.GroupSuspensionAppealResult, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	if jid.Server != types.GroupServer || jid.User == "" {
		return nil, fmt.Errorf("invalid group JID for suspension appeal: %s", jid)
	}
	input := map[string]any{
		"group_jid":  jid.User,
		"debug_info": cmp.Or(req.DebugInfo, "{}"),
	}
	if req.Reason != "" {
		input["appeal_reason"] = req.Reason
	}
	data, err := cli.sendMexIQ(ctx, mutationGroupSuspensionAppeal, map[string]any{"input": input})
	if err != nil {
		return nil, fmt.Errorf("failed to submit group suspension appeal: %w", err)
	}
	var response struct {
		Appeal *struct {
			ResponseCode       string      `json:"response_code"`
			ErrorMessage       string      `json:"error_message"`
			AppealCreationTime json.Number `json:"appeal_creation_time"`
		} `json:"wa_create_group_suspension_appeal"`
	}
	if err = json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal group suspension appeal response: %w", err)
	}
	if response.Appeal == nil || response.Appeal.ResponseCode == "" {
		return nil, fmt.Errorf("missing response code in group suspension appeal response")
	}
	result := &types.GroupSuspensionAppealResult{
		ResponseCode: response.Appeal.ResponseCode,
		ErrorMessage: response.Appeal.ErrorMessage,
	}
	if response.Appeal.AppealCreationTime != "" {
		creationTime, parseErr := response.Appeal.AppealCreationTime.Int64()
		if parseErr != nil {
			return nil, fmt.Errorf("invalid group suspension appeal creation time: %w", parseErr)
		}
		if creationTime != 0 {
			result.AppealCreationTime = time.Unix(creationTime, 0)
		}
	}
	switch result.ResponseCode {
	case "SUCCESS", "APPEAL_ALREADY_EXISTS":
		return result, nil
	default:
		return result, fmt.Errorf("group suspension appeal failed (%s): %s", result.ResponseCode, result.ErrorMessage)
	}
}
