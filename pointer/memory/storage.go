package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Storage struct {
	accountsMx sync.RWMutex
	accounts   map[int64]int64
}

func NewStorage() *Storage {
	return &Storage{
		accounts: map[int64]int64{
			123: 1000,
			234: 500,
		},
	}
}

func (s *Storage) DepositAccountAmount(ctx context.Context, accountID int64,
	amount int64) error {

	s.accountsMx.Lock()
	s.accounts[accountID] += amount
	fmt.Printf("[pointer] account %d has deposited and has %d amount\n",
		accountID, s.accounts[accountID])
	s.accountsMx.Unlock()
	return nil
}

func (s *Storage) WithdrawAccountAmount(ctx context.Context, accountID int64,
	amount int64) error {
	s.accountsMx.Lock()
	defer s.accountsMx.Unlock()
	if s.accounts[accountID]-amount < 0 {
		return errors.New("not enough amount")
	}
	s.accounts[accountID] -= amount
	fmt.Printf("[pointer] account %d has withdrawed and has %d amount\n",
		accountID, s.accounts[accountID])
	return nil
}

func (s *Storage) AccountAmount(_ context.Context, accountID int64) (int64, error) {
	s.accountsMx.RLock()
	defer s.accountsMx.RUnlock()
	return s.accounts[accountID], nil
}
