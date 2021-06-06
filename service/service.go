package service

// todo: move constants to better place (service discovery?)

const (
	dockerSwarmDefaultPrefix = "dns:///tasks."
	GRPCDefaultPort          = ":9090"

	// todo: kafka service discovery (zookeeper?)
	KafkaServiceName = "kafka"
	KafkaServiceAddr = "tasks." + KafkaServiceName + ":9092"

	WeberServiceName = "weber"
	WeberServiceURL  = dockerSwarmDefaultPrefix + WeberServiceName + GRPCDefaultPort

	OperatorServiceName         = "operator"
	OperatorServiceURL          = dockerSwarmDefaultPrefix + OperatorServiceName + GRPCDefaultPort
	OperatorServiceKafkaGroupID = OperatorServiceName + "s"

	PointerServiceName         = "pointer"
	PointerServiceURL          = dockerSwarmDefaultPrefix + PointerServiceName + GRPCDefaultPort
	PointerServiceKafkaGroupID = PointerServiceName + "s"

	AccounterServiceName         = "accounter"
	AccounterServiceURL          = dockerSwarmDefaultPrefix + AccounterServiceName + GRPCDefaultPort
	AccounterServiceKafkaGroupID = AccounterServiceName + "s"

	PusherServiceName         = "pusher"
	PusherServiceURL          = dockerSwarmDefaultPrefix + PusherServiceName + GRPCDefaultPort
	PusherServiceKafkaGroupID = PusherServiceName + "s"

	StartedState  int32 = 0
	CanceledState int32 = -1
	ErrorState    int32 = -2
)
