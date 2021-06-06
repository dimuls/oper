package operator

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"oper/entity"
	"oper/service"
)

type Storage interface {
	RegisteredOperation(ctx context.Context, id []byte) (
		*entity.RegisteredOperation, error)
	SetRegisteredOperation(ctx context.Context,
		ro *entity.RegisteredOperation) error
	AddOperationProcessing(ctx context.Context,
		op *entity.OperationProcessing) error
	OperationProcessing(ctx context.Context, opID []byte) (
		*entity.OperationProcessing, error)
}

type Server struct {
	storage     Storage
	grpcServer  *grpc.Server
	tcpListener net.Listener
	kafkaClient *kgo.Client
	stop        chan struct{}
	wg          sync.WaitGroup
	service.UnimplementedOperatorServer
}

func NewServer(s Storage) (*Server, error) {

	kafkaClient, err := kgo.NewClient(kgo.AutoTopicCreation(),
		kgo.SeedBrokers(service.KafkaServiceAddr))
	if err != nil {
		return nil, err
	}

	srv := &Server{
		storage:     s,
		kafkaClient: kafkaClient,
		stop:        make(chan struct{}),
	}

	srv.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_recovery.UnaryServerInterceptor(),
		//grpc_opentracing.UnaryServerInterceptor(),
		//grpc_prometheus.UnaryServerInterceptor,
		grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New())))))
	service.RegisterOperatorServer(srv.grpcServer, srv)

	srv.tcpListener, err = net.Listen("tcp", service.GRPCDefaultPort)
	if err != nil {
		panic(err)
	}

	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		err := srv.grpcServer.Serve(srv.tcpListener)
		if err != nil {
			panic(err)
		}
	}()

	krCtx, cancelKR := context.WithCancel(context.Background())

	kafkaClient.AssignGroup(service.OperatorServiceKafkaGroupID,
		kgo.GroupTopics(service.OperatorServiceName))

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

				logrus.WithField("operation_processing_id", entity.UUIDToStr(op.Id)).Info("got operation processing from kafka")

				ro, err := srv.storage.RegisteredOperation(context.Background(),
					op.RegisteredOperationId)
				if err != nil {
					continue
				}

				ro.State = op.State

				err = srv.storage.SetRegisteredOperation(context.Background(), ro)
				if err != nil {
					logrus.WithError(err).Error("failed to handle operation processing")
					continue
				}

				err = srv.storage.AddOperationProcessing(context.Background(), op)
				if err != nil {
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
	close(s.stop)
	s.grpcServer.GracefulStop()
	s.tcpListener.Close()
	s.wg.Wait()
	s.kafkaClient.Close()
}

func (s *Server) dispatchOperationProcessing(ctx context.Context,
	op *entity.OperationProcessing) error {

	opProto, err := proto.Marshal(op)
	if err != nil {
		return err
	}
	res := s.kafkaClient.ProduceSync(ctx, &kgo.Record{
		Topic: op.Steps[0].ToService,
		Value: opProto,
	})

	return res.FirstErr()
}

func newUUID() []byte {
	id := uuid.New()
	return id[:]
}

func (s *Server) RegisterOperation(ctx context.Context,
	req *service.RegisterOperationReq) (*service.RegisterOperationRes, error) {

	roID := newUUID()
	opID := newUUID()

	switch req.OperationId {

	case 1: // todo: operation id to registered operation mapping

		pe := &entity.Exchange{}

		err := proto.Unmarshal(req.Data, pe)
		if err != nil {
			return nil, err
		}

		withdrawPointsProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Withdraw{
				Withdraw: &entity.Withdraw{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		cancelWithdrawPointsProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Deposit{
				Deposit: &entity.Deposit{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		depositAccountProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Deposit{
				Deposit: &entity.Deposit{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		cancelDepositAccountProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Withdraw{
				Withdraw: &entity.Withdraw{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		// todo: devicer microservice with methods to get deviceID
		deviceID := rand.Int63n(math.MaxInt64)

		pushMessageProto, err := proto.Marshal(&entity.PushMessage{
			DeviceId: deviceID,
			Message: fmt.Sprintf(
				"у вас были списаны %d баллов и вы получили %d рублей",
				pe.Amount, pe.Amount),
		})

		cancelPushMessageProto, err := proto.Marshal(&entity.PushMessage{
			DeviceId: deviceID,
			Message: fmt.Sprintf(
				"операция по списанию %d баллов за %d руб отменена",
				pe.Amount, pe.Amount),
		})

		op := &entity.OperationProcessing{
			Id:                    opID,
			RegisteredOperationId: roID,
			State:                 service.StartedState, // just started
			Data:                  req.Data,
			Steps: []*entity.OperationProcessingStep{
				{
					Id:                  newUUID(),
					ToService:           service.PointerServiceName,
					Data:                withdrawPointsProto,
					StartCancelFromStep: 1,
				},
				{
					Id:                  newUUID(),
					ToService:           service.AccounterServiceName,
					Data:                depositAccountProto,
					StartCancelFromStep: 0,
				},
				{
					Id:                  newUUID(),
					ToService:           service.PusherServiceName,
					Data:                pushMessageProto,
					StartCancelFromStep: 0,
				},
			},
			CancelSteps: []*entity.OperationProcessingStep{
				{
					Id:        newUUID(),
					ToService: service.AccounterServiceName,
					Data:      cancelDepositAccountProto,
				},
				{
					Id:        newUUID(),
					ToService: service.PointerServiceName,
					Data:      cancelWithdrawPointsProto,
				},
				{
					Id:        newUUID(),
					ToService: service.PusherServiceName,
					Data:      cancelPushMessageProto,
				},
			},
			CurrentStep: 0,
			CreatedAt:   timestamppb.Now(),
		}

		ro := &entity.RegisteredOperation{
			Id:           roID,
			OperationId:  req.OperationId,
			ClientId:     req.ClientId,
			ProcessingId: op.Id,
			State:        1,
			CreatedAt:    timestamppb.Now(),
		}

		err = s.storage.SetRegisteredOperation(ctx, ro)
		if err != nil {
			return nil, err
		}

		// todo: dispatch retry

		err = s.dispatchOperationProcessing(ctx, op)
		if err != nil {
			return nil, err
		}

	case 2: // todo: operation id to registered operation mapping

		pe := &entity.Exchange{}

		err := proto.Unmarshal(req.Data, pe)
		if err != nil {
			return nil, err
		}

		withdrawAccountProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Withdraw{
				Withdraw: &entity.Withdraw{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		cancelWithdrawAccountProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Deposit{
				Deposit: &entity.Deposit{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		depositPointsProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Deposit{
				Deposit: &entity.Deposit{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		cancelDepositPointsProto, err := proto.Marshal(&entity.AccountOperation{
			Operation: &entity.AccountOperation_Withdraw{
				Withdraw: &entity.Withdraw{
					AccountId: pe.AccountId,
					Amount:    pe.Amount,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		// todo: devicer microservice with methods to get deviceID
		deviceID := rand.Int63n(math.MaxInt64)

		pushMessageProto, err := proto.Marshal(&entity.PushMessage{
			DeviceId: deviceID,
			Message: fmt.Sprintf(
				"у вас были списаны %d баллов и вы получили %d рублей",
				pe.Amount, pe.Amount),
		})

		cancelPushMessageProto, err := proto.Marshal(&entity.PushMessage{
			DeviceId: deviceID,
			Message: fmt.Sprintf(
				"операция по списанию %d баллов за %d руб отменена",
				pe.Amount, pe.Amount),
		})

		op := &entity.OperationProcessing{
			Id:                    newUUID(),
			RegisteredOperationId: roID,
			State:                 service.StartedState, // just started
			Data:                  req.Data,
			Steps: []*entity.OperationProcessingStep{
				{
					Id:                  newUUID(),
					ToService:           service.PointerServiceName,
					Data:                withdrawAccountProto,
					StartCancelFromStep: 1,
				},
				{
					Id:                  newUUID(),
					ToService:           service.AccounterServiceName,
					Data:                depositPointsProto,
					StartCancelFromStep: 0,
				},
				{
					Id:                  newUUID(),
					ToService:           service.PusherServiceName,
					Data:                pushMessageProto,
					StartCancelFromStep: 0,
				},
			},
			CancelSteps: []*entity.OperationProcessingStep{
				{
					Id:        newUUID(),
					ToService: service.AccounterServiceName,
					Data:      cancelDepositPointsProto,
				},
				{
					Id:        newUUID(),
					ToService: service.PointerServiceName,
					Data:      cancelWithdrawAccountProto,
				},
				{
					Id:        newUUID(),
					ToService: service.PusherServiceName,
					Data:      cancelPushMessageProto,
				},
			},
			CurrentStep: 0,
			CreatedAt:   timestamppb.Now(),
		}

		ro := &entity.RegisteredOperation{
			Id:           roID,
			OperationId:  req.OperationId,
			ClientId:     req.ClientId,
			ProcessingId: op.Id,
			State:        1,
			CreatedAt:    timestamppb.Now(),
		}

		err = s.storage.SetRegisteredOperation(ctx, ro)
		if err != nil {
			return nil, err
		}

		// todo: dispatch retry

		err = s.dispatchOperationProcessing(ctx, op)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("not implemented")
	}

	return &service.RegisterOperationRes{
		RegisteredOperationId: roID,
		OperationProcessingId: opID,
	}, nil
}

func (s *Server) UpdateOperation(ctx context.Context,
	req *service.UpdateOperationReq) (*service.UpdateOperationRes, error) {
	o, err := s.storage.RegisteredOperation(ctx, req.RegisteredOperationId)
	if err != nil {
		return nil, err
	}

	o.State = req.State

	err = s.storage.SetRegisteredOperation(ctx, o)
	if err != nil {
		return nil, err
	}

	return &service.UpdateOperationRes{}, nil
}

func (s *Server) OperationCanceled(ctx context.Context,
	req *service.OperationCanceledReq) (*service.OperationCanceledRes, error) {
	o, err := s.storage.RegisteredOperation(ctx, req.RegisteredOperationId)
	if err != nil {
		return nil, err
	}
	return &service.OperationCanceledRes{
		Canceled: o.State == service.CanceledState,
	}, nil
}

func (s *Server) OperationProcessing(ctx context.Context, req *service.OperationProcessingReq) (
	*service.OperationProcessingRes, error) {

	op, err := s.storage.OperationProcessing(ctx, req.OperationProcessingId)
	if err != nil {
		return nil, err
	}

	return &service.OperationProcessingRes{
		OperationProcessing: op,
	}, nil
}
