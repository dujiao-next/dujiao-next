// Package servicemac implements the versioned merchant-to-gateway envelope.
// The signature binds direction, authority, method, escaped path, body, time and
// nonce. Server-side nonce insertion must commit before executing the request.
package servicemac

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const Version = "dujiao-gateway-v1"

func Digest(r *http.Request, body []byte, secret string) string {
	sum := sha256.Sum256(body)
	text := strings.Join([]string{Version, "merchant->gateway", r.Host, r.Method, r.URL.EscapedPath(), r.Header.Get("X-Payment-Time"), r.Header.Get("X-Payment-Nonce"), hex.EncodeToString(sum[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(text))
	return hex.EncodeToString(mac.Sum(nil))
}
func Sign(r *http.Request, body []byte, secret string) error {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	r.Header.Set("X-Payment-Version", Version)
	r.Header.Set("X-Payment-Time", strconv.FormatInt(time.Now().Unix(), 10))
	r.Header.Set("X-Payment-Nonce", hex.EncodeToString(nonce[:]))
	r.Header.Set("X-Payment-MAC", Digest(r, body, secret))
	return nil
}
func Verify(r *http.Request, body []byte, secret string, now time.Time) error {
	denied := fmt.Errorf("service authentication denied")
	ts, err := strconv.ParseInt(r.Header.Get("X-Payment-Time"), 10, 64)
	if err != nil || strconv.FormatInt(ts, 10) != r.Header.Get("X-Payment-Time") || ts < now.Unix()-60 || ts > now.Unix()+30 {
		return denied
	}
	nonce, err := hex.DecodeString(r.Header.Get("X-Payment-Nonce"))
	if err != nil || len(nonce) != 32 || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.Header.Get("X-Payment-Version") != Version || len(secret) < 32 {
		return denied
	}
	got, err := hex.DecodeString(r.Header.Get("X-Payment-MAC"))
	want, _ := hex.DecodeString(Digest(r, body, secret))
	if err != nil || !hmac.Equal(got, want) {
		return denied
	}
	return nil
}
