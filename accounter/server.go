package accounter

import (
	"context"
	"errors"
	"net"
	"oper/operator"
	"sync"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"

	"github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"oper/entity"
	"oper/service"
)

type Storage interface {
	DepositAccountAmount(ctx context.Context, accountID int64, amount int64) error
	WithdrawAccountAmount(ctx context.Context, accountID int64, amount int64) error
	AccountAmount(ctx context.Context, accountID int64) (int64, error)
}

type Server struct {
	storage      Storage
	operator     service.OperatorClient
	operatorConn *grpc.ClientConn
	grpcServer   *grpc.Server
	tcpListener  net.Listener
	kafkaClient  *kgo.Client
	stop         chan struct{}
	wg           sync.WaitGroup
	service.UnimplementedAccounterServer
}

func NewServer(s Storage) (*Server, error) {

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

	srv := &Server{
		storage:      s,
		operator:     service.NewOperatorClient(operatorConn),
		operatorConn: operatorConn,
		kafkaClient:  kafkaClient,
		stop:         make(chan struct{}),
	}

	srv.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_recovery.UnaryServerInterceptor(),
		//grpc_opentracing.UnaryServerInterceptor(),
		//grpc_prometheus.UnaryServerInterceptor,
		grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New())))))
	service.RegisterAccounterServer(srv.grpcServer, srv)

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

	kafkaClient.AssignGroup(service.AccounterServiceKafkaGroupID,
		kgo.GroupTopics(service.AccounterServiceName))

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
				logrus.WithField("errors", errs).Error("got poll records errors")
				continue
			}

			ri := fs.RecordIter()
			for !ri.Done() {
				r := ri.Next()

				op := &entity.OperationProcessing{}

				err = proto.Unmarshal(r.Value, op)
				if err != nil {
					continue
				}

				logrus.WithField("operation_processing_id", entity.UUIDToStr(op.Id)).Info("got operation processing from kafka")

				err = srv.handleOperationProcessing(krCtx, op)
				if err != nil {
					logrus.WithError(err).Error("failed to handle operation processing")
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

func (s *Server) handleAccountOperation(ctx context.Context,
	ao *entity.AccountOperation) error {

	switch aot := ao.Operation.(type) {
	case *entity.AccountOperation_Deposit:
		return s.storage.DepositAccountAmount(ctx, aot.Deposit.AccountId,
			aot.Deposit.Amount)
	case *entity.AccountOperation_Withdraw:
		return s.storage.WithdrawAccountAmount(ctx, aot.Withdraw.AccountId,
			aot.Withdraw.Amount)
	}

	return errors.New("account operation not implemented")
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

	ao := &entity.AccountOperation{}

	err := proto.Unmarshal(opStep.Data, ao)
	if err != nil {
		return err
	}

	err = s.handleAccountOperation(ctx, ao)
	if err != nil {
		operator.HandleErrorOperationProcessing(op, err)
		return nil
	}

	op.CurrentStep += 1

	opStep.DoneAt = timestamppb.Now()

	return nil
}

func (s *Server) AccountAmount(ctx context.Context, req *service.AccountAmountReq) (
	*service.AccountAmountRes, error) {

	amount, err := s.storage.AccountAmount(ctx, req.AccountId)
	if err != nil {
		return nil, err
	}

	return &service.AccountAmountRes{
		Amount: amount,
	}, nil
}
