package whatsapp

type Handler interface {
	HandleError(err error)
}
type TextMessageHandler interface {
	Handler
	HandleTextMessage(message TextMessage)
}
type ImageMessageHandler interface {
	Handler
	HandleImageMessage(message ImageMessage)
}
type VideoMessageHandler interface {
	Handler
	HandleVideoMessage(message VideoMessage)
}

func (wac *conn) AddHandler(handler Handler) {
	wac.dispatcher.handler = append(wac.dispatcher.handler, handler)
}

func (dp *dispatcher) handle(message interface{}) {
	switch m := message.(type) {
	case error:
		for _, h := range dp.handler {
			go h.HandleError(m)
		}
	case TextMessage:
		for _, h := range dp.handler {
			x, ok := h.(TextMessageHandler)
			if !ok {
				continue
			}
			go x.HandleTextMessage(m)
		}
	case ImageMessage:
		for _, h := range dp.handler {
			x, ok := h.(ImageMessageHandler)
			if !ok {
				continue
			}
			go x.HandleImageMessage(m)
		}
	case VideoMessage:
		for _, h := range dp.handler {
			x, ok := h.(VideoMessageHandler)
			if !ok {
				continue
			}
			go x.HandleVideoMessage(m)
		}
	}
}
