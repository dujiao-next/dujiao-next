package adminapplication

import (
	"testing"
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"

	"golang.org/x/crypto/bcrypt"
)

type bootstrapStoreStub struct {
	admins map[uint]*admindomain.Admin
	nextID uint
}

func newBootstrapStoreStub() *bootstrapStoreStub {
	return &bootstrapStoreStub{admins: make(map[uint]*admindomain.Admin), nextID: 1}
}

func (s *bootstrapStoreStub) GetByUsername(username string) (*admindomain.Admin, error) {
	for _, admin := range s.admins {
		if admin.Username == username && admin.DeletedAt == nil {
			copy := *admin
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *bootstrapStoreStub) GetByID(id uint) (*admindomain.Admin, error) {
	admin := s.admins[id]
	if admin == nil || admin.DeletedAt != nil {
		return nil, nil
	}
	copy := *admin
	return &copy, nil
}

func (s *bootstrapStoreStub) List() ([]admindomain.Admin, error) {
	admins := make([]admindomain.Admin, 0, len(s.admins))
	for _, admin := range s.admins {
		if admin.DeletedAt == nil {
			admins = append(admins, *admin)
		}
	}
	return admins, nil
}

func (s *bootstrapStoreStub) Count() (int64, error) {
	admins, _ := s.List()
	return int64(len(admins)), nil
}

func (s *bootstrapStoreStub) Create(admin *admindomain.Admin) error {
	admin.ID = s.nextID
	s.nextID++
	copy := *admin
	s.admins[admin.ID] = &copy
	return nil
}

func (s *bootstrapStoreStub) Update(admin *admindomain.Admin) error {
	copy := *admin
	s.admins[admin.ID] = &copy
	return nil
}

func (s *bootstrapStoreStub) Delete(uint) error                                       { return nil }
func (s *bootstrapStoreStub) UpdateTOTPPending(uint, string, time.Time) error         { return nil }
func (s *bootstrapStoreStub) UpdateTOTPEnabled(uint, string, time.Time, string) error { return nil }
func (s *bootstrapStoreStub) UpdateRecoveryCodes(uint, string) error                  { return nil }
func (s *bootstrapStoreStub) ClearTOTP(uint) error                                    { return nil }
func (s *bootstrapStoreStub) UpdatePassword(uint, string) error                       { return nil }

func TestInitDefaultAdminMarksBootstrapAdminAsSuper(t *testing.T) {
	store := newBootstrapStoreStub()
	if _, err := InitDefaultAdmin(store, "root-admin", "secret123"); err != nil {
		t.Fatalf("init default admin failed: %v", err)
	}
	admin, err := store.GetByUsername("root-admin")
	if err != nil {
		t.Fatalf("query bootstrap admin failed: %v", err)
	}
	if admin == nil || !admin.IsSuper {
		t.Fatalf("bootstrap admin should be super: %#v", admin)
	}
}

func TestInitDefaultAdminRepairsExistingBootstrapAdminSuperFlag(t *testing.T) {
	store := newBootstrapStoreStub()
	admin := &admindomain.Admin{Username: "root-admin", PasswordHash: "hashed-password"}
	if err := store.Create(admin); err != nil {
		t.Fatalf("create bootstrap admin failed: %v", err)
	}
	if _, err := InitDefaultAdmin(store, "root-admin", "ignored"); err != nil {
		t.Fatalf("repair bootstrap admin failed: %v", err)
	}
	refreshed, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("reload bootstrap admin failed: %v", err)
	}
	if refreshed == nil || !refreshed.IsSuper {
		t.Fatalf("existing bootstrap admin should be repaired to super: %#v", refreshed)
	}
}

func TestInitDefaultAdminLeavesOtherExistingAdminsUnchanged(t *testing.T) {
	store := newBootstrapStoreStub()
	existing := &admindomain.Admin{Username: "someone-else", PasswordHash: "hash"}
	if err := store.Create(existing); err != nil {
		t.Fatalf("create existing admin: %v", err)
	}
	if _, err := InitDefaultAdmin(store, "root-admin", "ignored"); err != nil {
		t.Fatalf("init with missing bootstrap admin: %v", err)
	}
	count, err := store.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestInitDefaultAdminGeneratesRandomPasswordWhenEmpty(t *testing.T) {
	store := newBootstrapStoreStub()
	generated, err := InitDefaultAdmin(store, "", "")
	if err != nil {
		t.Fatalf("init default admin failed: %v", err)
	}
	if len(generated) < 16 || generated == "admin123" {
		t.Fatalf("generated password = %q, want a long random value", generated)
	}
	admin, _ := store.GetByUsername("admin")
	if admin == nil || bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(generated)) != nil {
		t.Fatal("bootstrap admin should be created with the generated password")
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("admin123")) == nil {
		t.Fatal("bootstrap admin must not fall back to admin123")
	}

	if again, err := InitDefaultAdmin(newBootstrapStoreStub(), "", "configured-Strong-1"); err != nil || again != "" {
		t.Fatalf("configured password should not be reported as generated: %q, %v", again, err)
	}
}
