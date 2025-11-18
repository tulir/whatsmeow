// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var (
	// ErrNoMessagesInRange is returned when no messages are found in the specified date range
	ErrNoMessagesInRange = errors.New("no messages found in the specified date range")

	// ErrInvalidDateRange is returned when the start time is after the end time
	ErrInvalidDateRange = errors.New("start time must be before end time")

	// ErrHistorySyncTimeout is returned when waiting for history sync response times out
	ErrHistorySyncTimeout = errors.New("timeout waiting for history sync response")
)

// DateRangeQuery contains parameters for querying messages within a date range.
type DateRangeQuery struct {
	// ChatJID is the JID of the chat/contact to retrieve messages from (required)
	ChatJID types.JID

	// StartTime is the start of the date range (inclusive)
	StartTime time.Time

	// EndTime is the end of the date range (inclusive)
	EndTime time.Time

	// AnchorMessage is an optional message to use as the starting point for pagination.
	// If nil, the function will attempt to start from the most recent messages.
	// The anchor message's timestamp should be at or after EndTime for best results.
	AnchorMessage *types.MessageInfo

	// MaxMessages limits the number of messages to retrieve (0 = no limit)
	// This is useful to prevent retrieving too many messages at once.
	MaxMessages int

	// Timeout specifies how long to wait for history sync responses
	// Default is 30 seconds if not specified
	Timeout time.Duration
}

// GetMessagesInDateRange retrieves messages from a specific chat within a date range.
//
// This function sends history sync requests to WhatsApp and filters messages by timestamp.
// Since WhatsApp's API uses pagination based on message IDs and counts rather than dates,
// this function may need to retrieve multiple batches of messages to cover the entire range.
//
// Example usage:
//
//	// Get messages from the last week
//	query := whatsmeow.DateRangeQuery{
//		ChatJID:   chatJID,
//		StartTime: time.Now().AddDate(0, 0, -7),
//		EndTime:   time.Now(),
//		MaxMessages: 1000,
//	}
//	messages, err := client.GetMessagesInDateRange(context.Background(), query)
//	if err != nil {
//		log.Printf("Error retrieving messages: %v", err)
//		return
//	}
//	for _, msg := range messages {
//		log.Printf("Message from %s at %s: %v", msg.Info.Sender, msg.Info.Timestamp, msg.Message)
//	}
//
// Note: This function requires an anchor message to start pagination. If you don't have
// a recent message, you may need to listen for incoming messages first or use
// GetMessagesInDateRangeFromRecent which attempts to use a synthetic anchor.
func (cli *Client) GetMessagesInDateRange(ctx context.Context, query DateRangeQuery) ([]*events.Message, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}

	// Validate parameters
	if query.ChatJID.IsEmpty() {
		return nil, fmt.Errorf("chat JID is required")
	}

	if query.StartTime.IsZero() || query.EndTime.IsZero() {
		return nil, fmt.Errorf("start time and end time are required")
	}

	if query.StartTime.After(query.EndTime) {
		return nil, ErrInvalidDateRange
	}

	if query.Timeout == 0 {
		query.Timeout = 30 * time.Second
	}

	// If no anchor message provided, create a synthetic one at the end time
	anchorMsg := query.AnchorMessage
	if anchorMsg == nil {
		// Create a synthetic anchor message at the end time
		// This will be used to request messages before this time
		anchorMsg = &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     query.ChatJID,
				IsFromMe: false,
			},
			ID:        cli.GenerateMessageID(), // Generate a synthetic ID
			Timestamp: query.EndTime,
		}
	}

	var allMessages []*events.Message
	messagesRetrieved := 0
	continueRetrieval := true

	for continueRetrieval {
		// Check if we've reached the message limit
		if query.MaxMessages > 0 && messagesRetrieved >= query.MaxMessages {
			break
		}

		// Calculate how many messages to request in this batch
		batchSize := 50 // WhatsApp's recommended batch size
		if query.MaxMessages > 0 {
			remaining := query.MaxMessages - messagesRetrieved
			if remaining < batchSize {
				batchSize = remaining
			}
		}

		// Request history sync
		batch, oldestMsg, err := cli.requestHistoryBatch(ctx, anchorMsg, batchSize, query.Timeout)
		if err != nil {
			return allMessages, fmt.Errorf("failed to retrieve history batch: %w", err)
		}

		// If no messages returned, we've reached the end
		if len(batch) == 0 {
			break
		}

		// Filter messages by date range and add to results
		foundInRange := false
		for _, msg := range batch {
			// Check if message is before start time - if so, stop retrieval
			if msg.Info.Timestamp.Before(query.StartTime) {
				continueRetrieval = false
				break
			}

			// Check if message is within range
			if !msg.Info.Timestamp.Before(query.StartTime) && !msg.Info.Timestamp.After(query.EndTime) {
				allMessages = append(allMessages, msg)
				messagesRetrieved++
				foundInRange = true
			}
		}

		// If we didn't find any messages in range in this batch,
		// and the oldest message is before start time, stop
		if !foundInRange && oldestMsg != nil && oldestMsg.Timestamp.Before(query.StartTime) {
			break
		}

		// Update anchor for next batch
		if oldestMsg != nil {
			anchorMsg = oldestMsg
		} else {
			// No more messages available
			break
		}
	}

	return allMessages, nil
}

// requestHistoryBatch requests a single batch of messages from WhatsApp.
// Returns the messages, the oldest message info for pagination, and any error.
func (cli *Client) requestHistoryBatch(ctx context.Context, anchorMsg *types.MessageInfo, count int, timeout time.Duration) ([]*events.Message, *types.MessageInfo, error) {
	// Create a channel to receive history sync events
	historySyncChan := make(chan *events.HistorySync, 1)

	// Register event handler for this specific request
	handlerID := cli.AddEventHandler(func(evt interface{}) {
		if hs, ok := evt.(*events.HistorySync); ok {
			// Only process if it contains conversations for our chat
			for _, conv := range hs.Data.GetConversations() {
				if conv.GetId() == anchorMsg.Chat.String() {
					select {
					case historySyncChan <- hs:
					default:
						// Channel already has a value, skip
					}
					return
				}
			}
		}
	})

	// Ensure we clean up the handler
	defer cli.RemoveEventHandler(handlerID)

	// Build and send history sync request
	histSyncMsg := cli.BuildHistorySyncRequest(anchorMsg, count)

	// Send the message to own JID to request history
	ownJID := cli.getOwnID()
	if ownJID.IsEmpty() {
		return nil, nil, ErrNotLoggedIn
	}

	_, err := cli.SendMessage(ctx, ownJID.ToNonAD(), histSyncMsg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send history sync request: %w", err)
	}

	// Wait for history sync response with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case historySync := <-historySyncChan:
		return cli.parseHistorySyncMessages(historySync, anchorMsg.Chat)
	case <-timeoutCtx.Done():
		return nil, nil, ErrHistorySyncTimeout
	}
}

// parseHistorySyncMessages parses messages from a history sync event for a specific chat.
// Returns the parsed messages and the oldest message info for pagination.
func (cli *Client) parseHistorySyncMessages(historySync *events.HistorySync, chatJID types.JID) ([]*events.Message, *types.MessageInfo, error) {
	var messages []*events.Message
	var oldestMsg *types.MessageInfo

	// Find the conversation for this chat
	for _, conv := range historySync.Data.GetConversations() {
		if conv.GetId() != chatJID.String() {
			continue
		}

		// Parse each message in the conversation
		for _, histMsg := range conv.GetMessages() {
			webMsg := histMsg.GetMessage()
			evt, err := cli.ParseWebMessage(chatJID, webMsg)
			if err != nil {
				cli.Log.Warnf("Failed to parse message from history sync: %v", err)
				continue
			}

			messages = append(messages, evt)

			// Track the oldest message for pagination
			if oldestMsg == nil || evt.Info.Timestamp.Before(oldestMsg.Timestamp) {
				oldestMsg = &evt.Info
			}
		}
	}

	return messages, oldestMsg, nil
}

// GetMessagesFromLastWeek is a convenience function to retrieve messages from the last 7 days.
func (cli *Client) GetMessagesFromLastWeek(ctx context.Context, chatJID types.JID) ([]*events.Message, error) {
	now := time.Now()
	query := DateRangeQuery{
		ChatJID:     chatJID,
		StartTime:   now.AddDate(0, 0, -7),
		EndTime:     now,
		MaxMessages: 1000, // Reasonable limit for a week
	}
	return cli.GetMessagesInDateRange(ctx, query)
}

// GetMessagesFromLastDay is a convenience function to retrieve messages from the last 24 hours.
func (cli *Client) GetMessagesFromLastDay(ctx context.Context, chatJID types.JID) ([]*events.Message, error) {
	now := time.Now()
	query := DateRangeQuery{
		ChatJID:     chatJID,
		StartTime:   now.Add(-24 * time.Hour),
		EndTime:     now,
		MaxMessages: 500, // Reasonable limit for a day
	}
	return cli.GetMessagesInDateRange(ctx, query)
}

// GetMessagesFromCustomRange is a convenience function to retrieve messages from a custom time range.
func (cli *Client) GetMessagesFromCustomRange(ctx context.Context, chatJID types.JID, daysAgo int) ([]*events.Message, error) {
	now := time.Now()
	query := DateRangeQuery{
		ChatJID:     chatJID,
		StartTime:   now.AddDate(0, 0, -daysAgo),
		EndTime:     now,
		MaxMessages: 0, // No limit
	}
	return cli.GetMessagesInDateRange(ctx, query)
}
