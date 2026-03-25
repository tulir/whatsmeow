// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

type NewChatMessageCappingInfo struct {
	TotalQuota          int    `json:"total_quota"`
	UsedQuota           int    `json:"used_quota"`
	CycleStartTimestamp string `json:"cycle_start_timestamp"`
	CycleEndTimestamp   string `json:"cycle_end_timestamp"`
	ServerSentTimestamp string `json:"server_sent_timestamp"`
	OTEStatus           string `json:"ote_status"`
	MVStatus            string `json:"mv_status"`
	CappingStatus       string `json:"capping_status"`
}

type AccountReachoutTimelock struct {
	IsActive            bool   `json:"is_active"`
	TimeEnforcementEnds string `json:"time_enforcement_ends"`
	EnforcementType     string `json:"enforcement_type"`
}
