// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.12.4
// source: protocol.proto

package braidpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Frame struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Payload:
	//
	//	*Frame_Noop
	//	*Frame_Message
	//	*Frame_RequestMessages
	//	*Frame_Peer
	//	*Frame_RequestPeers
	//	*Frame_LastField
	Payload isFrame_Payload `protobuf_oneof:"payload"`
}

func (x *Frame) Reset() {
	*x = Frame{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protocol_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Frame) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Frame) ProtoMessage() {}

func (x *Frame) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Frame.ProtoReflect.Descriptor instead.
func (*Frame) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{0}
}

func (m *Frame) GetPayload() isFrame_Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (x *Frame) GetNoop() *NoOp {
	if x, ok := x.GetPayload().(*Frame_Noop); ok {
		return x.Noop
	}
	return nil
}

func (x *Frame) GetMessage() *Message {
	if x, ok := x.GetPayload().(*Frame_Message); ok {
		return x.Message
	}
	return nil
}

func (x *Frame) GetRequestMessages() *RequestMessages {
	if x, ok := x.GetPayload().(*Frame_RequestMessages); ok {
		return x.RequestMessages
	}
	return nil
}

func (x *Frame) GetPeer() *Peer {
	if x, ok := x.GetPayload().(*Frame_Peer); ok {
		return x.Peer
	}
	return nil
}

func (x *Frame) GetRequestPeers() *RequestPeers {
	if x, ok := x.GetPayload().(*Frame_RequestPeers); ok {
		return x.RequestPeers
	}
	return nil
}

func (x *Frame) GetLastField() *NoOp {
	if x, ok := x.GetPayload().(*Frame_LastField); ok {
		return x.LastField
	}
	return nil
}

type isFrame_Payload interface {
	isFrame_Payload()
}

type Frame_Noop struct {
	Noop *NoOp `protobuf:"bytes,1,opt,name=noop,proto3,oneof"`
}

type Frame_Message struct {
	Message *Message `protobuf:"bytes,2,opt,name=message,proto3,oneof"`
}

type Frame_RequestMessages struct {
	RequestMessages *RequestMessages `protobuf:"bytes,3,opt,name=request_messages,json=requestMessages,proto3,oneof"`
}

type Frame_Peer struct {
	Peer *Peer `protobuf:"bytes,4,opt,name=peer,proto3,oneof"`
}

type Frame_RequestPeers struct {
	RequestPeers *RequestPeers `protobuf:"bytes,5,opt,name=request_peers,json=requestPeers,proto3,oneof"`
}

type Frame_LastField struct {
	LastField *NoOp `protobuf:"bytes,536870911,opt,name=lastField,proto3,oneof"` // for testing
}

func (*Frame_Noop) isFrame_Payload() {}

func (*Frame_Message) isFrame_Payload() {}

func (*Frame_RequestMessages) isFrame_Payload() {}

func (*Frame_Peer) isFrame_Payload() {}

func (*Frame_RequestPeers) isFrame_Payload() {}

func (*Frame_LastField) isFrame_Payload() {}

type NoOp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *NoOp) Reset() {
	*x = NoOp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protocol_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NoOp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NoOp) ProtoMessage() {}

func (x *NoOp) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NoOp.ProtoReflect.Descriptor instead.
func (*NoOp) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{1}
}

type FrontierRef struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Nodes should order messagerefs in messages as described in MessageSetRef
	// and not in the order of the node's frontier parent table since this would
	// leak the bookkeepping work for other nodes to exploit.
	Messages *MessageSetRef `protobuf:"bytes,1,opt,name=messages,proto3" json:"messages,omitempty"`
}

func (x *FrontierRef) Reset() {
	*x = FrontierRef{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protocol_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FrontierRef) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FrontierRef) ProtoMessage() {}

func (x *FrontierRef) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FrontierRef.ProtoReflect.Descriptor instead.
func (*FrontierRef) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{2}
}

func (x *FrontierRef) GetMessages() *MessageSetRef {
	if x != nil {
		return x.Messages
	}
	return nil
}

type RequestMessages struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Want []*MessageRef `protobuf:"bytes,1,rep,name=want,proto3" json:"want,omitempty"`
	// Optional frontier of the requesting node so that the responding node can
	// also send other messages the requesting node might need (e.g. the
	// transitive parents of the messages in want).
	Frontier *FrontierRef `protobuf:"bytes,2,opt,name=frontier,proto3" json:"frontier,omitempty"`
}

func (x *RequestMessages) Reset() {
	*x = RequestMessages{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protocol_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RequestMessages) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RequestMessages) ProtoMessage() {}

func (x *RequestMessages) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RequestMessages.ProtoReflect.Descriptor instead.
func (*RequestMessages) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{3}
}

func (x *RequestMessages) GetWant() []*MessageRef {
	if x != nil {
		return x.Want
	}
	return nil
}

func (x *RequestMessages) GetFrontier() *FrontierRef {
	if x != nil {
		return x.Frontier
	}
	return nil
}

type Peer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address  string    `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Identity *Identity `protobuf:"bytes,2,opt,name=identity,proto3" json:"identity,omitempty"` // optional, validate if provided
}

func (x *Peer) Reset() {
	*x = Peer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protocol_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Peer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Peer) ProtoMessage() {}

func (x *Peer) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Peer.ProtoReflect.Descriptor instead.
func (*Peer) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{4}
}

func (x *Peer) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Peer) GetIdentity() *Identity {
	if x != nil {
		return x.Identity
	}
	return nil
}

type RequestPeers struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *RequestPeers) Reset() {
	*x = RequestPeers{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protocol_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RequestPeers) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RequestPeers) ProtoMessage() {}

func (x *RequestPeers) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RequestPeers.ProtoReflect.Descriptor instead.
func (*RequestPeers) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{5}
}

var File_protocol_proto protoreflect.FileDescriptor

var file_protocol_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x05, 0x62, 0x72, 0x61, 0x69, 0x64, 0x1a, 0x0b, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0xb6, 0x02, 0x0a, 0x05, 0x46, 0x72, 0x61, 0x6d, 0x65, 0x12, 0x21,
	0x0a, 0x04, 0x6e, 0x6f, 0x6f, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x62,
	0x72, 0x61, 0x69, 0x64, 0x2e, 0x4e, 0x6f, 0x4f, 0x70, 0x48, 0x00, 0x52, 0x04, 0x6e, 0x6f, 0x6f,
	0x70, 0x12, 0x2a, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x48, 0x00, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x43, 0x0a,
	0x10, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x48,
	0x00, 0x52, 0x0f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x73, 0x12, 0x21, 0x0a, 0x04, 0x70, 0x65, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x0b, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e, 0x50, 0x65, 0x65, 0x72, 0x48, 0x00, 0x52,
	0x04, 0x70, 0x65, 0x65, 0x72, 0x12, 0x3a, 0x0a, 0x0d, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x5f, 0x70, 0x65, 0x65, 0x72, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x62,
	0x72, 0x61, 0x69, 0x64, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x50, 0x65, 0x65, 0x72,
	0x73, 0x48, 0x00, 0x52, 0x0c, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x50, 0x65, 0x65, 0x72,
	0x73, 0x12, 0x2f, 0x0a, 0x09, 0x6c, 0x61, 0x73, 0x74, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x18, 0xff,
	0xff, 0xff, 0xff, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64,
	0x2e, 0x4e, 0x6f, 0x4f, 0x70, 0x48, 0x00, 0x52, 0x09, 0x6c, 0x61, 0x73, 0x74, 0x46, 0x69, 0x65,
	0x6c, 0x64, 0x42, 0x09, 0x0a, 0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x22, 0x06, 0x0a,
	0x04, 0x4e, 0x6f, 0x4f, 0x70, 0x22, 0x3f, 0x0a, 0x0b, 0x46, 0x72, 0x6f, 0x6e, 0x74, 0x69, 0x65,
	0x72, 0x52, 0x65, 0x66, 0x12, 0x30, 0x0a, 0x08, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x53, 0x65, 0x74, 0x52, 0x65, 0x66, 0x52, 0x08, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x22, 0x68, 0x0a, 0x0f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x12, 0x25, 0x0a, 0x04, 0x77, 0x61, 0x6e,
	0x74, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e,
	0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x66, 0x52, 0x04, 0x77, 0x61, 0x6e, 0x74,
	0x12, 0x2e, 0x0a, 0x08, 0x66, 0x72, 0x6f, 0x6e, 0x74, 0x69, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x12, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e, 0x46, 0x72, 0x6f, 0x6e, 0x74,
	0x69, 0x65, 0x72, 0x52, 0x65, 0x66, 0x52, 0x08, 0x66, 0x72, 0x6f, 0x6e, 0x74, 0x69, 0x65, 0x72,
	0x22, 0x4d, 0x0a, 0x04, 0x50, 0x65, 0x65, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65,
	0x73, 0x73, 0x12, 0x2b, 0x0a, 0x08, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2e, 0x49, 0x64, 0x65,
	0x6e, 0x74, 0x69, 0x74, 0x79, 0x52, 0x08, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x22,
	0x0e, 0x0a, 0x0c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x50, 0x65, 0x65, 0x72, 0x73, 0x42,
	0x2a, 0x5a, 0x28, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x76, 0x73,
	0x65, 0x6b, 0x68, 0x61, 0x72, 0x2f, 0x62, 0x72, 0x61, 0x69, 0x64, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x61, 0x70, 0x69, 0x2f, 0x62, 0x72, 0x61, 0x69, 0x64, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_protocol_proto_rawDescOnce sync.Once
	file_protocol_proto_rawDescData = file_protocol_proto_rawDesc
)

func file_protocol_proto_rawDescGZIP() []byte {
	file_protocol_proto_rawDescOnce.Do(func() {
		file_protocol_proto_rawDescData = protoimpl.X.CompressGZIP(file_protocol_proto_rawDescData)
	})
	return file_protocol_proto_rawDescData
}

var file_protocol_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_protocol_proto_goTypes = []interface{}{
	(*Frame)(nil),           // 0: braid.Frame
	(*NoOp)(nil),            // 1: braid.NoOp
	(*FrontierRef)(nil),     // 2: braid.FrontierRef
	(*RequestMessages)(nil), // 3: braid.RequestMessages
	(*Peer)(nil),            // 4: braid.Peer
	(*RequestPeers)(nil),    // 5: braid.RequestPeers
	(*Message)(nil),         // 6: braid.Message
	(*MessageSetRef)(nil),   // 7: braid.MessageSetRef
	(*MessageRef)(nil),      // 8: braid.MessageRef
	(*Identity)(nil),        // 9: braid.Identity
}
var file_protocol_proto_depIdxs = []int32{
	1,  // 0: braid.Frame.noop:type_name -> braid.NoOp
	6,  // 1: braid.Frame.message:type_name -> braid.Message
	3,  // 2: braid.Frame.request_messages:type_name -> braid.RequestMessages
	4,  // 3: braid.Frame.peer:type_name -> braid.Peer
	5,  // 4: braid.Frame.request_peers:type_name -> braid.RequestPeers
	1,  // 5: braid.Frame.lastField:type_name -> braid.NoOp
	7,  // 6: braid.FrontierRef.messages:type_name -> braid.MessageSetRef
	8,  // 7: braid.RequestMessages.want:type_name -> braid.MessageRef
	2,  // 8: braid.RequestMessages.frontier:type_name -> braid.FrontierRef
	9,  // 9: braid.Peer.identity:type_name -> braid.Identity
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_protocol_proto_init() }
func file_protocol_proto_init() {
	if File_protocol_proto != nil {
		return
	}
	file_braid_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_protocol_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Frame); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protocol_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NoOp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protocol_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FrontierRef); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protocol_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RequestMessages); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protocol_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Peer); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protocol_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RequestPeers); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_protocol_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Frame_Noop)(nil),
		(*Frame_Message)(nil),
		(*Frame_RequestMessages)(nil),
		(*Frame_Peer)(nil),
		(*Frame_RequestPeers)(nil),
		(*Frame_LastField)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_protocol_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_protocol_proto_goTypes,
		DependencyIndexes: file_protocol_proto_depIdxs,
		MessageInfos:      file_protocol_proto_msgTypes,
	}.Build()
	File_protocol_proto = out.File
	file_protocol_proto_rawDesc = nil
	file_protocol_proto_goTypes = nil
	file_protocol_proto_depIdxs = nil
}
