package weber

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"oper/entity"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/mux"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"oper/service"
)

type Server struct {
	operator     service.OperatorClient
	operatorConn *grpc.ClientConn

	accounter     service.AccounterClient
	accounterConn *grpc.ClientConn

	pointer     service.PointerClient
	pointerConn *grpc.ClientConn

	grpcServer  *grpc.Server
	netListener net.Listener

	httpServer *http.Server

	wg sync.WaitGroup

	service.UnimplementedWeberServer
}

func NewServer() (*Server, error) {
	opConn, err := grpc.Dial(service.OperatorServiceURL, grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		return nil, err
	}

	accConn, err := grpc.Dial(service.AccounterServiceURL, grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		return nil, err
	}

	pointerConn, err := grpc.Dial(service.PointerServiceURL, grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		return nil, err
	}

	s := &Server{
		operator:      service.NewOperatorClient(opConn),
		operatorConn:  opConn,
		accounter:     service.NewAccounterClient(accConn),
		accounterConn: accConn,
		pointer:       service.NewPointerClient(pointerConn),
		pointerConn:   pointerConn,
	}

	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_recovery.UnaryServerInterceptor(),
			//grpc_opentracing.UnaryServerInterceptor(),
			//grpc_prometheus.UnaryServerInterceptor,
			grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New())))))

	service.RegisterWeberServer(s.grpcServer, s)

	s.netListener, err = net.Listen("tcp", service.GRPCDefaultPort)
	if err != nil {
		panic(err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := s.grpcServer.Serve(s.netListener)
		if err != nil {
			panic(err)
		}
	}()

	sm := runtime.NewServeMux()

	err = service.RegisterWeberHandlerFromEndpoint(
		context.Background(), sm, s.netListener.Addr().String(),
		[]grpc.DialOption{grpc.WithInsecure()})
	if err != nil {
		panic(err)
	}

	httpRouter := mux.NewRouter()

	httpRouter.PathPrefix("/api/v1/").Handler(
		cors.Default().Handler(sm))

	swaggerFile, err := service.WeberFS.Open("weber.swagger.json")
	if err != nil {
		panic(err)
	}

	swaggerJSON, err := ioutil.ReadAll(swaggerFile)
	swaggerFile.Close()
	if err != nil {
		panic(err)
	}

	httpRouter.Path("/api/swagger.json").Methods(http.MethodGet).
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write(swaggerJSON)
		})

	s.httpServer = &http.Server{
		Addr:    ":80",
		Handler: httpRouter,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			err := s.httpServer.ListenAndServe()
			if err != nil {
				if err == http.ErrServerClosed {
					break
				}
				log.Println(err)
			}
			time.Sleep(3 * time.Second)
		}
	}()

	return s, nil
}

func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
	s.grpcServer.GracefulStop()
	s.netListener.Close()
	s.operatorConn.Close()
	s.wg.Wait()
}

func (s *Server) PostExchange(ctx context.Context, req *service.PostExchangeReq) (
	*service.PostExchangeRes, error) {

	// todo: validation

	var opID int64

	switch {
	case req.Exchange.From == entity.Currency_POINTS && req.Exchange.To == entity.Currency_RUB:
		opID = 1
	case req.Exchange.From == entity.Currency_RUB && req.Exchange.To == entity.Currency_POINTS:
		opID = 2
	default:
		return nil, errors.New("invalid exchange")
	}

	exchangeProto, err := proto.Marshal(req.Exchange)
	if err != nil {
		return nil, err
	}

	res, err := s.operator.RegisterOperation(ctx, &service.RegisterOperationReq{
		OperationId: opID, // exchange, todo: operations enum (?)
		ClientId:    0,    // todo: jwt token (?) parsing and validation middleware
		Data:        exchangeProto,
	})
	if err != nil {
		return nil, err
	}

	return &service.PostExchangeRes{
		RegisteredOperationId: entity.UUIDToStr(res.RegisteredOperationId),
		OperationProcessingId: entity.UUIDToStr(res.OperationProcessingId),
	}, nil
}

func (s *Server) GetPointsAmount(ctx context.Context, req *service.GetPointsAmountReq) (
	*service.GetPointsAmountRes, error) {

	res, err := s.pointer.AccountAmount(ctx, &service.PointsAmountReq{
		AccountId: req.AccountId,
	})
	if err != nil {
		return nil, err
	}

	return &service.GetPointsAmountRes{
		Amount: res.Amount,
	}, nil
}

func (s *Server) GetAccountAmount(ctx context.Context, req *service.GetAccountAmountReq) (
	*service.GetAccountAmountRes, error) {

	res, err := s.accounter.AccountAmount(ctx, &service.AccountAmountReq{
		AccountId: req.AccountId,
	})
	if err != nil {
		return nil, err
	}

	return &service.GetAccountAmountRes{
		Amount: res.Amount,
	}, nil
}

func (s *Server) GetOperationProcessing(ctx context.Context, req *service.GetOperationProcessingReq) (
	*service.GetOperationProcessingRes, error) {

	opID, err := uuid.Parse(req.OperationProcessingId)
	if err != nil {
		return nil, err
	}

	res, err := s.operator.OperationProcessing(ctx, &service.OperationProcessingReq{
		OperationProcessingId: opID[:],
	})
	if err != nil {
		return nil, err
	}

	return &service.GetOperationProcessingRes{
		OperationProcessing: res.OperationProcessing,
	}, nil
}
