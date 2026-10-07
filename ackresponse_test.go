// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"
	"sync"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
)

func mediaACK(id string) *waBinary.Node {
	return &waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{
		"id": id, "class": "receipt", "type": "server-error", "error": "429",
	}}
}

func TestACKResponseMatchesBeforeConsumingWaiter(t *testing.T) {
	for _, unrelated := range []*waBinary.Node{
		{Tag: "ack", Attrs: waBinary.Attrs{"id": "M", "class": "receipt"}},
		{Tag: "ack", Attrs: waBinary.Attrs{"id": "M", "class": "receipt", "type": "read"}},
		{Tag: "ack", Attrs: waBinary.Attrs{"id": "M", "class": "message", "type": "server-error"}},
		{Tag: "ack", Attrs: waBinary.Attrs{"id": "other", "class": "receipt", "type": "server-error"}},
		{Tag: "iq", Attrs: waBinary.Attrs{"id": "M", "type": "result"}},
	} {
		t.Run(fmt.Sprint(unrelated), func(t *testing.T) {
			cli := NewClient(&store.Device{}, nil)
			response, cancel := cli.WaitResponseForACK("M", "receipt", "server-error")
			defer cancel()
			if cli.receiveResponse(context.Background(), unrelated) {
				t.Fatal("unrelated response consumed the qualified waiter")
			}
			ack := mediaACK("M")
			// Deliver both frames before reading the channel. Re-registering
			// after receiving the first frame would lose this second frame.
			if !cli.receiveResponse(context.Background(), ack) {
				t.Fatal("matching media ACK lost its waiter")
			}
			if got := <-response; got != ack {
				t.Fatalf("got %v, want the media-retry rejection", got)
			}
			if cli.receiveResponse(context.Background(), ack) {
				t.Fatal("matching ACK did not remove its waiter")
			}
		})
	}
}

func TestACKResponseCoexistsWithIDOnlyWaiter(t *testing.T) {
	for _, mediaFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(mediaFirst), func(t *testing.T) {
			cli := NewClient(&store.Device{}, nil)
			other := cli.waitResponse("M")
			response, cancel := cli.WaitResponseForACK("M", "receipt", "server-error")
			defer cancel()
			ack := mediaACK("M")
			delivery := &waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"id": "M", "class": "receipt"}}
			frames := []*waBinary.Node{delivery, ack}
			if mediaFirst {
				frames[0], frames[1] = frames[1], frames[0]
			}
			for _, frame := range frames {
				if !cli.receiveResponse(context.Background(), frame) {
					t.Fatalf("lost response %v", frame)
				}
			}
			if got := <-other; got != delivery {
				t.Fatal("ID-only waiter received the media ACK")
			}
			if got := <-response; got != ack {
				t.Fatal("qualified waiter received the delivery ACK")
			}
		})
	}
}

func TestACKResponseCancellationPreservesOtherWaiters(t *testing.T) {
	cli := NewClient(&store.Device{}, nil)
	_, cancelOld := cli.WaitResponseForACK("M", "receipt", "server-error")
	if !cli.receiveResponse(context.Background(), mediaACK("M")) {
		t.Fatal("first ACK was not delivered")
	}
	response, cancelNew := cli.WaitResponseForACK("M", "receipt", "server-error")
	defer cancelNew()
	other := cli.waitResponse("M")
	cancelOld()
	cancelOld()
	ack := mediaACK("M")
	if !cli.receiveResponse(context.Background(), ack) {
		t.Fatal("old cancellation removed the successor")
	}
	if got := <-response; got != ack {
		t.Fatal("successor received a cleanup response")
	}
	delivery := &waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"id": "M", "class": "receipt"}}
	if !cli.receiveResponse(context.Background(), delivery) {
		t.Fatal("cancellation removed the ID-only waiter")
	}
	if got := <-other; got != delivery {
		t.Fatal("ID-only waiter received a cleanup response")
	}
}

func TestACKResponseDisconnectReleasesWaiter(t *testing.T) {
	for _, node := range []*waBinary.Node{nil, xmlStreamEndNode, {Tag: "stream:error"}} {
		t.Run(fmt.Sprint(node), func(t *testing.T) {
			cli := NewClient(&store.Device{}, nil)
			response, cancel := cli.WaitResponseForACK("M", "receipt", "server-error")
			defer cancel()
			cli.clearResponseWaiters(node)
			if got := <-response; got != node {
				t.Fatalf("disconnect returned %v, want %v", got, node)
			}
			if cli.receiveResponse(context.Background(), mediaACK("M")) {
				t.Fatal("disconnect retained the waiter")
			}
		})
	}
}

func TestACKResponseCancelUnansweredWaiter(t *testing.T) {
	cli := NewClient(&store.Device{}, nil)
	response, cancel := cli.WaitResponseForACK("M", "receipt", "server-error")
	cancel()
	cancel()
	if cli.receiveResponse(context.Background(), mediaACK("M")) {
		t.Fatal("cancelled registration still accepted a response")
	}
	select {
	case <-response:
		t.Fatal("cancellation closed or delivered to the response channel")
	default:
	}
}

func TestACKResponseCancellationRacesDeliveryAndDisconnect(t *testing.T) {
	cli := NewClient(&store.Device{}, nil)
	for range 500 {
		_, cancel := cli.WaitResponseForACK("M", "receipt", "server-error")
		start := make(chan struct{})
		var done sync.WaitGroup
		for _, action := range []func(){
			func() { cli.receiveResponse(context.Background(), mediaACK("M")) },
			func() { cli.clearResponseWaiters(xmlStreamEndNode) },
			cancel,
		} {
			done.Go(func() { <-start; action() })
		}
		close(start)
		done.Wait()
		if cli.receiveResponse(context.Background(), mediaACK("M")) {
			t.Fatal("completed cancellation left a response waiter")
		}
	}
}
