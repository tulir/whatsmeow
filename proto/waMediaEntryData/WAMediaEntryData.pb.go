// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.28.2
// source: waMediaEntryData/WAMediaEntryData.proto

package waMediaEntryData

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type MediaEntry struct {
	state                        protoimpl.MessageState             `protogen:"open.v1"`
	FileSHA256                   []byte                             `protobuf:"bytes,1,opt,name=fileSHA256" json:"fileSHA256,omitempty"`
	MediaKey                     []byte                             `protobuf:"bytes,2,opt,name=mediaKey" json:"mediaKey,omitempty"`
	FileEncSHA256                []byte                             `protobuf:"bytes,3,opt,name=fileEncSHA256" json:"fileEncSHA256,omitempty"`
	DirectPath                   *string                            `protobuf:"bytes,4,opt,name=directPath" json:"directPath,omitempty"`
	MediaKeyTimestamp            *int64                             `protobuf:"varint,5,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ServerMediaType              *string                            `protobuf:"bytes,6,opt,name=serverMediaType" json:"serverMediaType,omitempty"`
	UploadToken                  []byte                             `protobuf:"bytes,7,opt,name=uploadToken" json:"uploadToken,omitempty"`
	ValidatedTimestamp           []byte                             `protobuf:"bytes,8,opt,name=validatedTimestamp" json:"validatedTimestamp,omitempty"`
	Sidecar                      []byte                             `protobuf:"bytes,9,opt,name=sidecar" json:"sidecar,omitempty"`
	ObjectID                     *string                            `protobuf:"bytes,10,opt,name=objectID" json:"objectID,omitempty"`
	FBID                         *string                            `protobuf:"bytes,11,opt,name=FBID" json:"FBID,omitempty"`
	DownloadableThumbnail        *MediaEntry_DownloadableThumbnail  `protobuf:"bytes,12,opt,name=downloadableThumbnail" json:"downloadableThumbnail,omitempty"`
	Handle                       *string                            `protobuf:"bytes,13,opt,name=handle" json:"handle,omitempty"`
	Filename                     *string                            `protobuf:"bytes,14,opt,name=filename" json:"filename,omitempty"`
	ProgressiveJPEGDetails       *MediaEntry_ProgressiveJpegDetails `protobuf:"bytes,15,opt,name=progressiveJPEGDetails" json:"progressiveJPEGDetails,omitempty"`
	Size                         *int64                             `protobuf:"varint,16,opt,name=size" json:"size,omitempty"`
	LastDownloadAttemptTimestamp *int64                             `protobuf:"varint,17,opt,name=lastDownloadAttemptTimestamp" json:"lastDownloadAttemptTimestamp,omitempty"`
	unknownFields                protoimpl.UnknownFields
	sizeCache                    protoimpl.SizeCache
}

func (x *MediaEntry) Reset() {
	*x = MediaEntry{}
	mi := &file_waMediaEntryData_WAMediaEntryData_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MediaEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MediaEntry) ProtoMessage() {}

func (x *MediaEntry) ProtoReflect() protoreflect.Message {
	mi := &file_waMediaEntryData_WAMediaEntryData_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MediaEntry.ProtoReflect.Descriptor instead.
func (*MediaEntry) Descriptor() ([]byte, []int) {
	return file_waMediaEntryData_WAMediaEntryData_proto_rawDescGZIP(), []int{0}
}

func (x *MediaEntry) GetFileSHA256() []byte {
	if x != nil {
		return x.FileSHA256
	}
	return nil
}

func (x *MediaEntry) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *MediaEntry) GetFileEncSHA256() []byte {
	if x != nil {
		return x.FileEncSHA256
	}
	return nil
}

func (x *MediaEntry) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *MediaEntry) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *MediaEntry) GetServerMediaType() string {
	if x != nil && x.ServerMediaType != nil {
		return *x.ServerMediaType
	}
	return ""
}

func (x *MediaEntry) GetUploadToken() []byte {
	if x != nil {
		return x.UploadToken
	}
	return nil
}

func (x *MediaEntry) GetValidatedTimestamp() []byte {
	if x != nil {
		return x.ValidatedTimestamp
	}
	return nil
}

func (x *MediaEntry) GetSidecar() []byte {
	if x != nil {
		return x.Sidecar
	}
	return nil
}

func (x *MediaEntry) GetObjectID() string {
	if x != nil && x.ObjectID != nil {
		return *x.ObjectID
	}
	return ""
}

func (x *MediaEntry) GetFBID() string {
	if x != nil && x.FBID != nil {
		return *x.FBID
	}
	return ""
}

func (x *MediaEntry) GetDownloadableThumbnail() *MediaEntry_DownloadableThumbnail {
	if x != nil {
		return x.DownloadableThumbnail
	}
	return nil
}

func (x *MediaEntry) GetHandle() string {
	if x != nil && x.Handle != nil {
		return *x.Handle
	}
	return ""
}

func (x *MediaEntry) GetFilename() string {
	if x != nil && x.Filename != nil {
		return *x.Filename
	}
	return ""
}

func (x *MediaEntry) GetProgressiveJPEGDetails() *MediaEntry_ProgressiveJpegDetails {
	if x != nil {
		return x.ProgressiveJPEGDetails
	}
	return nil
}

func (x *MediaEntry) GetSize() int64 {
	if x != nil && x.Size != nil {
		return *x.Size
	}
	return 0
}

func (x *MediaEntry) GetLastDownloadAttemptTimestamp() int64 {
	if x != nil && x.LastDownloadAttemptTimestamp != nil {
		return *x.LastDownloadAttemptTimestamp
	}
	return 0
}

type MediaEntry_ProgressiveJpegDetails struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ScanLengths   []uint32               `protobuf:"varint,1,rep,name=scanLengths" json:"scanLengths,omitempty"`
	Sidecar       []byte                 `protobuf:"bytes,2,opt,name=sidecar" json:"sidecar,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MediaEntry_ProgressiveJpegDetails) Reset() {
	*x = MediaEntry_ProgressiveJpegDetails{}
	mi := &file_waMediaEntryData_WAMediaEntryData_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MediaEntry_ProgressiveJpegDetails) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MediaEntry_ProgressiveJpegDetails) ProtoMessage() {}

func (x *MediaEntry_ProgressiveJpegDetails) ProtoReflect() protoreflect.Message {
	mi := &file_waMediaEntryData_WAMediaEntryData_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MediaEntry_ProgressiveJpegDetails.ProtoReflect.Descriptor instead.
func (*MediaEntry_ProgressiveJpegDetails) Descriptor() ([]byte, []int) {
	return file_waMediaEntryData_WAMediaEntryData_proto_rawDescGZIP(), []int{0, 0}
}

func (x *MediaEntry_ProgressiveJpegDetails) GetScanLengths() []uint32 {
	if x != nil {
		return x.ScanLengths
	}
	return nil
}

func (x *MediaEntry_ProgressiveJpegDetails) GetSidecar() []byte {
	if x != nil {
		return x.Sidecar
	}
	return nil
}

type MediaEntry_DownloadableThumbnail struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	FileSHA256        []byte                 `protobuf:"bytes,1,opt,name=fileSHA256" json:"fileSHA256,omitempty"`
	FileEncSHA256     []byte                 `protobuf:"bytes,2,opt,name=fileEncSHA256" json:"fileEncSHA256,omitempty"`
	DirectPath        *string                `protobuf:"bytes,3,opt,name=directPath" json:"directPath,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,4,opt,name=mediaKey" json:"mediaKey,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,5,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ObjectID          *string                `protobuf:"bytes,6,opt,name=objectID" json:"objectID,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *MediaEntry_DownloadableThumbnail) Reset() {
	*x = MediaEntry_DownloadableThumbnail{}
	mi := &file_waMediaEntryData_WAMediaEntryData_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MediaEntry_DownloadableThumbnail) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MediaEntry_DownloadableThumbnail) ProtoMessage() {}

func (x *MediaEntry_DownloadableThumbnail) ProtoReflect() protoreflect.Message {
	mi := &file_waMediaEntryData_WAMediaEntryData_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MediaEntry_DownloadableThumbnail.ProtoReflect.Descriptor instead.
func (*MediaEntry_DownloadableThumbnail) Descriptor() ([]byte, []int) {
	return file_waMediaEntryData_WAMediaEntryData_proto_rawDescGZIP(), []int{0, 1}
}

func (x *MediaEntry_DownloadableThumbnail) GetFileSHA256() []byte {
	if x != nil {
		return x.FileSHA256
	}
	return nil
}

func (x *MediaEntry_DownloadableThumbnail) GetFileEncSHA256() []byte {
	if x != nil {
		return x.FileEncSHA256
	}
	return nil
}

func (x *MediaEntry_DownloadableThumbnail) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *MediaEntry_DownloadableThumbnail) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *MediaEntry_DownloadableThumbnail) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *MediaEntry_DownloadableThumbnail) GetObjectID() string {
	if x != nil && x.ObjectID != nil {
		return *x.ObjectID
	}
	return ""
}

var File_waMediaEntryData_WAMediaEntryData_proto protoreflect.FileDescriptor

var file_waMediaEntryData_WAMediaEntryData_proto_rawDesc = string([]byte{
	0x0a, 0x27, 0x77, 0x61, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x44, 0x61,
	0x74, 0x61, 0x2f, 0x57, 0x41, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x44,
	0x61, 0x74, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x57, 0x41, 0x4d, 0x65, 0x64,
	0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x44, 0x61, 0x74, 0x61, 0x22, 0xa1, 0x08, 0x0a, 0x0a,
	0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x1e, 0x0a, 0x0a, 0x66, 0x69,
	0x6c, 0x65, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0a,
	0x66, 0x69, 0x6c, 0x65, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x12, 0x1a, 0x0a, 0x08, 0x6d, 0x65,
	0x64, 0x69, 0x61, 0x4b, 0x65, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x6d, 0x65,
	0x64, 0x69, 0x61, 0x4b, 0x65, 0x79, 0x12, 0x24, 0x0a, 0x0d, 0x66, 0x69, 0x6c, 0x65, 0x45, 0x6e,
	0x63, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0d, 0x66,
	0x69, 0x6c, 0x65, 0x45, 0x6e, 0x63, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x12, 0x1e, 0x0a, 0x0a,
	0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x50, 0x61, 0x74, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0a, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x50, 0x61, 0x74, 0x68, 0x12, 0x2c, 0x0a, 0x11,
	0x6d, 0x65, 0x64, 0x69, 0x61, 0x4b, 0x65, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x11, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x4b, 0x65,
	0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x28, 0x0a, 0x0f, 0x73, 0x65,
	0x72, 0x76, 0x65, 0x72, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x54, 0x79, 0x70, 0x65, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0f, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x4d, 0x65, 0x64, 0x69, 0x61,
	0x54, 0x79, 0x70, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x54, 0x6f,
	0x6b, 0x65, 0x6e, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x75, 0x70, 0x6c, 0x6f, 0x61,
	0x64, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x2e, 0x0a, 0x12, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x65, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x08, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x12, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x64, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x69, 0x64, 0x65, 0x63, 0x61,
	0x72, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x73, 0x69, 0x64, 0x65, 0x63, 0x61, 0x72,
	0x12, 0x1a, 0x0a, 0x08, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x18, 0x0a, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x12, 0x12, 0x0a, 0x04,
	0x46, 0x42, 0x49, 0x44, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x46, 0x42, 0x49, 0x44,
	0x12, 0x68, 0x0a, 0x15, 0x64, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x61, 0x62, 0x6c, 0x65,
	0x54, 0x68, 0x75, 0x6d, 0x62, 0x6e, 0x61, 0x69, 0x6c, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x32, 0x2e, 0x57, 0x41, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x44, 0x61,
	0x74, 0x61, 0x2e, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x44, 0x6f,
	0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x61, 0x62, 0x6c, 0x65, 0x54, 0x68, 0x75, 0x6d, 0x62, 0x6e,
	0x61, 0x69, 0x6c, 0x52, 0x15, 0x64, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x61, 0x62, 0x6c,
	0x65, 0x54, 0x68, 0x75, 0x6d, 0x62, 0x6e, 0x61, 0x69, 0x6c, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x61,
	0x6e, 0x64, 0x6c, 0x65, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x68, 0x61, 0x6e, 0x64,
	0x6c, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x0e,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x6b,
	0x0a, 0x16, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x69, 0x76, 0x65, 0x4a, 0x50, 0x45,
	0x47, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x33,
	0x2e, 0x57, 0x41, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x44, 0x61, 0x74,
	0x61, 0x2e, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x50, 0x72, 0x6f,
	0x67, 0x72, 0x65, 0x73, 0x73, 0x69, 0x76, 0x65, 0x4a, 0x70, 0x65, 0x67, 0x44, 0x65, 0x74, 0x61,
	0x69, 0x6c, 0x73, 0x52, 0x16, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x69, 0x76, 0x65,
	0x4a, 0x50, 0x45, 0x47, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x73,
	0x69, 0x7a, 0x65, 0x18, 0x10, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12,
	0x42, 0x0a, 0x1c, 0x6c, 0x61, 0x73, 0x74, 0x44, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x41,
	0x74, 0x74, 0x65, 0x6d, 0x70, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18,
	0x11, 0x20, 0x01, 0x28, 0x03, 0x52, 0x1c, 0x6c, 0x61, 0x73, 0x74, 0x44, 0x6f, 0x77, 0x6e, 0x6c,
	0x6f, 0x61, 0x64, 0x41, 0x74, 0x74, 0x65, 0x6d, 0x70, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x1a, 0x54, 0x0a, 0x16, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x69,
	0x76, 0x65, 0x4a, 0x70, 0x65, 0x67, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x20, 0x0a,
	0x0b, 0x73, 0x63, 0x61, 0x6e, 0x4c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0d, 0x52, 0x0b, 0x73, 0x63, 0x61, 0x6e, 0x4c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x73, 0x12,
	0x18, 0x0a, 0x07, 0x73, 0x69, 0x64, 0x65, 0x63, 0x61, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x07, 0x73, 0x69, 0x64, 0x65, 0x63, 0x61, 0x72, 0x1a, 0xe3, 0x01, 0x0a, 0x15, 0x44, 0x6f,
	0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x61, 0x62, 0x6c, 0x65, 0x54, 0x68, 0x75, 0x6d, 0x62, 0x6e,
	0x61, 0x69, 0x6c, 0x12, 0x1e, 0x0a, 0x0a, 0x66, 0x69, 0x6c, 0x65, 0x53, 0x48, 0x41, 0x32, 0x35,
	0x36, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0a, 0x66, 0x69, 0x6c, 0x65, 0x53, 0x48, 0x41,
	0x32, 0x35, 0x36, 0x12, 0x24, 0x0a, 0x0d, 0x66, 0x69, 0x6c, 0x65, 0x45, 0x6e, 0x63, 0x53, 0x48,
	0x41, 0x32, 0x35, 0x36, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0d, 0x66, 0x69, 0x6c, 0x65,
	0x45, 0x6e, 0x63, 0x53, 0x48, 0x41, 0x32, 0x35, 0x36, 0x12, 0x1e, 0x0a, 0x0a, 0x64, 0x69, 0x72,
	0x65, 0x63, 0x74, 0x50, 0x61, 0x74, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x64,
	0x69, 0x72, 0x65, 0x63, 0x74, 0x50, 0x61, 0x74, 0x68, 0x12, 0x1a, 0x0a, 0x08, 0x6d, 0x65, 0x64,
	0x69, 0x61, 0x4b, 0x65, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x6d, 0x65, 0x64,
	0x69, 0x61, 0x4b, 0x65, 0x79, 0x12, 0x2c, 0x0a, 0x11, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x4b, 0x65,
	0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x11, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x4b, 0x65, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x12, 0x1a, 0x0a, 0x08, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x42,
	0x51, 0x5a, 0x4f, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x61, 0x6b,
	0x68, 0x65, 0x6e, 0x61, 0x74, 0x65, 0x6e, 0x78, 0x79, 0x7a, 0x2f, 0x61, 0x6d, 0x61, 0x72, 0x6e,
	0x61, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2f, 0x67, 0x6f, 0x2f, 0x6c, 0x69, 0x62,
	0x73, 0x2f, 0x77, 0x68, 0x61, 0x74, 0x73, 0x6d, 0x65, 0x6f, 0x77, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x77, 0x61, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x44, 0x61,
	0x74, 0x61,
})

var (
	file_waMediaEntryData_WAMediaEntryData_proto_rawDescOnce sync.Once
	file_waMediaEntryData_WAMediaEntryData_proto_rawDescData []byte
)

func file_waMediaEntryData_WAMediaEntryData_proto_rawDescGZIP() []byte {
	file_waMediaEntryData_WAMediaEntryData_proto_rawDescOnce.Do(func() {
		file_waMediaEntryData_WAMediaEntryData_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_waMediaEntryData_WAMediaEntryData_proto_rawDesc), len(file_waMediaEntryData_WAMediaEntryData_proto_rawDesc)))
	})
	return file_waMediaEntryData_WAMediaEntryData_proto_rawDescData
}

var file_waMediaEntryData_WAMediaEntryData_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_waMediaEntryData_WAMediaEntryData_proto_goTypes = []any{
	(*MediaEntry)(nil),                        // 0: WAMediaEntryData.MediaEntry
	(*MediaEntry_ProgressiveJpegDetails)(nil), // 1: WAMediaEntryData.MediaEntry.ProgressiveJpegDetails
	(*MediaEntry_DownloadableThumbnail)(nil),  // 2: WAMediaEntryData.MediaEntry.DownloadableThumbnail
}
var file_waMediaEntryData_WAMediaEntryData_proto_depIdxs = []int32{
	2, // 0: WAMediaEntryData.MediaEntry.downloadableThumbnail:type_name -> WAMediaEntryData.MediaEntry.DownloadableThumbnail
	1, // 1: WAMediaEntryData.MediaEntry.progressiveJPEGDetails:type_name -> WAMediaEntryData.MediaEntry.ProgressiveJpegDetails
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_waMediaEntryData_WAMediaEntryData_proto_init() }
func file_waMediaEntryData_WAMediaEntryData_proto_init() {
	if File_waMediaEntryData_WAMediaEntryData_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_waMediaEntryData_WAMediaEntryData_proto_rawDesc), len(file_waMediaEntryData_WAMediaEntryData_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_waMediaEntryData_WAMediaEntryData_proto_goTypes,
		DependencyIndexes: file_waMediaEntryData_WAMediaEntryData_proto_depIdxs,
		MessageInfos:      file_waMediaEntryData_WAMediaEntryData_proto_msgTypes,
	}.Build()
	File_waMediaEntryData_WAMediaEntryData_proto = out.File
	file_waMediaEntryData_WAMediaEntryData_proto_goTypes = nil
	file_waMediaEntryData_WAMediaEntryData_proto_depIdxs = nil
}
