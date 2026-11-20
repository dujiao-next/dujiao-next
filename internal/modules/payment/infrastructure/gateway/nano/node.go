package nano

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
)

// Node reads from a Nano node's JSON RPC. It sends no key and signs nothing.
type Node struct {
	URL    string
	Client *http.Client
}

// ErrBlockNotFound means the node does not know the block.
var ErrBlockNotFound = errors.New("nano: block not found")

func (n Node) call(ctx context.Context, req map[string]string, out interface{}) error {
	if n.URL == "" {
		return errors.New("nano: node url is not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	hr.Header.Set("Content-Type", "application/json")
	c := n.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(hr)
	if err != nil {
		return fmt.Errorf("nano node %s: %w", req["action"], err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("nano node %s: HTTP %d", req["action"], resp.StatusCode)
	}
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error != "" {
		if e.Error == "Block not found" {
			return ErrBlockNotFound
		}
		return fmt.Errorf("nano node %s: %s", req["action"], e.Error)
	}
	return json.Unmarshal(data, out)
}

// BlockInfo reads one block.
func (n Node) BlockInfo(ctx context.Context, hash string) (BlockInfo, error) {
	var info BlockInfo
	h, err := NormalizeHash(hash)
	if err != nil {
		return info, err
	}
	err = n.call(ctx, map[string]string{"action": "block_info", "json_block": "true", "hash": h}, &info)
	return info, err
}

// FindSend looks for a send of exactly amount raw to account, first among the
// blocks the account has not received yet, then in its recent history. It
// returns the send block's hash, or "" when there is none.
func (n Node) FindSend(ctx context.Context, account string, amount *big.Int, count int) (string, error) {
	acct, err := ParseAddress(account)
	if err != nil {
		return "", err
	}
	if count <= 0 {
		count = 100
	}
	var rec struct {
		Blocks map[string]struct {
			Amount string `json:"amount"`
		} `json:"blocks"`
	}
	err = n.call(ctx, map[string]string{
		"action": "receivable", "account": acct, "count": strconv.Itoa(count),
		"threshold": amount.String(), "source": "true",
	}, &rec)
	if err != nil {
		return "", err
	}
	for h, b := range rec.Blocks {
		if v, err := ParseRaw(b.Amount); err == nil && v.Cmp(amount) == 0 {
			return NormalizeHash(h)
		}
	}
	var hist struct {
		History []struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
			Amount  string `json:"amount"`
			Link    string `json:"link"`
		} `json:"history"`
	}
	err = n.call(ctx, map[string]string{
		"action": "account_history", "account": acct, "count": strconv.Itoa(count), "raw": "true",
	}, &hist)
	if err != nil {
		return "", err
	}
	for _, e := range hist.History {
		if e.Subtype != "receive" && e.Type != "receive" {
			continue
		}
		if v, err := ParseRaw(e.Amount); err == nil && v.Cmp(amount) == 0 {
			// For a receive, link is the hash of the send it pocketed.
			return NormalizeHash(e.Link)
		}
	}
	return "", nil
}
