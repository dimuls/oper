package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"oper/entity"
)

type Storage struct {
	registeredOperationsMx sync.RWMutex
	registeredOperations   map[string]*entity.RegisteredOperation
	operationProcessingsMx sync.RWMutex
	operationProcessings   map[string]*entity.OperationProcessing
}

func NewStorage() *Storage {
	return &Storage{
		registeredOperations: map[string]*entity.RegisteredOperation{},
		operationProcessings: map[string]*entity.OperationProcessing{},
	}
}

func (s *Storage) RegisteredOperation(_ context.Context, id []byte) (
	*entity.RegisteredOperation, error) {
	idd, err := uuid.FromBytes(id)
	if err != nil {
		return nil, err
	}
	s.registeredOperationsMx.RLock()
	ro, exists := s.registeredOperations[idd.String()]
	s.registeredOperationsMx.RUnlock()
	if !exists {
		return nil, errors.New("registered operation doesn't exists")
	}
	return ro, nil
}

func (s *Storage) SetRegisteredOperation(_ context.Context,
	ro *entity.RegisteredOperation) error {
	id, err := uuid.FromBytes(ro.Id)
	if err != nil {
		return err
	}
	s.registeredOperationsMx.Lock()
	s.registeredOperations[id.String()] = ro
	s.registeredOperationsMx.Unlock()
	return nil
}

func (s *Storage) AddOperationProcessing(ctx context.Context,
	op *entity.OperationProcessing) error {
	id, err := uuid.FromBytes(op.Id)
	if err != nil {
		return err
	}
	s.operationProcessingsMx.Lock()
	s.operationProcessings[id.String()] = op
	s.operationProcessingsMx.Unlock()
	return nil
}

func (s *Storage) OperationProcessing(ctx context.Context, opID []byte) (
	*entity.OperationProcessing, error) {
	id, err := uuid.FromBytes(opID)
	if err != nil {
		return nil, err
	}
	s.operationProcessingsMx.RLock()
	op, exists := s.operationProcessings[id.String()]
	s.operationProcessingsMx.RUnlock()
	if !exists {
		return nil, errors.New("operation processing doesn't exists")
	}
	return op, nil
}
