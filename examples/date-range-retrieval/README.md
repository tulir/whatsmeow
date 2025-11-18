# Date Range Message Retrieval Example

This example demonstrates how to retrieve WhatsApp messages from a specific contact within a date range using the `GetMessagesInDateRange` function.

## Features

- Retrieve messages from any time period (last week, last day, custom range)
- Filter messages by specific chat/contact
- Limit the number of messages retrieved
- Parse and display different message types (text, images, videos, etc.)

## Usage

### Basic Usage

```bash
go run main.go -phone +1234567890 -days 7 -max 100
```

### Parameters

- `-phone`: Phone number in international format (required)
  - Example: `+1234567890`
  - Format: `+[country code][number]`
- `-days`: Number of days ago to retrieve messages from (default: 7)
  - Example: `-days 30` for last 30 days
- `-max`: Maximum number of messages to retrieve (default: 100)
  - Example: `-max 500` to retrieve up to 500 messages

### Examples

1. **Retrieve messages from last week**:
   ```bash
   go run main.go -phone +1234567890 -days 7
   ```

2. **Retrieve messages from last 30 days with limit of 500**:
   ```bash
   go run main.go -phone +1234567890 -days 30 -max 500
   ```

3. **Retrieve messages from yesterday**:
   ```bash
   go run main.go -phone +1234567890 -days 1 -max 200
   ```

## How It Works

1. **Connection**: The example connects to WhatsApp using stored credentials or QR code pairing
2. **Query Building**: Creates a `DateRangeQuery` with the specified parameters
3. **Message Retrieval**: Calls `GetMessagesInDateRange` which:
   - Sends history sync requests to WhatsApp
   - Paginates through message batches
   - Filters messages by timestamp
   - Returns messages within the date range
4. **Display**: Shows retrieved messages with timestamps and content

## API Usage

### Basic Date Range Query

```go
import (
    "context"
    "time"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types"
)

// Create a query
query := whatsmeow.DateRangeQuery{
    ChatJID:     chatJID,                          // Contact/group JID
    StartTime:   time.Now().AddDate(0, 0, -7),     // 7 days ago
    EndTime:     time.Now(),                        // Now
    MaxMessages: 100,                               // Limit to 100 messages
    Timeout:     30 * time.Second,                  // Timeout for each request
}

// Retrieve messages
messages, err := client.GetMessagesInDateRange(context.Background(), query)
if err != nil {
    log.Printf("Error: %v", err)
    return
}

// Process messages
for _, msg := range messages {
    log.Printf("From: %s, Time: %s, Text: %s",
        msg.Info.Sender,
        msg.Info.Timestamp,
        msg.Message.GetConversation(),
    )
}
```

### Convenience Functions

The library also provides convenience functions for common use cases:

```go
// Get messages from last week
messages, err := client.GetMessagesFromLastWeek(ctx, chatJID)

// Get messages from last 24 hours
messages, err := client.GetMessagesFromLastDay(ctx, chatJID)

// Get messages from custom number of days
messages, err := client.GetMessagesFromCustomRange(ctx, chatJID, 30) // Last 30 days
```

## Message Types Supported

The example can parse and display:
- ✅ Text messages
- ✅ Extended text messages (with formatting, links)
- ✅ Images (with captions)
- ✅ Videos (with captions)
- ✅ Audio messages
- ✅ Documents
- ✅ Contacts
- ✅ Locations
- ✅ Stickers
- ✅ Reactions
- ✅ Protocol messages (edits, deletes, etc.)

## Important Notes

### Limitations

1. **History Availability**: You can only retrieve messages that are available in WhatsApp's history sync. This typically includes recent messages, but very old messages may not be available.

2. **Pagination**: The function paginates through messages in batches of 50 (WhatsApp's recommended size). For large date ranges, this may take multiple requests.

3. **Rate Limiting**: WhatsApp may rate limit history sync requests. If you encounter issues, try reducing the `MaxMessages` limit or the date range.

4. **Anchor Messages**: The function works best when you have recent message activity with the contact. If there's no recent history, you may need to wait for a new message to establish an anchor point.

### Best Practices

1. **Use Reasonable Limits**: Start with smaller date ranges and message limits to avoid timeouts
2. **Handle Errors**: Always check for errors and handle timeouts appropriately
3. **Cache Results**: Consider storing retrieved messages locally to avoid repeated API calls
4. **Respect Privacy**: Only retrieve messages for legitimate purposes and respect user privacy

## Troubleshooting

### "timeout waiting for history sync response"

- Increase the `Timeout` value in the query
- Reduce the `MaxMessages` limit
- Check your internet connection

### "no messages found in the specified date range"

- Verify the date range is correct
- Check that messages exist in that time period
- Try a more recent date range

### "failed to send history sync request"

- Ensure you're connected to WhatsApp (check connection status)
- Verify the chat JID is valid
- Make sure you're logged in properly

## Dependencies

```bash
go get go.mau.fi/whatsmeow
go get github.com/mattn/go-sqlite3
go get google.golang.org/protobuf/proto
```

## License

This example is licensed under the Mozilla Public License 2.0, same as whatsmeow.
