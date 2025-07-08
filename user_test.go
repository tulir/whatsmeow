// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestParseBusinessProfile(t *testing.T) {
	// Manually construct the business profile node that was causing the panic in issue #866
	expectedJID, _ := types.ParseJID("628112640770@s.whatsapp.net")

	// Create business hours configs that match the problematic XML
	businessHoursConfigs := []waBinary.Node{
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "sun",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "mon",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "tue",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "wed",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "thu",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "fri",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
		{
			Tag: "business_hours_config",
			Attrs: waBinary.Attrs{
				"close_time":  "1200",
				"day_of_week": "sat",
				"mode":        "specific_hours",
				"open_time":   "600",
			},
		},
	}

	// Create profile options that have nested elements (this was part of the issue)
	profileOptionsChildren := []waBinary.Node{
		{
			Tag:     "commerce_experience",
			Content: []byte("catalog"),
		},
		{
			Tag:     "cart_enabled",
			Content: []byte("true"),
		},
		{
			Tag:     "direct_connection",
			Content: []byte("false"),
		},
		{
			Tag: "bot_fields",
			Content: []waBinary.Node{
				{
					Tag:     "is_typing_indicator_enabled",
					Content: []byte("false"),
				},
			},
		},
	}

	businessProfileNode := waBinary.Node{
		Tag: "business_profile",
		Content: []waBinary.Node{
			{
				Tag: "profile",
				Attrs: waBinary.Attrs{
					"jid": expectedJID,
					"tag": "125605989",
				},
				Content: []waBinary.Node{
					{
						Tag:     "address",
						Content: []byte("Jl. Pandanaran No.252d, Recosari, Banaran, Kec. Boyolali, Kabupaten Boyolali, Jawa Tengah 57313, Indonesia.                                                           https://maps.app.goo.gl/mz4BLgpREVfvEw8o9"),
					},
					{
						Tag:     "email",
						Content: []byte("doozboyolali@gmail.com"),
					},
					{
						Tag: "business_hours",
						Attrs: waBinary.Attrs{
							"timezone": "Asia/Jakarta",
						},
						Content: businessHoursConfigs,
					},
					{
						Tag: "categories",
						Content: []waBinary.Node{
							{
								Tag: "category",
								Attrs: waBinary.Attrs{
									"id": "145118935550090",
								},
								Content: []byte("Medical & health"),
							},
						},
					},
					{
						Tag:     "profile_options",
						Content: profileOptionsChildren,
					},
				},
			},
		},
	}

	// Create a mock client for testing
	cli := &Client{}

	// This should not panic anymore
	profile, err := cli.parseBusinessProfile(&businessProfileNode)
	if err != nil {
		t.Fatalf("parseBusinessProfile failed: %v", err)
	}

	// Verify the parsed data
	if profile.JID != expectedJID {
		t.Errorf("Expected JID %v, got %v", expectedJID, profile.JID)
	}

	if profile.Email != "doozboyolali@gmail.com" {
		t.Errorf("Expected email 'doozboyolali@gmail.com', got '%s'", profile.Email)
	}

	expectedAddress := "Jl. Pandanaran No.252d, Recosari, Banaran, Kec. Boyolali, Kabupaten Boyolali, Jawa Tengah 57313, Indonesia.                                                           https://maps.app.goo.gl/mz4BLgpREVfvEw8o9"
	if profile.Address != expectedAddress {
		t.Errorf("Address mismatch")
	}

	if profile.BusinessHoursTimeZone != "Asia/Jakarta" {
		t.Errorf("Expected timezone 'Asia/Jakarta', got '%s'", profile.BusinessHoursTimeZone)
	}

	// Check business hours - should have 7 entries (one for each day)
	if len(profile.BusinessHours) != 7 {
		t.Errorf("Expected 7 business hours configs, got %d", len(profile.BusinessHours))
	}

	// Check first business hour config
	if len(profile.BusinessHours) > 0 {
		firstConfig := profile.BusinessHours[0]
		if firstConfig.DayOfWeek != "sun" {
			t.Errorf("Expected first day 'sun', got '%s'", firstConfig.DayOfWeek)
		}
		if firstConfig.Mode != "specific_hours" {
			t.Errorf("Expected mode 'specific_hours', got '%s'", firstConfig.Mode)
		}
		if firstConfig.OpenTime != "600" {
			t.Errorf("Expected open time '600', got '%s'", firstConfig.OpenTime)
		}
		if firstConfig.CloseTime != "1200" {
			t.Errorf("Expected close time '1200', got '%s'", firstConfig.CloseTime)
		}
	}

	// Check categories
	if len(profile.Categories) != 1 {
		t.Errorf("Expected 1 category, got %d", len(profile.Categories))
	} else {
		category := profile.Categories[0]
		if category.ID != "145118935550090" {
			t.Errorf("Expected category ID '145118935550090', got '%s'", category.ID)
		}
		if category.Name != "Medical & health" {
			t.Errorf("Expected category name 'Medical & health', got '%s'", category.Name)
		}
	}

	// Check profile options
	if len(profile.ProfileOptions) == 0 {
		t.Error("Expected profile options to be populated")
	}

	// Check some specific profile options
	if profile.ProfileOptions["commerce_experience"] != "catalog" {
		t.Errorf("Expected commerce_experience 'catalog', got '%s'", profile.ProfileOptions["commerce_experience"])
	}

	if profile.ProfileOptions["cart_enabled"] != "true" {
		t.Errorf("Expected cart_enabled 'true', got '%s'", profile.ProfileOptions["cart_enabled"])
	}

	if profile.ProfileOptions["direct_connection"] != "false" {
		t.Errorf("Expected direct_connection 'false', got '%s'", profile.ProfileOptions["direct_connection"])
	}
}

func TestParseBusinessProfileEmptyFields(t *testing.T) {
	// Test with empty/missing fields to ensure we handle them gracefully
	expectedJID, _ := types.ParseJID("test@s.whatsapp.net")

	businessProfileNode := waBinary.Node{
		Tag: "business_profile",
		Content: []waBinary.Node{
			{
				Tag: "profile",
				Attrs: waBinary.Attrs{
					"jid": expectedJID,
				},
				Content: []waBinary.Node{
					{
						Tag:     "address",
						Content: []byte(""),
					},
					{
						Tag:     "email",
						Content: []byte(""),
					},
					{
						Tag: "business_hours",
						Attrs: waBinary.Attrs{
							"timezone": "",
						},
						Content: []waBinary.Node{},
					},
					{
						Tag:     "categories",
						Content: []waBinary.Node{},
					},
					{
						Tag:     "profile_options",
						Content: []waBinary.Node{},
					},
				},
			},
		},
	}

	cli := &Client{}
	profile, err := cli.parseBusinessProfile(&businessProfileNode)
	if err != nil {
		t.Fatalf("parseBusinessProfile failed: %v", err)
	}

	// Should handle empty fields gracefully
	if profile.JID != expectedJID {
		t.Errorf("Expected JID %v, got %v", expectedJID, profile.JID)
	}

	if profile.Email != "" {
		t.Errorf("Expected empty email, got '%s'", profile.Email)
	}

	if profile.Address != "" {
		t.Errorf("Expected empty address, got '%s'", profile.Address)
	}

	if len(profile.Categories) != 0 {
		t.Errorf("Expected 0 categories, got %d", len(profile.Categories))
	}

	if len(profile.BusinessHours) != 0 {
		t.Errorf("Expected 0 business hours, got %d", len(profile.BusinessHours))
	}

	if len(profile.ProfileOptions) != 0 {
		t.Errorf("Expected 0 profile options, got %d", len(profile.ProfileOptions))
	}
}

// TestParseBusinessProfileFromIssue866XML tests parsing business profile from a structure
// that exactly matches the XML response from issue #866. This test creates waBinary.Node
// structures that mirror what would be received when WhatsApp sends the problematic
// XML response that caused the panic.
func TestParseBusinessProfileFromIssue866XML(t *testing.T) {
	expectedJID, _ := types.ParseJID("628112640770@s.whatsapp.net")

	// Create the exact structure that would result from parsing the XML in issue #866
	// This simulates what binary.Unmarshal would produce from the problematic XML response
	businessProfileNode := waBinary.Node{
		Tag: "business_profile",
		Content: []waBinary.Node{
			{
				Tag: "profile",
				Attrs: waBinary.Attrs{
					"jid": expectedJID,
					"tag": "125605989",
				},
				Content: []waBinary.Node{
					{
						Tag:     "address",
						Content: []byte("Jl. Pandanaran No.252d, Recosari, Banaran, Kec. Boyolali, Kabupaten Boyolali, Jawa Tengah 57313, Indonesia.                                                           https://maps.app.goo.gl/mz4BLgpREVfvEw8o9"),
					},
					{
						Tag:     "description",
						Content: []byte("<!-- 493 bytes -->"), // Simulating the comment-style content
					},
					{
						Tag:     "email",
						Content: []byte("doozboyolali@gmail.com"),
					},
					{
						Tag:     "website",
						Content: []byte("https://www.instagram.com/doozoptik?utm_source=qr&igsh=ZDR1M3IxbTVqYnpk"),
					},
					{
						Tag:     "latitude",
						Content: []byte("-7.5478"),
					},
					{
						Tag:     "longitude",
						Content: []byte("110.5805"),
					},
					{
						Tag: "business_hours",
						Attrs: waBinary.Attrs{
							"timezone": "Asia/Jakarta",
						},
						Content: []waBinary.Node{
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "sun",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "mon",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "tue",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "wed",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "thu",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "fri",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"close_time":  "1200",
									"day_of_week": "sat",
									"mode":        "specific_hours",
									"open_time":   "600",
								},
							},
						},
					},
					{
						Tag: "categories",
						Content: []waBinary.Node{
							{
								Tag: "category",
								Attrs: waBinary.Attrs{
									"id": "145118935550090",
								},
								Content: []byte("Medical & health"),
							},
						},
					},
					{
						Tag: "profile_options",
						Content: []waBinary.Node{
							{
								Tag:     "commerce_experience",
								Content: []byte("catalog"),
							},
							{
								Tag:     "cart_enabled",
								Content: []byte("true"),
							},
							{
								Tag:     "direct_connection",
								Content: []byte("false"),
							},
							{
								Tag: "bot_fields",
								Content: []waBinary.Node{
									{
										Tag:     "is_typing_indicator_enabled",
										Content: []byte("false"),
									},
								},
							},
						},
					},
					{
						Tag: "direct_connection",
						Attrs: waBinary.Attrs{
							"enabled": "false",
						},
						Content: []waBinary.Node{
							{
								Tag: "features",
								Attrs: waBinary.Attrs{
									"name": "default",
								},
							},
						},
					},
					{
						Tag:     "member_since_text",
						Content: []byte("Joined in February, 2022"),
					},
					{
						Tag:     "automated_type",
						Content: []byte("unknown"),
					},
					{
						Tag: "biz_identity_info",
						Attrs: waBinary.Attrs{
							"actual_actors": "self",
							"display_name":  "DOOZ OPTIK BOYOLALI",
							"host_storage":  "on_premise",
							"is_signed":     "true",
							"phone_number":  "",
							"revoked":       "false",
							"serial":        "3880641614313507344",
							"type":          "smb",
							"vlevel":        "unknown",
						},
					},
				},
			},
		},
	}

	// Create a mock client for testing
	cli := &Client{}

	// This should not panic - the fix should handle the mixed content types
	profile, err := cli.parseBusinessProfile(&businessProfileNode)
	if err != nil {
		t.Fatalf("parseBusinessProfile failed with XML structure from issue #866: %v", err)
	}

	// Verify the parsed data matches expected values from the XML
	if profile.JID != expectedJID {
		t.Errorf("Expected JID %v, got %v", expectedJID, profile.JID)
	}

	if profile.Email != "doozboyolali@gmail.com" {
		t.Errorf("Expected email 'doozboyolali@gmail.com', got '%s'", profile.Email)
	}

	expectedAddress := "Jl. Pandanaran No.252d, Recosari, Banaran, Kec. Boyolali, Kabupaten Boyolali, Jawa Tengah 57313, Indonesia.                                                           https://maps.app.goo.gl/mz4BLgpREVfvEw8o9"
	if profile.Address != expectedAddress {
		t.Errorf("Address mismatch. Expected: %s, Got: %s", expectedAddress, profile.Address)
	}

	if profile.BusinessHoursTimeZone != "Asia/Jakarta" {
		t.Errorf("Expected timezone 'Asia/Jakarta', got '%s'", profile.BusinessHoursTimeZone)
	}

	// Check business hours - should have 7 entries (one for each day)
	if len(profile.BusinessHours) != 7 {
		t.Errorf("Expected 7 business hours configs, got %d", len(profile.BusinessHours))
	}

	// Check categories
	if len(profile.Categories) != 1 {
		t.Errorf("Expected 1 category, got %d", len(profile.Categories))
	} else {
		category := profile.Categories[0]
		if category.ID != "145118935550090" {
			t.Errorf("Expected category ID '145118935550090', got '%s'", category.ID)
		}
		if category.Name != "Medical & health" {
			t.Errorf("Expected category name 'Medical & health', got '%s'", category.Name)
		}
	}

	// Check profile options - verify nested bot_fields are handled correctly
	if len(profile.ProfileOptions) == 0 {
		t.Error("Expected profile options to be populated")
	}

	// Check main profile options
	if profile.ProfileOptions["commerce_experience"] != "catalog" {
		t.Errorf("Expected commerce_experience 'catalog', got '%s'", profile.ProfileOptions["commerce_experience"])
	}

	if profile.ProfileOptions["cart_enabled"] != "true" {
		t.Errorf("Expected cart_enabled 'true', got '%s'", profile.ProfileOptions["cart_enabled"])
	}

	if profile.ProfileOptions["direct_connection"] != "false" {
		t.Errorf("Expected direct_connection 'false', got '%s'", profile.ProfileOptions["direct_connection"])
	}

	// The bot_fields should be included but with empty value since it contains child nodes, not byte content
	// This was part of the original issue - nested elements should be handled safely
	if value, exists := profile.ProfileOptions["bot_fields"]; !exists {
		t.Error("bot_fields should be included in profile options")
	} else if value != "" {
		t.Errorf("bot_fields should have empty value since it contains child nodes, got '%s'", value)
	}
}

// TestParseBusinessProfileMixedContent tests the specific issue from #866
// where some XML elements contain child nodes instead of byte content
func TestParseBusinessProfileMixedContent(t *testing.T) {
	expectedJID, _ := types.ParseJID("test@s.whatsapp.net")

	// This simulates the issue where profile_options contains child nodes instead of just bytes
	businessProfileNode := waBinary.Node{
		Tag: "business_profile",
		Content: []waBinary.Node{
			{
				Tag: "profile",
				Attrs: waBinary.Attrs{
					"jid": expectedJID,
				},
				Content: []waBinary.Node{
					{
						Tag: "address",
						// This has child nodes instead of byte content (the problematic case)
						Content: []waBinary.Node{
							{
								Tag:     "street",
								Content: []byte("123 Main St"),
							},
							{
								Tag:     "city",
								Content: []byte("Test City"),
							},
						},
					},
					{
						Tag:     "email",
						Content: []byte("test@example.com"),
					},
					{
						Tag: "business_hours",
						Attrs: waBinary.Attrs{
							"timezone": "UTC",
						},
						Content: []waBinary.Node{},
					},
					{
						Tag:     "categories",
						Content: []waBinary.Node{},
					},
					{
						Tag: "profile_options",
						// This has deeply nested child nodes (another problematic case)
						Content: []waBinary.Node{
							{
								Tag:     "commerce_experience",
								Content: []byte("catalog"),
							},
							{
								Tag: "bot_fields",
								Content: []waBinary.Node{
									{
										Tag:     "is_typing_indicator_enabled",
										Content: []byte("false"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cli := &Client{}
	profile, err := cli.parseBusinessProfile(&businessProfileNode)
	if err != nil {
		t.Fatalf("parseBusinessProfile failed: %v", err)
	}

	// Should handle mixed content gracefully
	if profile.JID != expectedJID {
		t.Errorf("Expected JID %v, got %v", expectedJID, profile.JID)
	}

	if profile.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", profile.Email)
	}

	// Address should be empty since it contains child nodes, not byte content
	if profile.Address != "" {
		t.Errorf("Expected empty address (due to child nodes), got '%s'", profile.Address)
	}

	// Should have two profile options (commerce_experience and bot_fields)
	if len(profile.ProfileOptions) != 2 {
		t.Errorf("Expected 2 profile options, got %d", len(profile.ProfileOptions))
	}

	if profile.ProfileOptions["commerce_experience"] != "catalog" {
		t.Errorf("Expected commerce_experience 'catalog', got '%s'", profile.ProfileOptions["commerce_experience"])
	}
}
