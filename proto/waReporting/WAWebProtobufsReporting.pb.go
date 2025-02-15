// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.28.2
// source: waReporting/WAWebProtobufsReporting.proto

package waReporting

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

type Reportable struct {
	state                   protoimpl.MessageState `protogen:"open.v1"`
	MinVersion              uint32                 `protobuf:"varint,1,opt,name=minVersion,proto3" json:"minVersion,omitempty"`
	MaxVersion              uint32                 `protobuf:"varint,2,opt,name=maxVersion,proto3" json:"maxVersion,omitempty"`
	NotReportableMinVersion uint32                 `protobuf:"varint,3,opt,name=notReportableMinVersion,proto3" json:"notReportableMinVersion,omitempty"`
	Never                   bool                   `protobuf:"varint,4,opt,name=never,proto3" json:"never,omitempty"`
	unknownFields           protoimpl.UnknownFields
	sizeCache               protoimpl.SizeCache
}

func (x *Reportable) Reset() {
	*x = Reportable{}
	mi := &file_waReporting_WAWebProtobufsReporting_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Reportable) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Reportable) ProtoMessage() {}

func (x *Reportable) ProtoReflect() protoreflect.Message {
	mi := &file_waReporting_WAWebProtobufsReporting_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Reportable.ProtoReflect.Descriptor instead.
func (*Reportable) Descriptor() ([]byte, []int) {
	return file_waReporting_WAWebProtobufsReporting_proto_rawDescGZIP(), []int{0}
}

func (x *Reportable) GetMinVersion() uint32 {
	if x != nil {
		return x.MinVersion
	}
	return 0
}

func (x *Reportable) GetMaxVersion() uint32 {
	if x != nil {
		return x.MaxVersion
	}
	return 0
}

func (x *Reportable) GetNotReportableMinVersion() uint32 {
	if x != nil {
		return x.NotReportableMinVersion
	}
	return 0
}

func (x *Reportable) GetNever() bool {
	if x != nil {
		return x.Never
	}
	return false
}

type Config struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Field         map[uint32]*Field      `protobuf:"bytes,1,rep,name=field,proto3" json:"field,omitempty" protobuf_key:"varint,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	Version       uint32                 `protobuf:"varint,2,opt,name=version,proto3" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Config) Reset() {
	*x = Config{}
	mi := &file_waReporting_WAWebProtobufsReporting_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Config) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Config) ProtoMessage() {}

func (x *Config) ProtoReflect() protoreflect.Message {
	mi := &file_waReporting_WAWebProtobufsReporting_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Config.ProtoReflect.Descriptor instead.
func (*Config) Descriptor() ([]byte, []int) {
	return file_waReporting_WAWebProtobufsReporting_proto_rawDescGZIP(), []int{1}
}

func (x *Config) GetField() map[uint32]*Field {
	if x != nil {
		return x.Field
	}
	return nil
}

func (x *Config) GetVersion() uint32 {
	if x != nil {
		return x.Version
	}
	return 0
}

type Field struct {
	state                   protoimpl.MessageState `protogen:"open.v1"`
	MinVersion              uint32                 `protobuf:"varint,1,opt,name=minVersion,proto3" json:"minVersion,omitempty"`
	MaxVersion              uint32                 `protobuf:"varint,2,opt,name=maxVersion,proto3" json:"maxVersion,omitempty"`
	NotReportableMinVersion uint32                 `protobuf:"varint,3,opt,name=notReportableMinVersion,proto3" json:"notReportableMinVersion,omitempty"`
	IsMessage               bool                   `protobuf:"varint,4,opt,name=isMessage,proto3" json:"isMessage,omitempty"`
	Subfield                map[uint32]*Field      `protobuf:"bytes,5,rep,name=subfield,proto3" json:"subfield,omitempty" protobuf_key:"varint,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields           protoimpl.UnknownFields
	sizeCache               protoimpl.SizeCache
}

func (x *Field) Reset() {
	*x = Field{}
	mi := &file_waReporting_WAWebProtobufsReporting_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Field) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Field) ProtoMessage() {}

func (x *Field) ProtoReflect() protoreflect.Message {
	mi := &file_waReporting_WAWebProtobufsReporting_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Field.ProtoReflect.Descriptor instead.
func (*Field) Descriptor() ([]byte, []int) {
	return file_waReporting_WAWebProtobufsReporting_proto_rawDescGZIP(), []int{2}
}

func (x *Field) GetMinVersion() uint32 {
	if x != nil {
		return x.MinVersion
	}
	return 0
}

func (x *Field) GetMaxVersion() uint32 {
	if x != nil {
		return x.MaxVersion
	}
	return 0
}

func (x *Field) GetNotReportableMinVersion() uint32 {
	if x != nil {
		return x.NotReportableMinVersion
	}
	return 0
}

func (x *Field) GetIsMessage() bool {
	if x != nil {
		return x.IsMessage
	}
	return false
}

func (x *Field) GetSubfield() map[uint32]*Field {
	if x != nil {
		return x.Subfield
	}
	return nil
}

var File_waReporting_WAWebProtobufsReporting_proto protoreflect.FileDescriptor

var file_waReporting_WAWebProtobufsReporting_proto_rawDesc = string([]byte{
	0x0a, 0x29, 0x77, 0x61, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x2f, 0x57, 0x41,
	0x57, 0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x52, 0x65, 0x70, 0x6f,
	0x72, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x17, 0x57, 0x41, 0x57,
	0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x52, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x69, 0x6e, 0x67, 0x22, 0x9c, 0x01, 0x0a, 0x0a, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x61,
	0x62, 0x6c, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x6d, 0x69, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0a, 0x6d, 0x69, 0x6e, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x1e, 0x0a, 0x0a, 0x6d, 0x61, 0x78, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0a, 0x6d, 0x61, 0x78, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x38, 0x0a, 0x17, 0x6e, 0x6f, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74,
	0x61, 0x62, 0x6c, 0x65, 0x4d, 0x69, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0d, 0x52, 0x17, 0x6e, 0x6f, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x61,
	0x62, 0x6c, 0x65, 0x4d, 0x69, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a,
	0x05, 0x6e, 0x65, 0x76, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x6e, 0x65,
	0x76, 0x65, 0x72, 0x22, 0xbe, 0x01, 0x0a, 0x06, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x40,
	0x0a, 0x05, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2a, 0x2e,
	0x57, 0x41, 0x57, 0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x52, 0x65,
	0x70, 0x6f, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x46,
	0x69, 0x65, 0x6c, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x05, 0x66, 0x69, 0x65, 0x6c, 0x64,
	0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x1a, 0x58, 0x0a, 0x0a, 0x46, 0x69,
	0x65, 0x6c, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x34, 0x0a, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x57, 0x41, 0x57, 0x65,
	0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74,
	0x69, 0x6e, 0x67, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x3a, 0x02, 0x38, 0x01, 0x22, 0xc6, 0x02, 0x0a, 0x05, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x1e,
	0x0a, 0x0a, 0x6d, 0x69, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x0a, 0x6d, 0x69, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1e,
	0x0a, 0x0a, 0x6d, 0x61, 0x78, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x0a, 0x6d, 0x61, 0x78, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x38,
	0x0a, 0x17, 0x6e, 0x6f, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x4d,
	0x69, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x17, 0x6e, 0x6f, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x4d, 0x69,
	0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1c, 0x0a, 0x09, 0x69, 0x73, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x69, 0x73, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x48, 0x0a, 0x08, 0x73, 0x75, 0x62, 0x66, 0x69, 0x65,
	0x6c, 0x64, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x57, 0x41, 0x57, 0x65, 0x62,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x69,
	0x6e, 0x67, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x2e, 0x53, 0x75, 0x62, 0x66, 0x69, 0x65, 0x6c,
	0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x73, 0x75, 0x62, 0x66, 0x69, 0x65, 0x6c, 0x64,
	0x1a, 0x5b, 0x0a, 0x0d, 0x53, 0x75, 0x62, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x03,
	0x6b, 0x65, 0x79, 0x12, 0x34, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x57, 0x41, 0x57, 0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x73, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x46, 0x69, 0x65,
	0x6c, 0x64, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x42, 0x27, 0x5a,
	0x25, 0x67, 0x6f, 0x2e, 0x6d, 0x61, 0x75, 0x2e, 0x66, 0x69, 0x2f, 0x77, 0x68, 0x61, 0x74, 0x73,
	0x6d, 0x65, 0x6f, 0x77, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x77, 0x61, 0x52, 0x65, 0x70,
	0x6f, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_waReporting_WAWebProtobufsReporting_proto_rawDescOnce sync.Once
	file_waReporting_WAWebProtobufsReporting_proto_rawDescData []byte
)

func file_waReporting_WAWebProtobufsReporting_proto_rawDescGZIP() []byte {
	file_waReporting_WAWebProtobufsReporting_proto_rawDescOnce.Do(func() {
		file_waReporting_WAWebProtobufsReporting_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_waReporting_WAWebProtobufsReporting_proto_rawDesc), len(file_waReporting_WAWebProtobufsReporting_proto_rawDesc)))
	})
	return file_waReporting_WAWebProtobufsReporting_proto_rawDescData
}

var file_waReporting_WAWebProtobufsReporting_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_waReporting_WAWebProtobufsReporting_proto_goTypes = []any{
	(*Reportable)(nil), // 0: WAWebProtobufsReporting.Reportable
	(*Config)(nil),     // 1: WAWebProtobufsReporting.Config
	(*Field)(nil),      // 2: WAWebProtobufsReporting.Field
	nil,                // 3: WAWebProtobufsReporting.Config.FieldEntry
	nil,                // 4: WAWebProtobufsReporting.Field.SubfieldEntry
}
var file_waReporting_WAWebProtobufsReporting_proto_depIdxs = []int32{
	3, // 0: WAWebProtobufsReporting.Config.field:type_name -> WAWebProtobufsReporting.Config.FieldEntry
	4, // 1: WAWebProtobufsReporting.Field.subfield:type_name -> WAWebProtobufsReporting.Field.SubfieldEntry
	2, // 2: WAWebProtobufsReporting.Config.FieldEntry.value:type_name -> WAWebProtobufsReporting.Field
	2, // 3: WAWebProtobufsReporting.Field.SubfieldEntry.value:type_name -> WAWebProtobufsReporting.Field
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_waReporting_WAWebProtobufsReporting_proto_init() }
func file_waReporting_WAWebProtobufsReporting_proto_init() {
	if File_waReporting_WAWebProtobufsReporting_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_waReporting_WAWebProtobufsReporting_proto_rawDesc), len(file_waReporting_WAWebProtobufsReporting_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_waReporting_WAWebProtobufsReporting_proto_goTypes,
		DependencyIndexes: file_waReporting_WAWebProtobufsReporting_proto_depIdxs,
		MessageInfos:      file_waReporting_WAWebProtobufsReporting_proto_msgTypes,
	}.Build()
	File_waReporting_WAWebProtobufsReporting_proto = out.File
	file_waReporting_WAWebProtobufsReporting_proto_goTypes = nil
	file_waReporting_WAWebProtobufsReporting_proto_depIdxs = nil
}
