package oper

// entity
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --go_out=.. entity/operation.proto
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --go_out=.. entity/account.proto
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --go_out=.. entity/push.proto

// services
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --proto_path=./.third_party/protocolbuffers/src --go_out=.. --go-grpc_out=.. --grpc-gateway_out=.. --openapiv2_out=.. service/weber.proto
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --proto_path=./.third_party/protocolbuffers/src --go_out=.. --go-grpc_out=.. service/operator.proto
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --proto_path=./.third_party/protocolbuffers/src --go_out=.. --go-grpc_out=.. service/accounter.proto
//go:generate protoc --proto_path=. --proto_path=./.third_party/googleapis --proto_path=./.third_party/protocolbuffers/src --go_out=.. --go-grpc_out=.. service/pointer.proto
