# whatsmeow
[![godocs.io](https://godocs.io/go.mau.fi/whatsmeow?status.svg)](https://godocs.io/go.mau.fi/whatsmeow)

whatsmeow is a Go library for the WhatsApp web multidevice API.

The basics already work (sending and receiving messages), but lots of methods
and events still need to be added. Documentation will be added later.

This was initially forked from [go-whatsapp] (MIT license), but large parts of
the code have been rewritten for multidevice support. Parts of the code are
ported from [WhatsappWeb4j] and [Baileys] (also MIT license).

[go-whatsapp]: https://github.com/Rhymen/go-whatsapp
[WhatsappWeb4j]: https://github.com/Auties00/WhatsappWeb4j
[Baileys]: https://github.com/adiwajshing/Baileys

## Usage
The [godoc](https://godocs.io/go.mau.fi/whatsmeow) includes docs for all methods and event types.
There's also a [simple example](https://godocs.io/go.mau.fi/whatsmeow#example-package) at the top.

Also see [mdtest](./mdtest) for a CLI tool you can easily try out whatsmeow with.

## Discussion
Matrix room: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)
