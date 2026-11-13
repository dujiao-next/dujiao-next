package adminapitokencontract

import (
	"time"

	"github.com/dujiao-next/internal/modules/identity/adminapitoken/domain"
)

type Store interface {
	Create(*adminapitokendomain.Token) error
	List(adminID uint) ([]adminapitokendomain.Token, error)
	GetByDigest(string) (*adminapitokendomain.Token, error)
	GetByID(adminID, tokenID uint) (*adminapitokendomain.Token, error)
	Rotate(adminID, tokenID uint, oldDigest, newDigest string, now time.Time) (bool, error)
	Revoke(adminID, tokenID uint, now time.Time) (bool, error)
	Touch(tokenID uint, now time.Time) error
}
