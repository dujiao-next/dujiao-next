package nano

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrReplayed means the block already settled a different order.
var ErrReplayed = errors.New("nano: block already settled another order")

// FileClaims records which order each send block settled, one file per block
// hash in Dir. A claim is made with O_EXCL, so two processes racing on one
// block cannot both win, and it is on disk, so a restart does not forget it.
type FileClaims struct {
	Dir string
}

// Claim records hash as settling order. Claiming the same block for the same
// order again is not a replay (a webhook delivered twice); it reports
// duplicate=true so the caller does not book the payment twice. Claiming it
// for any other order returns ErrReplayed.
func (c FileClaims) Claim(hash, order string) (duplicate bool, err error) {
	h, err := NormalizeHash(hash)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(c.Dir) == "" {
		return false, errors.New("nano: claim directory is not configured")
	}
	if order == "" || strings.ContainsAny(order, "\r\n") {
		return false, fmt.Errorf("nano: order id %q cannot be recorded", order)
	}
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return false, err
	}
	path := filepath.Join(c.Dir, h)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		prev, rerr := os.ReadFile(path)
		if rerr != nil {
			return false, rerr
		}
		if strings.TrimSpace(string(prev)) == order {
			return true, nil
		}
		return false, ErrReplayed
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(order + "\n"); err != nil {
		f.Close()
		os.Remove(path)
		return false, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(path)
		return false, err
	}
	return false, f.Close()
}
