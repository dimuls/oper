package entity

import "github.com/google/uuid"

//go:generate protoc --proto_path=. --proto_path=../third_party/googleapis --go_out=. operation.proto score.proto

func UUIDToStr(id []byte) string {
	bs, _ := uuid.FromBytes(id)
	return bs.String()
}
