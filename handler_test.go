package whatsapp

import (
	"testing"
)

type h1 struct {
	a string
}

func (h *h1) HandleError(err error) {}

type h2 struct {
	b string
}

func (h *h2) HandleError(err error) {}

func TestAddRemoveHandlers(t *testing.T) {
	wac := &Conn{
		handler: make([]Handler, 0),
	}
	hh1 := &h1{}
	hh2 := &h2{b: "b"}
	hh3 := &h2{b: "b2"}
	wac.AddHandler(hh1)
	wac.AddHandler(hh2)
	wac.AddHandler(hh3)
	wac.RemoveHandler(hh2)
	z := wac.handler[0].(*h1)
	if z != hh1 {
		t.Fail()
	}
	z2 := wac.handler[1].(*h2)
	if z2.b != hh3.b {
		t.Fail()
	}
	if len(wac.handler) != 2 {
		t.Fail()
	}
}
