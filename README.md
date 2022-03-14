# whatsmeow
[![Go Reference](https://pkg.go.dev/badge/go.mau.fi/whatsmeow.svg)](https://pkg.go.dev/go.mau.fi/whatsmeow)

whatsmeow is a Go library for the WhatsApp web multidevice API.

## Discussion
Matrix room: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)

## Usage
The [godoc](https://pkg.go.dev/go.mau.fi/whatsmeow) includes docs for all methods and event types.
There's also a [simple example](https://godocs.io/go.mau.fi/whatsmeow#example-package) at the top.

Also see [mdtest](./mdtest) for a CLI tool you can easily try out whatsmeow with.

## Features
Most core features are already present:

* Sending messages to private chats and groups (both text and media)
* Receiving all messages
* Managing groups and receiving group change events
* Joining via invite messages, using and creating invite links
* Sending and receiving typing notifications
* Sending and receiving delivery and read receipts
* Reading app state (contact list, chat pin/mute status, etc)
* Sending and handling retry receipts if message decryption fails

Things that are not yet implemented:

* Writing app state (contact list, chat pin/mute status, etc)
* Sending status messages or broadcast list messages (this is not supported on WhatsApp web either)
* Calls
