package dummy

import "log"

type MessagePusher struct{}

func NewMessagePusher() *MessagePusher {
	return &MessagePusher{}
}

func (*MessagePusher) PushMessage(deviceID int64, message string) error {
	log.Printf("push message for deviceID=%d: %s:", deviceID, message)
	return nil
}
