package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
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

	if accountID == 234 {
		return fmt.Errorf("test error")
	}

	s.accountsMx.Lock()
	s.accounts[accountID] += amount
	logrus.WithFields(logrus.Fields{
		"account_amount": s.accounts[accountID],
		"account_id":     accountID,
	}).Info("account has deposited")
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
	logrus.WithFields(logrus.Fields{
		"account_amount": s.accounts[accountID],
		"account_id":     accountID,
	}).Info("account has withdrawed")
	return nil
}

func (s *Storage) AccountAmount(_ context.Context, accountID int64) (int64, error) {
	s.accountsMx.RLock()
	defer s.accountsMx.RUnlock()
	return s.accounts[accountID], nil
}
