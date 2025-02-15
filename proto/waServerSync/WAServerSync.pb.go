// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.28.2
// source: waServerSync/WAServerSync.proto

package waServerSync

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

type SyncdMutation_SyncdOperation int32

const (
	SyncdMutation_SET    SyncdMutation_SyncdOperation = 0
	SyncdMutation_REMOVE SyncdMutation_SyncdOperation = 1
)

// Enum value maps for SyncdMutation_SyncdOperation.
var (
	SyncdMutation_SyncdOperation_name = map[int32]string{
		0: "SET",
		1: "REMOVE",
	}
	SyncdMutation_SyncdOperation_value = map[string]int32{
		"SET":    0,
		"REMOVE": 1,
	}
)

func (x SyncdMutation_SyncdOperation) Enum() *SyncdMutation_SyncdOperation {
	p := new(SyncdMutation_SyncdOperation)
	*p = x
	return p
}

func (x SyncdMutation_SyncdOperation) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SyncdMutation_SyncdOperation) Descriptor() protoreflect.EnumDescriptor {
	return file_waServerSync_WAServerSync_proto_enumTypes[0].Descriptor()
}

func (SyncdMutation_SyncdOperation) Type() protoreflect.EnumType {
	return &file_waServerSync_WAServerSync_proto_enumTypes[0]
}

func (x SyncdMutation_SyncdOperation) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Do not use.
func (x *SyncdMutation_SyncdOperation) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = SyncdMutation_SyncdOperation(num)
	return nil
}

// Deprecated: Use SyncdMutation_SyncdOperation.Descriptor instead.
func (SyncdMutation_SyncdOperation) EnumDescriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{0, 0}
}

type SyncdMutation struct {
	state         protoimpl.MessageState        `protogen:"open.v1"`
	Operation     *SyncdMutation_SyncdOperation `protobuf:"varint,1,opt,name=operation,enum=WAServerSync.SyncdMutation_SyncdOperation" json:"operation,omitempty"`
	Record        *SyncdRecord                  `protobuf:"bytes,2,opt,name=record" json:"record,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdMutation) Reset() {
	*x = SyncdMutation{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdMutation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdMutation) ProtoMessage() {}

func (x *SyncdMutation) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdMutation.ProtoReflect.Descriptor instead.
func (*SyncdMutation) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{0}
}

func (x *SyncdMutation) GetOperation() SyncdMutation_SyncdOperation {
	if x != nil && x.Operation != nil {
		return *x.Operation
	}
	return SyncdMutation_SET
}

func (x *SyncdMutation) GetRecord() *SyncdRecord {
	if x != nil {
		return x.Record
	}
	return nil
}

type SyncdVersion struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Version       *uint64                `protobuf:"varint,1,opt,name=version" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdVersion) Reset() {
	*x = SyncdVersion{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdVersion) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdVersion) ProtoMessage() {}

func (x *SyncdVersion) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdVersion.ProtoReflect.Descriptor instead.
func (*SyncdVersion) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{1}
}

func (x *SyncdVersion) GetVersion() uint64 {
	if x != nil && x.Version != nil {
		return *x.Version
	}
	return 0
}

type ExitCode struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          *uint64                `protobuf:"varint,1,opt,name=code" json:"code,omitempty"`
	Text          *string                `protobuf:"bytes,2,opt,name=text" json:"text,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExitCode) Reset() {
	*x = ExitCode{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExitCode) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExitCode) ProtoMessage() {}

func (x *ExitCode) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExitCode.ProtoReflect.Descriptor instead.
func (*ExitCode) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{2}
}

func (x *ExitCode) GetCode() uint64 {
	if x != nil && x.Code != nil {
		return *x.Code
	}
	return 0
}

func (x *ExitCode) GetText() string {
	if x != nil && x.Text != nil {
		return *x.Text
	}
	return ""
}

type SyncdIndex struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Blob          []byte                 `protobuf:"bytes,1,opt,name=blob" json:"blob,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdIndex) Reset() {
	*x = SyncdIndex{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdIndex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdIndex) ProtoMessage() {}

func (x *SyncdIndex) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdIndex.ProtoReflect.Descriptor instead.
func (*SyncdIndex) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{3}
}

func (x *SyncdIndex) GetBlob() []byte {
	if x != nil {
		return x.Blob
	}
	return nil
}

type SyncdValue struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Blob          []byte                 `protobuf:"bytes,1,opt,name=blob" json:"blob,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdValue) Reset() {
	*x = SyncdValue{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdValue) ProtoMessage() {}

func (x *SyncdValue) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdValue.ProtoReflect.Descriptor instead.
func (*SyncdValue) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{4}
}

func (x *SyncdValue) GetBlob() []byte {
	if x != nil {
		return x.Blob
	}
	return nil
}

type KeyId struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ID            []byte                 `protobuf:"bytes,1,opt,name=ID" json:"ID,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *KeyId) Reset() {
	*x = KeyId{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *KeyId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KeyId) ProtoMessage() {}

func (x *KeyId) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KeyId.ProtoReflect.Descriptor instead.
func (*KeyId) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{5}
}

func (x *KeyId) GetID() []byte {
	if x != nil {
		return x.ID
	}
	return nil
}

type SyncdRecord struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Index         *SyncdIndex            `protobuf:"bytes,1,opt,name=index" json:"index,omitempty"`
	Value         *SyncdValue            `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
	KeyID         *KeyId                 `protobuf:"bytes,3,opt,name=keyID" json:"keyID,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdRecord) Reset() {
	*x = SyncdRecord{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdRecord) ProtoMessage() {}

func (x *SyncdRecord) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdRecord.ProtoReflect.Descriptor instead.
func (*SyncdRecord) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{6}
}

func (x *SyncdRecord) GetIndex() *SyncdIndex {
	if x != nil {
		return x.Index
	}
	return nil
}

func (x *SyncdRecord) GetValue() *SyncdValue {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *SyncdRecord) GetKeyID() *KeyId {
	if x != nil {
		return x.KeyID
	}
	return nil
}

type ExternalBlobReference struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	MediaKey      []byte                 `protobuf:"bytes,1,opt,name=mediaKey" json:"mediaKey,omitempty"`
	DirectPath    *string                `protobuf:"bytes,2,opt,name=directPath" json:"directPath,omitempty"`
	Handle        *string                `protobuf:"bytes,3,opt,name=handle" json:"handle,omitempty"`
	FileSizeBytes *uint64                `protobuf:"varint,4,opt,name=fileSizeBytes" json:"fileSizeBytes,omitempty"`
	FileSHA256    []byte                 `protobuf:"bytes,5,opt,name=fileSHA256" json:"fileSHA256,omitempty"`
	FileEncSHA256 []byte                 `protobuf:"bytes,6,opt,name=fileEncSHA256" json:"fileEncSHA256,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExternalBlobReference) Reset() {
	*x = ExternalBlobReference{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExternalBlobReference) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExternalBlobReference) ProtoMessage() {}

func (x *ExternalBlobReference) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExternalBlobReference.ProtoReflect.Descriptor instead.
func (*ExternalBlobReference) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{7}
}

func (x *ExternalBlobReference) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *ExternalBlobReference) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *ExternalBlobReference) GetHandle() string {
	if x != nil && x.Handle != nil {
		return *x.Handle
	}
	return ""
}

func (x *ExternalBlobReference) GetFileSizeBytes() uint64 {
	if x != nil && x.FileSizeBytes != nil {
		return *x.FileSizeBytes
	}
	return 0
}

func (x *ExternalBlobReference) GetFileSHA256() []byte {
	if x != nil {
		return x.FileSHA256
	}
	return nil
}

func (x *ExternalBlobReference) GetFileEncSHA256() []byte {
	if x != nil {
		return x.FileEncSHA256
	}
	return nil
}

type SyncdSnapshot struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Version       *SyncdVersion          `protobuf:"bytes,1,opt,name=version" json:"version,omitempty"`
	Records       []*SyncdRecord         `protobuf:"bytes,2,rep,name=records" json:"records,omitempty"`
	Mac           []byte                 `protobuf:"bytes,3,opt,name=mac" json:"mac,omitempty"`
	KeyID         *KeyId                 `protobuf:"bytes,4,opt,name=keyID" json:"keyID,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdSnapshot) Reset() {
	*x = SyncdSnapshot{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdSnapshot) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdSnapshot) ProtoMessage() {}

func (x *SyncdSnapshot) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdSnapshot.ProtoReflect.Descriptor instead.
func (*SyncdSnapshot) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{8}
}

func (x *SyncdSnapshot) GetVersion() *SyncdVersion {
	if x != nil {
		return x.Version
	}
	return nil
}

func (x *SyncdSnapshot) GetRecords() []*SyncdRecord {
	if x != nil {
		return x.Records
	}
	return nil
}

func (x *SyncdSnapshot) GetMac() []byte {
	if x != nil {
		return x.Mac
	}
	return nil
}

func (x *SyncdSnapshot) GetKeyID() *KeyId {
	if x != nil {
		return x.KeyID
	}
	return nil
}

type SyncdMutations struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Mutations     []*SyncdMutation       `protobuf:"bytes,1,rep,name=mutations" json:"mutations,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdMutations) Reset() {
	*x = SyncdMutations{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdMutations) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdMutations) ProtoMessage() {}

func (x *SyncdMutations) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdMutations.ProtoReflect.Descriptor instead.
func (*SyncdMutations) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{9}
}

func (x *SyncdMutations) GetMutations() []*SyncdMutation {
	if x != nil {
		return x.Mutations
	}
	return nil
}

type SyncdPatch struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Version           *SyncdVersion          `protobuf:"bytes,1,opt,name=version" json:"version,omitempty"`
	Mutations         []*SyncdMutation       `protobuf:"bytes,2,rep,name=mutations" json:"mutations,omitempty"`
	ExternalMutations *ExternalBlobReference `protobuf:"bytes,3,opt,name=externalMutations" json:"externalMutations,omitempty"`
	SnapshotMAC       []byte                 `protobuf:"bytes,4,opt,name=snapshotMAC" json:"snapshotMAC,omitempty"`
	PatchMAC          []byte                 `protobuf:"bytes,5,opt,name=patchMAC" json:"patchMAC,omitempty"`
	KeyID             *KeyId                 `protobuf:"bytes,6,opt,name=keyID" json:"keyID,omitempty"`
	ExitCode          *ExitCode              `protobuf:"bytes,7,opt,name=exitCode" json:"exitCode,omitempty"`
	DeviceIndex       *uint32                `protobuf:"varint,8,opt,name=deviceIndex" json:"deviceIndex,omitempty"`
	ClientDebugData   []byte                 `protobuf:"bytes,9,opt,name=clientDebugData" json:"clientDebugData,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *SyncdPatch) Reset() {
	*x = SyncdPatch{}
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdPatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdPatch) ProtoMessage() {}

func (x *SyncdPatch) ProtoReflect() protoreflect.Message {
	mi := &file_waServerSync_WAServerSync_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncdPatch.ProtoReflect.Descriptor instead.
func (*SyncdPatch) Descriptor() ([]byte, []int) {
	return file_waServerSync_WAServerSync_proto_rawDescGZIP(), []int{10}
}

func (x *SyncdPatch) GetVersion() *SyncdVersion {
	if x != nil {
		return x.Version
	}
	return nil
}

func (x *SyncdPatch) GetMutations() []*SyncdMutation {
	if x != nil {
		return x.Mutations
	}
	return nil
}

func (x *SyncdPatch) GetExternalMutations() *ExternalBlobReference {
	if x != nil {
		return x.ExternalMutations
	}
	return nil
}

func (x *SyncdPatch) GetSnapshotMAC() []byte {
	if x != nil {
		return x.SnapshotMAC
	}
	return nil
}

func (x *SyncdPatch) GetPatchMAC() []byte {
	if x != nil {
		return x.PatchMAC
	}
	return nil
}

func (x *SyncdPatch) GetKeyID() *KeyId {
	if x != nil {
		return x.KeyID
	}
	return nil
}

func (x *SyncdPatch) GetExitCode() *ExitCode {
	if x != nil {
		return x.ExitCode
	}
	return nil
}

func (x *SyncdPatch) GetDeviceIndex() uint32 {
	if x != nil && x.DeviceIndex != nil {
		return *x.DeviceIndex
	}
	return 0
}

func (x *SyncdPatch) GetClientDebugData() []byte {
	if x != nil {
		return x.ClientDebugData
	}
	return nil
}

var File_waServerSync_WAServerSync_proto protoreflect.FileDescriptor

var file_waServerSync_WAServerSync_proto_rawDesc = string([]byte{
	0x0a, 0x1f, 0x77, 0x61, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2f, 0x57,
	0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x0c, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x22,
	0xb3, 0x01, 0x0a, 0x0d, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x48, 0x0a, 0x09, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0e, 0x32, 0x2a, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53,
	0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x09, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x31, 0x0a, 0x06, 0x72,
	0x65, 0x63, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x57, 0x41,
	0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x64,
	0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x52, 0x06, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x22, 0x25,
	0x0a, 0x0e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x07, 0x0a, 0x03, 0x53, 0x45, 0x54, 0x10, 0x00, 0x12, 0x0a, 0x0a, 0x06, 0x52, 0x45, 0x4d,
	0x4f, 0x56, 0x45, 0x10, 0x01, 0x22, 0x28, 0x0a, 0x0c, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x56, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x22,
	0x32, 0x0a, 0x08, 0x45, 0x78, 0x69, 0x74, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x63,
	0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12,
	0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74,
	0x65, 0x78, 0x74, 0x22, 0x20, 0x0a, 0x0a, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x49, 0x6e, 0x64, 0x65,
	0x78, 0x12, 0x12, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x20, 0x0a, 0x0a, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x17, 0x0a, 0x05, 0x4b, 0x65, 0x79, 0x49, 0x64,
	0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x49, 0x44,
	0x22, 0x98, 0x01, 0x0a, 0x0b, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64,
	0x12, 0x2e, 0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x18, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x53,
	0x79, 0x6e, 0x63, 0x64, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78,
	0x12, 0x2e, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x18, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x53,
	0x79, 0x6e, 0x63, 0x64, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x12, 0x29, 0x0a, 0x05, 0x6b, 0x65, 0x79, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x13, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x4b,
	0x65, 0x79, 0x49, 0x64, 0x52, 0x05, 0x6b, 0x65, 0x79, 0x49, 0x44, 0x22, 0xd7, 0x01, 0x0a, 0x15,
	0x45, 0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x65, 0x66, 0x65,
	0x72, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x4b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x4b, 0x65,
	0x79, 0x12, 0x1e, 0x0a, 0x0a, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x50, 0x61, 0x74, 0x68, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x50, 0x61, 0x74,
	0x68, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x68, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x12, 0x24, 0x0a, 0x0d, 0x66, 0x69, 0x6c,
	0x65, 0x53, 0x69, 0x7a, 0x65, 0x42, 0x79, 0x74, 0x65, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x0d, 0x66, 0x69, 0x6c, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x42, 0x79, 0x74, 0x65, 0x73, 0x12,
	0x1e, 0x0a, 0x0a, 0x66, 0x69, 0x6c, 0x65, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x0a, 0x66, 0x69, 0x6c, 0x65, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x12,
	0x24, 0x0a, 0x0d, 0x66, 0x69, 0x6c, 0x65, 0x45, 0x6e, 0x63, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0d, 0x66, 0x69, 0x6c, 0x65, 0x45, 0x6e, 0x63, 0x53,
	0x48, 0x41, 0x32, 0x35, 0x36, 0x22, 0xb7, 0x01, 0x0a, 0x0d, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x53,
	0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x12, 0x34, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69,
	0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72,
	0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x56, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x33, 0x0a,
	0x07, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x19,
	0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79,
	0x6e, 0x63, 0x64, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x52, 0x07, 0x72, 0x65, 0x63, 0x6f, 0x72,
	0x64, 0x73, 0x12, 0x10, 0x0a, 0x03, 0x6d, 0x61, 0x63, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x03, 0x6d, 0x61, 0x63, 0x12, 0x29, 0x0a, 0x05, 0x6b, 0x65, 0x79, 0x49, 0x44, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79,
	0x6e, 0x63, 0x2e, 0x4b, 0x65, 0x79, 0x49, 0x64, 0x52, 0x05, 0x6b, 0x65, 0x79, 0x49, 0x44, 0x22,
	0x4b, 0x0a, 0x0e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x12, 0x39, 0x0a, 0x09, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53,
	0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x09, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0xb9, 0x03, 0x0a,
	0x0a, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x50, 0x61, 0x74, 0x63, 0x68, 0x12, 0x34, 0x0a, 0x07, 0x76,
	0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x57,
	0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79, 0x6e, 0x63,
	0x64, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x12, 0x39, 0x0a, 0x09, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53,
	0x79, 0x6e, 0x63, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x64, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x09, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x51, 0x0a, 0x11,
	0x65, 0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x57, 0x41, 0x53, 0x65, 0x72, 0x76,
	0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x45, 0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x42,
	0x6c, 0x6f, 0x62, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x11, 0x65, 0x78,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12,
	0x20, 0x0a, 0x0b, 0x73, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x4d, 0x41, 0x43, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x73, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x4d, 0x41,
	0x43, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x74, 0x63, 0x68, 0x4d, 0x41, 0x43, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x08, 0x70, 0x61, 0x74, 0x63, 0x68, 0x4d, 0x41, 0x43, 0x12, 0x29, 0x0a,
	0x05, 0x6b, 0x65, 0x79, 0x49, 0x44, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x57,
	0x41, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x4b, 0x65, 0x79, 0x49,
	0x64, 0x52, 0x05, 0x6b, 0x65, 0x79, 0x49, 0x44, 0x12, 0x32, 0x0a, 0x08, 0x65, 0x78, 0x69, 0x74,
	0x43, 0x6f, 0x64, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x57, 0x41, 0x53,
	0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79, 0x6e, 0x63, 0x2e, 0x45, 0x78, 0x69, 0x74, 0x43, 0x6f,
	0x64, 0x65, 0x52, 0x08, 0x65, 0x78, 0x69, 0x74, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x20, 0x0a, 0x0b,
	0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x0b, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x28,
	0x0a, 0x0f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x44, 0x65, 0x62, 0x75, 0x67, 0x44, 0x61, 0x74,
	0x61, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x44,
	0x65, 0x62, 0x75, 0x67, 0x44, 0x61, 0x74, 0x61, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x6f, 0x2e, 0x6d,
	0x61, 0x75, 0x2e, 0x66, 0x69, 0x2f, 0x77, 0x68, 0x61, 0x74, 0x73, 0x6d, 0x65, 0x6f, 0x77, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x77, 0x61, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x79,
	0x6e, 0x63,
})

var (
	file_waServerSync_WAServerSync_proto_rawDescOnce sync.Once
	file_waServerSync_WAServerSync_proto_rawDescData []byte
)

func file_waServerSync_WAServerSync_proto_rawDescGZIP() []byte {
	file_waServerSync_WAServerSync_proto_rawDescOnce.Do(func() {
		file_waServerSync_WAServerSync_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_waServerSync_WAServerSync_proto_rawDesc), len(file_waServerSync_WAServerSync_proto_rawDesc)))
	})
	return file_waServerSync_WAServerSync_proto_rawDescData
}

var file_waServerSync_WAServerSync_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_waServerSync_WAServerSync_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_waServerSync_WAServerSync_proto_goTypes = []any{
	(SyncdMutation_SyncdOperation)(0), // 0: WAServerSync.SyncdMutation.SyncdOperation
	(*SyncdMutation)(nil),             // 1: WAServerSync.SyncdMutation
	(*SyncdVersion)(nil),              // 2: WAServerSync.SyncdVersion
	(*ExitCode)(nil),                  // 3: WAServerSync.ExitCode
	(*SyncdIndex)(nil),                // 4: WAServerSync.SyncdIndex
	(*SyncdValue)(nil),                // 5: WAServerSync.SyncdValue
	(*KeyId)(nil),                     // 6: WAServerSync.KeyId
	(*SyncdRecord)(nil),               // 7: WAServerSync.SyncdRecord
	(*ExternalBlobReference)(nil),     // 8: WAServerSync.ExternalBlobReference
	(*SyncdSnapshot)(nil),             // 9: WAServerSync.SyncdSnapshot
	(*SyncdMutations)(nil),            // 10: WAServerSync.SyncdMutations
	(*SyncdPatch)(nil),                // 11: WAServerSync.SyncdPatch
}
var file_waServerSync_WAServerSync_proto_depIdxs = []int32{
	0,  // 0: WAServerSync.SyncdMutation.operation:type_name -> WAServerSync.SyncdMutation.SyncdOperation
	7,  // 1: WAServerSync.SyncdMutation.record:type_name -> WAServerSync.SyncdRecord
	4,  // 2: WAServerSync.SyncdRecord.index:type_name -> WAServerSync.SyncdIndex
	5,  // 3: WAServerSync.SyncdRecord.value:type_name -> WAServerSync.SyncdValue
	6,  // 4: WAServerSync.SyncdRecord.keyID:type_name -> WAServerSync.KeyId
	2,  // 5: WAServerSync.SyncdSnapshot.version:type_name -> WAServerSync.SyncdVersion
	7,  // 6: WAServerSync.SyncdSnapshot.records:type_name -> WAServerSync.SyncdRecord
	6,  // 7: WAServerSync.SyncdSnapshot.keyID:type_name -> WAServerSync.KeyId
	1,  // 8: WAServerSync.SyncdMutations.mutations:type_name -> WAServerSync.SyncdMutation
	2,  // 9: WAServerSync.SyncdPatch.version:type_name -> WAServerSync.SyncdVersion
	1,  // 10: WAServerSync.SyncdPatch.mutations:type_name -> WAServerSync.SyncdMutation
	8,  // 11: WAServerSync.SyncdPatch.externalMutations:type_name -> WAServerSync.ExternalBlobReference
	6,  // 12: WAServerSync.SyncdPatch.keyID:type_name -> WAServerSync.KeyId
	3,  // 13: WAServerSync.SyncdPatch.exitCode:type_name -> WAServerSync.ExitCode
	14, // [14:14] is the sub-list for method output_type
	14, // [14:14] is the sub-list for method input_type
	14, // [14:14] is the sub-list for extension type_name
	14, // [14:14] is the sub-list for extension extendee
	0,  // [0:14] is the sub-list for field type_name
}

func init() { file_waServerSync_WAServerSync_proto_init() }
func file_waServerSync_WAServerSync_proto_init() {
	if File_waServerSync_WAServerSync_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_waServerSync_WAServerSync_proto_rawDesc), len(file_waServerSync_WAServerSync_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_waServerSync_WAServerSync_proto_goTypes,
		DependencyIndexes: file_waServerSync_WAServerSync_proto_depIdxs,
		EnumInfos:         file_waServerSync_WAServerSync_proto_enumTypes,
		MessageInfos:      file_waServerSync_WAServerSync_proto_msgTypes,
	}.Build()
	File_waServerSync_WAServerSync_proto = out.File
	file_waServerSync_WAServerSync_proto_goTypes = nil
	file_waServerSync_WAServerSync_proto_depIdxs = nil
}
