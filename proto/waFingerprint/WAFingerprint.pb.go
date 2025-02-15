// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.28.2
// source: waFingerprint/WAFingerprint.proto

package waFingerprint

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type HostedState int32

const (
	HostedState_E2EE   HostedState = 0
	HostedState_HOSTED HostedState = 1
)

// Enum value maps for HostedState.
var (
	HostedState_name = map[int32]string{
		0: "E2EE",
		1: "HOSTED",
	}
	HostedState_value = map[string]int32{
		"E2EE":   0,
		"HOSTED": 1,
	}
)

func (x HostedState) Enum() *HostedState {
	p := new(HostedState)
	*p = x
	return p
}

func (x HostedState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (HostedState) Descriptor() protoreflect.EnumDescriptor {
	return file_waFingerprint_WAFingerprint_proto_enumTypes[0].Descriptor()
}

func (HostedState) Type() protoreflect.EnumType {
	return &file_waFingerprint_WAFingerprint_proto_enumTypes[0]
}

func (x HostedState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Do not use.
func (x *HostedState) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = HostedState(num)
	return nil
}

// Deprecated: Use HostedState.Descriptor instead.
func (HostedState) EnumDescriptor() ([]byte, []int) {
	return file_waFingerprint_WAFingerprint_proto_rawDescGZIP(), []int{0}
}

type FingerprintData struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	PublicKey          []byte                 `protobuf:"bytes,1,opt,name=publicKey" json:"publicKey,omitempty"`
	PnIdentifier       []byte                 `protobuf:"bytes,2,opt,name=pnIdentifier" json:"pnIdentifier,omitempty"`
	LidIdentifier      []byte                 `protobuf:"bytes,3,opt,name=lidIdentifier" json:"lidIdentifier,omitempty"`
	UsernameIdentifier []byte                 `protobuf:"bytes,4,opt,name=usernameIdentifier" json:"usernameIdentifier,omitempty"`
	HostedState        *HostedState           `protobuf:"varint,5,opt,name=hostedState,enum=WAFingerprint.HostedState" json:"hostedState,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *FingerprintData) Reset() {
	*x = FingerprintData{}
	mi := &file_waFingerprint_WAFingerprint_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FingerprintData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FingerprintData) ProtoMessage() {}

func (x *FingerprintData) ProtoReflect() protoreflect.Message {
	mi := &file_waFingerprint_WAFingerprint_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FingerprintData.ProtoReflect.Descriptor instead.
func (*FingerprintData) Descriptor() ([]byte, []int) {
	return file_waFingerprint_WAFingerprint_proto_rawDescGZIP(), []int{0}
}

func (x *FingerprintData) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

func (x *FingerprintData) GetPnIdentifier() []byte {
	if x != nil {
		return x.PnIdentifier
	}
	return nil
}

func (x *FingerprintData) GetLidIdentifier() []byte {
	if x != nil {
		return x.LidIdentifier
	}
	return nil
}

func (x *FingerprintData) GetUsernameIdentifier() []byte {
	if x != nil {
		return x.UsernameIdentifier
	}
	return nil
}

func (x *FingerprintData) GetHostedState() HostedState {
	if x != nil && x.HostedState != nil {
		return *x.HostedState
	}
	return HostedState_E2EE
}

type CombinedFingerprint struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Version           *uint32                `protobuf:"varint,1,opt,name=version" json:"version,omitempty"`
	LocalFingerprint  *FingerprintData       `protobuf:"bytes,2,opt,name=localFingerprint" json:"localFingerprint,omitempty"`
	RemoteFingerprint *FingerprintData       `protobuf:"bytes,3,opt,name=remoteFingerprint" json:"remoteFingerprint,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *CombinedFingerprint) Reset() {
	*x = CombinedFingerprint{}
	mi := &file_waFingerprint_WAFingerprint_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CombinedFingerprint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CombinedFingerprint) ProtoMessage() {}

func (x *CombinedFingerprint) ProtoReflect() protoreflect.Message {
	mi := &file_waFingerprint_WAFingerprint_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CombinedFingerprint.ProtoReflect.Descriptor instead.
func (*CombinedFingerprint) Descriptor() ([]byte, []int) {
	return file_waFingerprint_WAFingerprint_proto_rawDescGZIP(), []int{1}
}

func (x *CombinedFingerprint) GetVersion() uint32 {
	if x != nil && x.Version != nil {
		return *x.Version
	}
	return 0
}

func (x *CombinedFingerprint) GetLocalFingerprint() *FingerprintData {
	if x != nil {
		return x.LocalFingerprint
	}
	return nil
}

func (x *CombinedFingerprint) GetRemoteFingerprint() *FingerprintData {
	if x != nil {
		return x.RemoteFingerprint
	}
	return nil
}

var File_waFingerprint_WAFingerprint_proto protoreflect.FileDescriptor

var file_waFingerprint_WAFingerprint_proto_rawDesc = string([]byte{
	0x0a, 0x21, 0x77, 0x61, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x2f,
	0x57, 0x41, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x57, 0x41, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69,
	0x6e, 0x74, 0x22, 0xe7, 0x01, 0x0a, 0x0f, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69,
	0x6e, 0x74, 0x44, 0x61, 0x74, 0x61, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63,
	0x4b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69,
	0x63, 0x4b, 0x65, 0x79, 0x12, 0x22, 0x0a, 0x0c, 0x70, 0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69,
	0x66, 0x69, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0c, 0x70, 0x6e, 0x49, 0x64,
	0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x12, 0x24, 0x0a, 0x0d, 0x6c, 0x69, 0x64, 0x49,
	0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x0d, 0x6c, 0x69, 0x64, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x12, 0x2e,
	0x0a, 0x12, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69,
	0x66, 0x69, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x12, 0x75, 0x73, 0x65, 0x72,
	0x6e, 0x61, 0x6d, 0x65, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x12, 0x3c,
	0x0a, 0x0b, 0x68, 0x6f, 0x73, 0x74, 0x65, 0x64, 0x53, 0x74, 0x61, 0x74, 0x65, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x1a, 0x2e, 0x57, 0x41, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72,
	0x69, 0x6e, 0x74, 0x2e, 0x48, 0x6f, 0x73, 0x74, 0x65, 0x64, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52,
	0x0b, 0x68, 0x6f, 0x73, 0x74, 0x65, 0x64, 0x53, 0x74, 0x61, 0x74, 0x65, 0x22, 0xc9, 0x01, 0x0a,
	0x13, 0x43, 0x6f, 0x6d, 0x62, 0x69, 0x6e, 0x65, 0x64, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70,
	0x72, 0x69, 0x6e, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x4a,
	0x0a, 0x10, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69,
	0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x57, 0x41, 0x46, 0x69, 0x6e,
	0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x2e, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70,
	0x72, 0x69, 0x6e, 0x74, 0x44, 0x61, 0x74, 0x61, 0x52, 0x10, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x46,
	0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x12, 0x4c, 0x0a, 0x11, 0x72, 0x65,
	0x6d, 0x6f, 0x74, 0x65, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x57, 0x41, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72,
	0x70, 0x72, 0x69, 0x6e, 0x74, 0x2e, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e,
	0x74, 0x44, 0x61, 0x74, 0x61, 0x52, 0x11, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x46, 0x69, 0x6e,
	0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x2a, 0x23, 0x0a, 0x0b, 0x48, 0x6f, 0x73, 0x74,
	0x65, 0x64, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x08, 0x0a, 0x04, 0x45, 0x32, 0x45, 0x45, 0x10,
	0x00, 0x12, 0x0a, 0x0a, 0x06, 0x48, 0x4f, 0x53, 0x54, 0x45, 0x44, 0x10, 0x01, 0x42, 0x29, 0x5a,
	0x27, 0x67, 0x6f, 0x2e, 0x6d, 0x61, 0x75, 0x2e, 0x66, 0x69, 0x2f, 0x77, 0x68, 0x61, 0x74, 0x73,
	0x6d, 0x65, 0x6f, 0x77, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x77, 0x61, 0x46, 0x69, 0x6e,
	0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74,
})

var (
	file_waFingerprint_WAFingerprint_proto_rawDescOnce sync.Once
	file_waFingerprint_WAFingerprint_proto_rawDescData []byte
)

func file_waFingerprint_WAFingerprint_proto_rawDescGZIP() []byte {
	file_waFingerprint_WAFingerprint_proto_rawDescOnce.Do(func() {
		file_waFingerprint_WAFingerprint_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_waFingerprint_WAFingerprint_proto_rawDesc), len(file_waFingerprint_WAFingerprint_proto_rawDesc)))
	})
	return file_waFingerprint_WAFingerprint_proto_rawDescData
}

var file_waFingerprint_WAFingerprint_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_waFingerprint_WAFingerprint_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_waFingerprint_WAFingerprint_proto_goTypes = []any{
	(HostedState)(0),            // 0: WAFingerprint.HostedState
	(*FingerprintData)(nil),     // 1: WAFingerprint.FingerprintData
	(*CombinedFingerprint)(nil), // 2: WAFingerprint.CombinedFingerprint
}
var file_waFingerprint_WAFingerprint_proto_depIdxs = []int32{
	0, // 0: WAFingerprint.FingerprintData.hostedState:type_name -> WAFingerprint.HostedState
	1, // 1: WAFingerprint.CombinedFingerprint.localFingerprint:type_name -> WAFingerprint.FingerprintData
	1, // 2: WAFingerprint.CombinedFingerprint.remoteFingerprint:type_name -> WAFingerprint.FingerprintData
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_waFingerprint_WAFingerprint_proto_init() }
func file_waFingerprint_WAFingerprint_proto_init() {
	if File_waFingerprint_WAFingerprint_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_waFingerprint_WAFingerprint_proto_rawDesc), len(file_waFingerprint_WAFingerprint_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_waFingerprint_WAFingerprint_proto_goTypes,
		DependencyIndexes: file_waFingerprint_WAFingerprint_proto_depIdxs,
		EnumInfos:         file_waFingerprint_WAFingerprint_proto_enumTypes,
		MessageInfos:      file_waFingerprint_WAFingerprint_proto_msgTypes,
	}.Build()
	File_waFingerprint_WAFingerprint_proto = out.File
	file_waFingerprint_WAFingerprint_proto_goTypes = nil
	file_waFingerprint_WAFingerprint_proto_depIdxs = nil
}
