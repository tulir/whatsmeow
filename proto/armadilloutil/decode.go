package armadilloutil

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waCommon"
)

var ErrUnsupportedVersion = errors.New("unsupported subprotocol version")

func Unmarshal[T proto.Message](into T, msg *waCommon.SubProtocol, expectedVersion int32) (T, error) {
	if msg.GetVersion() != expectedVersion {
		return into, fmt.Errorf("%w %d in %T (expected %d)", ErrUnsupportedVersion, msg.GetVersion(), into, expectedVersion)
	}

	err := proto.Unmarshal(msg.GetPayload(), into)
	return into, err
}

func Marshal[T proto.Message](msg T, version int32) (*waCommon.SubProtocol, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return &waCommon.SubProtocol{
		Payload: payload,
		Version: &version,
	}, nil
}
