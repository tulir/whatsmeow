package wam

import (
	"encoding/binary"

	"go.mau.fi/whatsmeow/wam"
)

type EventMeta struct {
	id      int
	channel string
	weight  int
	statsID int
}

type Event struct {
	event   interface{}
	globals wam.WAMGlobals
}

type WAMEncoder struct {
	version  int
	sequence int
	events   []Event
	data     []byte
}

func NewDefaultWAMEncoder() *WAMEncoder { return &WAMEncoder{data: []byte{0}, sequence: 0, version: 5} }

func (wam *WAMEncoder) writeHeader() {
	headerBytes := make([]byte, 8)

	copy(headerBytes, "WAM") // header

	headerBytes[3] = byte(wam.version) // version of the protocol

	headerBytes[4] = 1 // random ?

	binary.BigEndian.PutUint16(headerBytes[5:], uint16(wam.sequence)) // wam buffer sequence

	headerBytes[7] = 0 // channel

	wam.data = append(wam.data, headerBytes...)
}

func (wam *WAMEncoder) PutEvent(event Event) {
	wam.events = append(wam.events, event)
}

func (wam *WAMEncoder) writeEvent(event Event) {
	eventBytes := make([]byte, 0)
	meta := extractEventMeta(event.event)
	extended := isExtended(event.event)

	// write globals
	eventBytes = append(eventBytes, encodeGlobalAttributes(event.globals)...)

	// write event header
	eventBytes = append(eventBytes, encodeEventHeader(meta, extended)...)

	// write props
	eventBytes = append(eventBytes, encodeEventProps(event.event)...)

	wam.data = append(wam.data, eventBytes...)
}

func (wam *WAMEncoder) GetData() []byte {
	wam.writeHeader()

	for _, event := range wam.events {
		wam.writeEvent(event)
	}

	return wam.data
}
