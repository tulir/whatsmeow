// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.28.2
// source: waArmadilloICDC/WAArmadilloICDC.proto

package waArmadilloICDC

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

type ICDCIdentityList struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	Seq                *int32                 `protobuf:"varint,1,opt,name=seq" json:"seq,omitempty"`
	Timestamp          *int64                 `protobuf:"varint,2,opt,name=timestamp" json:"timestamp,omitempty"`
	Devices            [][]byte               `protobuf:"bytes,3,rep,name=devices" json:"devices,omitempty"`
	SigningDeviceIndex *int32                 `protobuf:"varint,4,opt,name=signingDeviceIndex" json:"signingDeviceIndex,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *ICDCIdentityList) Reset() {
	*x = ICDCIdentityList{}
	mi := &file_waArmadilloICDC_WAArmadilloICDC_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ICDCIdentityList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ICDCIdentityList) ProtoMessage() {}

func (x *ICDCIdentityList) ProtoReflect() protoreflect.Message {
	mi := &file_waArmadilloICDC_WAArmadilloICDC_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ICDCIdentityList.ProtoReflect.Descriptor instead.
func (*ICDCIdentityList) Descriptor() ([]byte, []int) {
	return file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescGZIP(), []int{0}
}

func (x *ICDCIdentityList) GetSeq() int32 {
	if x != nil && x.Seq != nil {
		return *x.Seq
	}
	return 0
}

func (x *ICDCIdentityList) GetTimestamp() int64 {
	if x != nil && x.Timestamp != nil {
		return *x.Timestamp
	}
	return 0
}

func (x *ICDCIdentityList) GetDevices() [][]byte {
	if x != nil {
		return x.Devices
	}
	return nil
}

func (x *ICDCIdentityList) GetSigningDeviceIndex() int32 {
	if x != nil && x.SigningDeviceIndex != nil {
		return *x.SigningDeviceIndex
	}
	return 0
}

type SignedICDCIdentityList struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Details       []byte                 `protobuf:"bytes,1,opt,name=details" json:"details,omitempty"`
	Signature     []byte                 `protobuf:"bytes,2,opt,name=signature" json:"signature,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SignedICDCIdentityList) Reset() {
	*x = SignedICDCIdentityList{}
	mi := &file_waArmadilloICDC_WAArmadilloICDC_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignedICDCIdentityList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedICDCIdentityList) ProtoMessage() {}

func (x *SignedICDCIdentityList) ProtoReflect() protoreflect.Message {
	mi := &file_waArmadilloICDC_WAArmadilloICDC_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedICDCIdentityList.ProtoReflect.Descriptor instead.
func (*SignedICDCIdentityList) Descriptor() ([]byte, []int) {
	return file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescGZIP(), []int{1}
}

func (x *SignedICDCIdentityList) GetDetails() []byte {
	if x != nil {
		return x.Details
	}
	return nil
}

func (x *SignedICDCIdentityList) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

var File_waArmadilloICDC_WAArmadilloICDC_proto protoreflect.FileDescriptor

var file_waArmadilloICDC_WAArmadilloICDC_proto_rawDesc = string([]byte{
	0x0a, 0x25, 0x77, 0x61, 0x41, 0x72, 0x6d, 0x61, 0x64, 0x69, 0x6c, 0x6c, 0x6f, 0x49, 0x43, 0x44,
	0x43, 0x2f, 0x57, 0x41, 0x41, 0x72, 0x6d, 0x61, 0x64, 0x69, 0x6c, 0x6c, 0x6f, 0x49, 0x43, 0x44,
	0x43, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0f, 0x57, 0x41, 0x41, 0x72, 0x6d, 0x61, 0x64,
	0x69, 0x6c, 0x6c, 0x6f, 0x49, 0x43, 0x44, 0x43, 0x22, 0x8c, 0x01, 0x0a, 0x10, 0x49, 0x43, 0x44,
	0x43, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x10, 0x0a,
	0x03, 0x73, 0x65, 0x71, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x73, 0x65, 0x71, 0x12,
	0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x18, 0x0a,
	0x07, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x07,
	0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x73, 0x12, 0x2e, 0x0a, 0x12, 0x73, 0x69, 0x67, 0x6e, 0x69,
	0x6e, 0x67, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x12, 0x73, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x44, 0x65, 0x76, 0x69,
	0x63, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x22, 0x50, 0x0a, 0x16, 0x53, 0x69, 0x67, 0x6e, 0x65,
	0x64, 0x49, 0x43, 0x44, 0x43, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x4c, 0x69, 0x73,
	0x74, 0x12, 0x18, 0x0a, 0x07, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x07, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x1c, 0x0a, 0x09, 0x73,
	0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09,
	0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x42, 0x2b, 0x5a, 0x29, 0x67, 0x6f, 0x2e,
	0x6d, 0x61, 0x75, 0x2e, 0x66, 0x69, 0x2f, 0x77, 0x68, 0x61, 0x74, 0x73, 0x6d, 0x65, 0x6f, 0x77,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x77, 0x61, 0x41, 0x72, 0x6d, 0x61, 0x64, 0x69, 0x6c,
	0x6c, 0x6f, 0x49, 0x43, 0x44, 0x43,
})

var (
	file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescOnce sync.Once
	file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescData []byte
)

func file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescGZIP() []byte {
	file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescOnce.Do(func() {
		file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_waArmadilloICDC_WAArmadilloICDC_proto_rawDesc), len(file_waArmadilloICDC_WAArmadilloICDC_proto_rawDesc)))
	})
	return file_waArmadilloICDC_WAArmadilloICDC_proto_rawDescData
}

var file_waArmadilloICDC_WAArmadilloICDC_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_waArmadilloICDC_WAArmadilloICDC_proto_goTypes = []any{
	(*ICDCIdentityList)(nil),       // 0: WAArmadilloICDC.ICDCIdentityList
	(*SignedICDCIdentityList)(nil), // 1: WAArmadilloICDC.SignedICDCIdentityList
}
var file_waArmadilloICDC_WAArmadilloICDC_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_waArmadilloICDC_WAArmadilloICDC_proto_init() }
func file_waArmadilloICDC_WAArmadilloICDC_proto_init() {
	if File_waArmadilloICDC_WAArmadilloICDC_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_waArmadilloICDC_WAArmadilloICDC_proto_rawDesc), len(file_waArmadilloICDC_WAArmadilloICDC_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_waArmadilloICDC_WAArmadilloICDC_proto_goTypes,
		DependencyIndexes: file_waArmadilloICDC_WAArmadilloICDC_proto_depIdxs,
		MessageInfos:      file_waArmadilloICDC_WAArmadilloICDC_proto_msgTypes,
	}.Build()
	File_waArmadilloICDC_WAArmadilloICDC_proto = out.File
	file_waArmadilloICDC_WAArmadilloICDC_proto_goTypes = nil
	file_waArmadilloICDC_WAArmadilloICDC_proto_depIdxs = nil
}
