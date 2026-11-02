package credentialreader

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
)

type sourceStub struct {
	credential *apicredentialdomain.ApiCredential
}

func (s sourceStub) GetByID(uint) (*apicredentialdomain.ApiCredential, error) {
	return s.credential, nil
}
func (s sourceStub) GetByApiKey(string) (*apicredentialdomain.ApiCredential, error) {
	return s.credential, nil
}

func TestReaderMarksCredentialActiveOnlyWhenAPIAuthWouldAccept(t *testing.T) {
	cases := map[string]struct {
		mutate func(*apicredentialdomain.ApiCredential)
		want   bool
	}{
		"usable":        {func(*apicredentialdomain.ApiCredential) {}, true},
		"disabled":      {func(c *apicredentialdomain.ApiCredential) { c.IsActive = false }, false},
		"not approved":  {func(c *apicredentialdomain.ApiCredential) { c.Status = constants.ApiCredentialStatusRejected }, false},
		"user disabled": {func(c *apicredentialdomain.ApiCredential) { c.User.Status = constants.UserStatusDisabled }, false},
		"user missing":  {func(c *apicredentialdomain.ApiCredential) { c.User = nil }, false},
	}
	for name, tc := range cases {
		credential := &apicredentialdomain.ApiCredential{ID: 5, ApiKey: "k", ApiSecret: "s", Status: constants.ApiCredentialStatusApproved,
			IsActive: true, User: &userdomain.User{Status: constants.UserStatusActive}}
		tc.mutate(credential)
		got, err := New(sourceStub{credential: credential}).GetByID(5)
		if err != nil || got == nil || got.Active != tc.want {
			t.Fatalf("%s: got=%+v err=%v, want active=%v", name, got, err, tc.want)
		}
	}
}
