# whatsmeow
[![godocs.io](https://godocs.io/go.mau.fi/whatsmeow?status.svg)](https://godocs.io/go.mau.fi/whatsmeow)

whatsmeow is a Go library for the WhatsApp web multidevice API.

This was initially forked from [go-whatsapp] (MIT license), but large parts of
the code have been rewritten for multidevice support. Parts of the code are
ported from [WhatsappWeb4j] and [Baileys] (also MIT license).

[go-whatsapp]: https://github.com/Rhymen/go-whatsapp
[WhatsappWeb4j]: https://github.com/Auties00/WhatsappWeb4j
[Baileys]: https://github.com/adiwajshing/Baileys

## Discussion
Matrix room: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)

## Usage
The [godoc](https://godocs.io/go.mau.fi/whatsmeow) includes docs for all methods and event types.
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
