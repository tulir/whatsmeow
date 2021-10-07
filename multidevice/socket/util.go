// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"sync"
)

type Frameable interface {
	SetOnFrame(func([]byte))
	GetOnFrame() func([]byte)
}

func ConsumeNextFrame(frameable Frameable) (output <-chan []byte, cancel func()) {
	prevOnFrame := frameable.GetOnFrame()
	var once sync.Once
	onFinish := func() {
		once.Do(func() {
			frameable.SetOnFrame(prevOnFrame)
		})
	}
	ch := make(chan []byte, 1)
	frameable.SetOnFrame(func(bytes []byte) {
		ch <- bytes
		onFinish()
	})
	return ch, onFinish
}
