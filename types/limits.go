// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import "go.mau.fi/util/jsontime"

type NewChatMessageCappingOTEStatus string

const (
	NewChatMessageCappingOTEStatusNotEligible          NewChatMessageCappingOTEStatus = "NOT_ELIGIBLE"
	NewChatMessageCappingOTEStatusEligible             NewChatMessageCappingOTEStatus = "ELIGIBLE"
	NewChatMessageCappingOTEStatusActiveInCurrentCycle NewChatMessageCappingOTEStatus = "ACTIVE_IN_CURRENT_CYCLE"
	NewChatMessageCappingOTEStatusExhausted            NewChatMessageCappingOTEStatus = "EXHAUSTED"
)

type NewChatMessageCappingMVStatus string

const (
	NewChatMessageCappingMVStatusNotEligible            NewChatMessageCappingMVStatus = "NOT_ELIGIBLE"
	NewChatMessageCappingMVStatusNotActive              NewChatMessageCappingMVStatus = "NOT_ACTIVE"
	NewChatMessageCappingMVStatusActive                 NewChatMessageCappingMVStatus = "ACTIVE"
	NewChatMessageCappingMVStatusActiveUpgradeAvailable NewChatMessageCappingMVStatus = "ACTIVE_UPGRADE_AVAILABLE"
)

type NewChatMessageCappingStatus string

const (
	NewChatMessageCappingStatusNone          NewChatMessageCappingStatus = "NONE"
	NewChatMessageCappingStatusFirstWarning  NewChatMessageCappingStatus = "FIRST_WARNING"
	NewChatMessageCappingStatusSecondWarning NewChatMessageCappingStatus = "SECOND_WARNING"
	NewChatMessageCappingStatusCapped        NewChatMessageCappingStatus = "CAPPED"
)

type ReachoutTimelockEnforcementType string

const (
	ReachoutTimelockEnforcementTypeDefault                                     ReachoutTimelockEnforcementType = "DEFAULT"
	ReachoutTimelockEnforcementTypeBizQuality                                  ReachoutTimelockEnforcementType = "BIZ_QUALITY"
	ReachoutTimelockEnforcementTypeBizCommerceViolationAdult                   ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ADULT"
	ReachoutTimelockEnforcementTypeBizCommerceViolationAlcohol                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ALCOHOL"
	ReachoutTimelockEnforcementTypeBizCommerceViolationAnimals                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ANIMALS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationBodyPartsFluids         ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_BODY_PARTS_FLUIDS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDating                  ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DATING"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDigitalServicesProducts ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DIGITAL_SERVICES_PRODUCTS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDrugs                   ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DRUGS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDrugsOnlyOTC            ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DRUGS_ONLY_OTC"
	ReachoutTimelockEnforcementTypeBizCommerceViolationGambling                ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_GAMBLING"
	ReachoutTimelockEnforcementTypeBizCommerceViolationHealthcare              ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_HEALTHCARE"
	ReachoutTimelockEnforcementTypeBizCommerceViolationRealFakeCurrency        ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_REAL_FAKE_CURRENCY"
	ReachoutTimelockEnforcementTypeBizCommerceViolationSupplements             ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_SUPPLEMENTS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationTobacco                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_TOBACCO"
	ReachoutTimelockEnforcementTypeBizCommerceViolationViolentContent          ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_VIOLENT_CONTENT"
	ReachoutTimelockEnforcementTypeBizCommerceViolationWeapons                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_WEAPONS"
	ReachoutTimelockEnforcementTypeWebCompanionOnly                            ReachoutTimelockEnforcementType = "WEB_COMPANION_ONLY"
)

type NewChatMessageCappingInfo struct {
	TotalQuota          int                            `json:"total_quota"`
	UsedQuota           int                            `json:"used_quota"`
	CycleStartTimestamp jsontime.UnixString            `json:"cycle_start_timestamp"`
	CycleEndTimestamp   jsontime.UnixString            `json:"cycle_end_timestamp"`
	ServerSentTimestamp jsontime.UnixString            `json:"server_sent_timestamp"`
	OTEStatus           NewChatMessageCappingOTEStatus `json:"ote_status"`
	MVStatus            NewChatMessageCappingMVStatus  `json:"mv_status"`
	CappingStatus       NewChatMessageCappingStatus    `json:"capping_status"`
}

type AccountReachoutTimelock struct {
	IsActive            bool                            `json:"is_active"`
	TimeEnforcementEnds jsontime.UnixString             `json:"time_enforcement_ends"`
	EnforcementType     ReachoutTimelockEnforcementType `json:"enforcement_type"`
}
