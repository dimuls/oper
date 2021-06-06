package pusher

import (
	"context"
	"oper/operator"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/twmb/franz-go/pkg/kgo"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"

	"oper/entity"
	"oper/service"
)

type MessagePusher interface {
	PushMessage(deviceID int64, message string) error
}

type Server struct {
	messagePusher MessagePusher
	operator      service.OperatorClient
	operatorConn  *grpc.ClientConn
	kafkaClient   *kgo.Client
	stop          chan struct{}
	wg            sync.WaitGroup
	service.UnimplementedOperatorServer
}

func NewServer(mp MessagePusher) (*Server, error) {

	kafkaClient, err := kgo.NewClient(kgo.AutoTopicCreation(),
		kgo.SeedBrokers(service.KafkaServiceAddr))
	if err != nil {
		return nil, err
	}

	operatorConn, err := grpc.Dial(service.OperatorServiceURL, grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		return nil, err
	}

	operator := service.NewOperatorClient(operatorConn)

	srv := &Server{
		messagePusher: mp,
		operator:      operator,
		operatorConn:  operatorConn,
		kafkaClient:   kafkaClient,
		stop:          make(chan struct{}),
	}

	krCtx, cancelKR := context.WithCancel(context.Background())

	kafkaClient.AssignGroup(service.PusherServiceKafkaGroupID,
		kgo.GroupTopics(service.PusherServiceName))

	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()

		for {
			select {
			case <-krCtx.Done():
				return
			default:
			}

			fs := kafkaClient.PollRecords(krCtx, 100)

			errs := fs.Errors()
			if len(errs) != 0 {
				panic(errs)
			}

			ri := fs.RecordIter()
			for !ri.Done() {
				r := ri.Next()

				op := &entity.OperationProcessing{}

				// todo: log errors

				err = proto.Unmarshal(r.Value, op)
				if err != nil {
					continue
				}

				err = srv.handleOperationProcessing(krCtx, op)
				if err != nil {
					op.State = service.ErrorState
				}

				err = srv.dispatchOperationProcessing(krCtx, op)
				if err != nil {
					logrus.WithError(err).Error("failed to dispatch operation processing")
					continue
				}
			}
		}
	}()

	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		<-srv.stop
		cancelKR()
	}()

	return srv, nil
}

func (s *Server) Close() {
	// todo: handle error
	close(s.stop)
	s.wg.Wait()
	s.kafkaClient.Close()
}

func (s *Server) dispatchOperationProcessing(ctx context.Context,
	op *entity.OperationProcessing) error {

	opProto, err := proto.Marshal(op)
	if err != nil {
		return err
	}

	if op.State == service.ErrorState || int(op.CurrentStep) >= len(op.Steps) {
		res := s.kafkaClient.ProduceSync(ctx, &kgo.Record{
			Topic: service.OperatorServiceName,
			Value: opProto,
		})
		if err := res.FirstErr(); err != nil {
			return err
		}
		return nil
	}

	res := s.kafkaClient.ProduceSync(ctx, &kgo.Record{
		Topic: op.Steps[op.CurrentStep].ToService,
		Value: opProto,
	})
	return res.FirstErr()
}

func (s *Server) handleOperationProcessing(ctx context.Context,
	op *entity.OperationProcessing) error {

	if !op.Canceled {
		res, err := s.operator.OperationCanceled(ctx, &service.OperationCanceledReq{
			RegisteredOperationId: op.RegisteredOperationId,
		})
		if err != nil {
			return err
		}

		if res.Canceled {
			operator.HandleCanceledOperationProcessing(op)
			return nil
		}
	}

	opStep := op.Steps[op.CurrentStep]

	opStep.StartedAt = timestamppb.Now()

	pm := &entity.PushMessage{}

	err := proto.Unmarshal(op.Steps[op.CurrentStep].Data, pm)
	if err != nil {
		return err
	}

	err = s.messagePusher.PushMessage(pm.DeviceId, pm.Message)
	if err != nil {
		return err
	}

	op.CurrentStep += 1
	opStep.DoneAt = timestamppb.Now()

	return nil
}
