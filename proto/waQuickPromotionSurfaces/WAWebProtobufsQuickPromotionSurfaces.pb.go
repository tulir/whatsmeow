// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.28.2
// source: waQuickPromotionSurfaces/WAWebProtobufsQuickPromotionSurfaces.proto

package waQuickPromotionSurfaces

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

type QP_FilterResult int32

const (
	QP_TRUE    QP_FilterResult = 1
	QP_FALSE   QP_FilterResult = 2
	QP_UNKNOWN QP_FilterResult = 3
)

// Enum value maps for QP_FilterResult.
var (
	QP_FilterResult_name = map[int32]string{
		1: "TRUE",
		2: "FALSE",
		3: "UNKNOWN",
	}
	QP_FilterResult_value = map[string]int32{
		"TRUE":    1,
		"FALSE":   2,
		"UNKNOWN": 3,
	}
)

func (x QP_FilterResult) Enum() *QP_FilterResult {
	p := new(QP_FilterResult)
	*p = x
	return p
}

func (x QP_FilterResult) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (QP_FilterResult) Descriptor() protoreflect.EnumDescriptor {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes[0].Descriptor()
}

func (QP_FilterResult) Type() protoreflect.EnumType {
	return &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes[0]
}

func (x QP_FilterResult) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Do not use.
func (x *QP_FilterResult) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = QP_FilterResult(num)
	return nil
}

// Deprecated: Use QP_FilterResult.Descriptor instead.
func (QP_FilterResult) EnumDescriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0, 0}
}

type QP_FilterClientNotSupportedConfig int32

const (
	QP_PASS_BY_DEFAULT QP_FilterClientNotSupportedConfig = 1
	QP_FAIL_BY_DEFAULT QP_FilterClientNotSupportedConfig = 2
)

// Enum value maps for QP_FilterClientNotSupportedConfig.
var (
	QP_FilterClientNotSupportedConfig_name = map[int32]string{
		1: "PASS_BY_DEFAULT",
		2: "FAIL_BY_DEFAULT",
	}
	QP_FilterClientNotSupportedConfig_value = map[string]int32{
		"PASS_BY_DEFAULT": 1,
		"FAIL_BY_DEFAULT": 2,
	}
)

func (x QP_FilterClientNotSupportedConfig) Enum() *QP_FilterClientNotSupportedConfig {
	p := new(QP_FilterClientNotSupportedConfig)
	*p = x
	return p
}

func (x QP_FilterClientNotSupportedConfig) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (QP_FilterClientNotSupportedConfig) Descriptor() protoreflect.EnumDescriptor {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes[1].Descriptor()
}

func (QP_FilterClientNotSupportedConfig) Type() protoreflect.EnumType {
	return &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes[1]
}

func (x QP_FilterClientNotSupportedConfig) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Do not use.
func (x *QP_FilterClientNotSupportedConfig) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = QP_FilterClientNotSupportedConfig(num)
	return nil
}

// Deprecated: Use QP_FilterClientNotSupportedConfig.Descriptor instead.
func (QP_FilterClientNotSupportedConfig) EnumDescriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0, 1}
}

type QP_ClauseType int32

const (
	QP_AND QP_ClauseType = 1
	QP_OR  QP_ClauseType = 2
	QP_NOR QP_ClauseType = 3
)

// Enum value maps for QP_ClauseType.
var (
	QP_ClauseType_name = map[int32]string{
		1: "AND",
		2: "OR",
		3: "NOR",
	}
	QP_ClauseType_value = map[string]int32{
		"AND": 1,
		"OR":  2,
		"NOR": 3,
	}
)

func (x QP_ClauseType) Enum() *QP_ClauseType {
	p := new(QP_ClauseType)
	*p = x
	return p
}

func (x QP_ClauseType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (QP_ClauseType) Descriptor() protoreflect.EnumDescriptor {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes[2].Descriptor()
}

func (QP_ClauseType) Type() protoreflect.EnumType {
	return &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes[2]
}

func (x QP_ClauseType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Do not use.
func (x *QP_ClauseType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = QP_ClauseType(num)
	return nil
}

// Deprecated: Use QP_ClauseType.Descriptor instead.
func (QP_ClauseType) EnumDescriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0, 2}
}

type QP struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *QP) Reset() {
	*x = QP{}
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QP) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QP) ProtoMessage() {}

func (x *QP) ProtoReflect() protoreflect.Message {
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QP.ProtoReflect.Descriptor instead.
func (*QP) Descriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0}
}

type QP_FilterClause struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ClauseType    *QP_ClauseType         `protobuf:"varint,1,req,name=clauseType,enum=WAWebProtobufsQuickPromotionSurfaces.QP_ClauseType" json:"clauseType,omitempty"`
	Clauses       []*QP_FilterClause     `protobuf:"bytes,2,rep,name=clauses" json:"clauses,omitempty"`
	Filters       []*QP_Filter           `protobuf:"bytes,3,rep,name=filters" json:"filters,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *QP_FilterClause) Reset() {
	*x = QP_FilterClause{}
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QP_FilterClause) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QP_FilterClause) ProtoMessage() {}

func (x *QP_FilterClause) ProtoReflect() protoreflect.Message {
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QP_FilterClause.ProtoReflect.Descriptor instead.
func (*QP_FilterClause) Descriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0, 0}
}

func (x *QP_FilterClause) GetClauseType() QP_ClauseType {
	if x != nil && x.ClauseType != nil {
		return *x.ClauseType
	}
	return QP_AND
}

func (x *QP_FilterClause) GetClauses() []*QP_FilterClause {
	if x != nil {
		return x.Clauses
	}
	return nil
}

func (x *QP_FilterClause) GetFilters() []*QP_Filter {
	if x != nil {
		return x.Filters
	}
	return nil
}

type QP_Filter struct {
	state                    protoimpl.MessageState             `protogen:"open.v1"`
	FilterName               *string                            `protobuf:"bytes,1,req,name=filterName" json:"filterName,omitempty"`
	Parameters               []*QP_FilterParameters             `protobuf:"bytes,2,rep,name=parameters" json:"parameters,omitempty"`
	FilterResult             *QP_FilterResult                   `protobuf:"varint,3,opt,name=filterResult,enum=WAWebProtobufsQuickPromotionSurfaces.QP_FilterResult" json:"filterResult,omitempty"`
	ClientNotSupportedConfig *QP_FilterClientNotSupportedConfig `protobuf:"varint,4,req,name=clientNotSupportedConfig,enum=WAWebProtobufsQuickPromotionSurfaces.QP_FilterClientNotSupportedConfig" json:"clientNotSupportedConfig,omitempty"`
	unknownFields            protoimpl.UnknownFields
	sizeCache                protoimpl.SizeCache
}

func (x *QP_Filter) Reset() {
	*x = QP_Filter{}
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QP_Filter) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QP_Filter) ProtoMessage() {}

func (x *QP_Filter) ProtoReflect() protoreflect.Message {
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QP_Filter.ProtoReflect.Descriptor instead.
func (*QP_Filter) Descriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0, 1}
}

func (x *QP_Filter) GetFilterName() string {
	if x != nil && x.FilterName != nil {
		return *x.FilterName
	}
	return ""
}

func (x *QP_Filter) GetParameters() []*QP_FilterParameters {
	if x != nil {
		return x.Parameters
	}
	return nil
}

func (x *QP_Filter) GetFilterResult() QP_FilterResult {
	if x != nil && x.FilterResult != nil {
		return *x.FilterResult
	}
	return QP_TRUE
}

func (x *QP_Filter) GetClientNotSupportedConfig() QP_FilterClientNotSupportedConfig {
	if x != nil && x.ClientNotSupportedConfig != nil {
		return *x.ClientNotSupportedConfig
	}
	return QP_PASS_BY_DEFAULT
}

type QP_FilterParameters struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Key           *string                `protobuf:"bytes,1,opt,name=key" json:"key,omitempty"`
	Value         *string                `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *QP_FilterParameters) Reset() {
	*x = QP_FilterParameters{}
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QP_FilterParameters) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QP_FilterParameters) ProtoMessage() {}

func (x *QP_FilterParameters) ProtoReflect() protoreflect.Message {
	mi := &file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QP_FilterParameters.ProtoReflect.Descriptor instead.
func (*QP_FilterParameters) Descriptor() ([]byte, []int) {
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP(), []int{0, 2}
}

func (x *QP_FilterParameters) GetKey() string {
	if x != nil && x.Key != nil {
		return *x.Key
	}
	return ""
}

func (x *QP_FilterParameters) GetValue() string {
	if x != nil && x.Value != nil {
		return *x.Value
	}
	return ""
}

var File_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto protoreflect.FileDescriptor

var file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDesc = string([]byte{
	0x0a, 0x43, 0x77, 0x61, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72, 0x6f, 0x6d, 0x6f, 0x74, 0x69,
	0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x2f, 0x57, 0x41, 0x57, 0x65, 0x62,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72,
	0x6f, 0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x24, 0x57, 0x41, 0x57, 0x65, 0x62, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72, 0x6f, 0x6d, 0x6f, 0x74,
	0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x22, 0xcf, 0x06, 0x0a, 0x02,
	0x51, 0x50, 0x1a, 0xff, 0x01, 0x0a, 0x0c, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x43, 0x6c, 0x61,
	0x75, 0x73, 0x65, 0x12, 0x53, 0x0a, 0x0a, 0x63, 0x6c, 0x61, 0x75, 0x73, 0x65, 0x54, 0x79, 0x70,
	0x65, 0x18, 0x01, 0x20, 0x02, 0x28, 0x0e, 0x32, 0x33, 0x2e, 0x57, 0x41, 0x57, 0x65, 0x62, 0x50,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72, 0x6f,
	0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x2e, 0x51,
	0x50, 0x2e, 0x43, 0x6c, 0x61, 0x75, 0x73, 0x65, 0x54, 0x79, 0x70, 0x65, 0x52, 0x0a, 0x63, 0x6c,
	0x61, 0x75, 0x73, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x4f, 0x0a, 0x07, 0x63, 0x6c, 0x61, 0x75,
	0x73, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x35, 0x2e, 0x57, 0x41, 0x57, 0x65,
	0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50,
	0x72, 0x6f, 0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73,
	0x2e, 0x51, 0x50, 0x2e, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x43, 0x6c, 0x61, 0x75, 0x73, 0x65,
	0x52, 0x07, 0x63, 0x6c, 0x61, 0x75, 0x73, 0x65, 0x73, 0x12, 0x49, 0x0a, 0x07, 0x66, 0x69, 0x6c,
	0x74, 0x65, 0x72, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2f, 0x2e, 0x57, 0x41, 0x57,
	0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b,
	0x50, 0x72, 0x6f, 0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65,
	0x73, 0x2e, 0x51, 0x50, 0x2e, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52, 0x07, 0x66, 0x69, 0x6c,
	0x74, 0x65, 0x72, 0x73, 0x1a, 0xe4, 0x02, 0x0a, 0x06, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x12,
	0x1e, 0x0a, 0x0a, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x02, 0x28, 0x09, 0x52, 0x0a, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12,
	0x59, 0x0a, 0x0a, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x39, 0x2e, 0x57, 0x41, 0x57, 0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72, 0x6f, 0x6d, 0x6f, 0x74, 0x69,
	0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x2e, 0x51, 0x50, 0x2e, 0x46, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x52, 0x0a,
	0x70, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x12, 0x59, 0x0a, 0x0c, 0x66, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x35, 0x2e, 0x57, 0x41, 0x57, 0x65, 0x62, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72, 0x6f, 0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53,
	0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x2e, 0x51, 0x50, 0x2e, 0x46, 0x69, 0x6c, 0x74, 0x65,
	0x72, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x0c, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x83, 0x01, 0x0a, 0x18, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x4e, 0x6f, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x18, 0x04, 0x20, 0x02, 0x28, 0x0e, 0x32, 0x47, 0x2e, 0x57, 0x41, 0x57, 0x65, 0x62,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x73, 0x51, 0x75, 0x69, 0x63, 0x6b, 0x50, 0x72,
	0x6f, 0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61, 0x63, 0x65, 0x73, 0x2e,
	0x51, 0x50, 0x2e, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4e,
	0x6f, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x52, 0x18, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4e, 0x6f, 0x74, 0x53, 0x75, 0x70, 0x70,
	0x6f, 0x72, 0x74, 0x65, 0x64, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x1a, 0x3a, 0x0a, 0x10, 0x46,
	0x69, 0x6c, 0x74, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x30, 0x0a, 0x0c, 0x46, 0x69, 0x6c, 0x74, 0x65,
	0x72, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x08, 0x0a, 0x04, 0x54, 0x52, 0x55, 0x45, 0x10,
	0x01, 0x12, 0x09, 0x0a, 0x05, 0x46, 0x41, 0x4c, 0x53, 0x45, 0x10, 0x02, 0x12, 0x0b, 0x0a, 0x07,
	0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x03, 0x22, 0x4a, 0x0a, 0x1e, 0x46, 0x69, 0x6c,
	0x74, 0x65, 0x72, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4e, 0x6f, 0x74, 0x53, 0x75, 0x70, 0x70,
	0x6f, 0x72, 0x74, 0x65, 0x64, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x13, 0x0a, 0x0f, 0x50,
	0x41, 0x53, 0x53, 0x5f, 0x42, 0x59, 0x5f, 0x44, 0x45, 0x46, 0x41, 0x55, 0x4c, 0x54, 0x10, 0x01,
	0x12, 0x13, 0x0a, 0x0f, 0x46, 0x41, 0x49, 0x4c, 0x5f, 0x42, 0x59, 0x5f, 0x44, 0x45, 0x46, 0x41,
	0x55, 0x4c, 0x54, 0x10, 0x02, 0x22, 0x26, 0x0a, 0x0a, 0x43, 0x6c, 0x61, 0x75, 0x73, 0x65, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x07, 0x0a, 0x03, 0x41, 0x4e, 0x44, 0x10, 0x01, 0x12, 0x06, 0x0a, 0x02,
	0x4f, 0x52, 0x10, 0x02, 0x12, 0x07, 0x0a, 0x03, 0x4e, 0x4f, 0x52, 0x10, 0x03, 0x42, 0x34, 0x5a,
	0x32, 0x67, 0x6f, 0x2e, 0x6d, 0x61, 0x75, 0x2e, 0x66, 0x69, 0x2f, 0x77, 0x68, 0x61, 0x74, 0x73,
	0x6d, 0x65, 0x6f, 0x77, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x77, 0x61, 0x51, 0x75, 0x69,
	0x63, 0x6b, 0x50, 0x72, 0x6f, 0x6d, 0x6f, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x75, 0x72, 0x66, 0x61,
	0x63, 0x65, 0x73,
})

var (
	file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescOnce sync.Once
	file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescData []byte
)

func file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescGZIP() []byte {
	file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescOnce.Do(func() {
		file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDesc), len(file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDesc)))
	})
	return file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDescData
}

var file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_goTypes = []any{
	(QP_FilterResult)(0),                   // 0: WAWebProtobufsQuickPromotionSurfaces.QP.FilterResult
	(QP_FilterClientNotSupportedConfig)(0), // 1: WAWebProtobufsQuickPromotionSurfaces.QP.FilterClientNotSupportedConfig
	(QP_ClauseType)(0),                     // 2: WAWebProtobufsQuickPromotionSurfaces.QP.ClauseType
	(*QP)(nil),                             // 3: WAWebProtobufsQuickPromotionSurfaces.QP
	(*QP_FilterClause)(nil),                // 4: WAWebProtobufsQuickPromotionSurfaces.QP.FilterClause
	(*QP_Filter)(nil),                      // 5: WAWebProtobufsQuickPromotionSurfaces.QP.Filter
	(*QP_FilterParameters)(nil),            // 6: WAWebProtobufsQuickPromotionSurfaces.QP.FilterParameters
}
var file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_depIdxs = []int32{
	2, // 0: WAWebProtobufsQuickPromotionSurfaces.QP.FilterClause.clauseType:type_name -> WAWebProtobufsQuickPromotionSurfaces.QP.ClauseType
	4, // 1: WAWebProtobufsQuickPromotionSurfaces.QP.FilterClause.clauses:type_name -> WAWebProtobufsQuickPromotionSurfaces.QP.FilterClause
	5, // 2: WAWebProtobufsQuickPromotionSurfaces.QP.FilterClause.filters:type_name -> WAWebProtobufsQuickPromotionSurfaces.QP.Filter
	6, // 3: WAWebProtobufsQuickPromotionSurfaces.QP.Filter.parameters:type_name -> WAWebProtobufsQuickPromotionSurfaces.QP.FilterParameters
	0, // 4: WAWebProtobufsQuickPromotionSurfaces.QP.Filter.filterResult:type_name -> WAWebProtobufsQuickPromotionSurfaces.QP.FilterResult
	1, // 5: WAWebProtobufsQuickPromotionSurfaces.QP.Filter.clientNotSupportedConfig:type_name -> WAWebProtobufsQuickPromotionSurfaces.QP.FilterClientNotSupportedConfig
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_init() }
func file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_init() {
	if File_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDesc), len(file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_rawDesc)),
			NumEnums:      3,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_goTypes,
		DependencyIndexes: file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_depIdxs,
		EnumInfos:         file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_enumTypes,
		MessageInfos:      file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_msgTypes,
	}.Build()
	File_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto = out.File
	file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_goTypes = nil
	file_waQuickPromotionSurfaces_WAWebProtobufsQuickPromotionSurfaces_proto_depIdxs = nil
}
