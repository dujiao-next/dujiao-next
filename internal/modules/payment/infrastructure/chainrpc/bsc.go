package chainrpc

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"strconv"
	"strings"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

type evmBlock struct {
	Hash         string   `json:"hash"`
	Number       string   `json:"number"`
	Timestamp    string   `json:"timestamp"`
	Transactions []string `json:"transactions"`
}

type evmLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	BlockNumber      string   `json:"blockNumber"`
	LogIndex         string   `json:"logIndex"`
	Removed          *bool    `json:"removed"`
}

func (v *Verifier) bscBlock(ctx context.Context, s Source, tag string) (evmBlock, uint64, int64, error) {
	var b evmBlock
	if err := v.evmCall(ctx, s, "eth_getBlockByNumber", []interface{}{tag, false}, &b); err != nil {
		return b, 0, 0, err
	}
	n, err := quantity(b.Number)
	secs, terr := quantity(b.Timestamp)
	if err != nil || terr != nil || secs > math.MaxInt64/1000 || canonicalHash(b.Hash, true) == "" {
		return b, 0, 0, paymentcontract.ErrChainUnavailable
	}
	return b, n, int64(secs) * 1000, nil
}

func (v *Verifier) verifyBSC(ctx context.Context, s Source, p NetworkPolicy, e paymentdomain.ChainExpectation, txID string) (*paymentdomain.ChainProof, error) {
	var chainID string
	if err := v.evmCall(ctx, s, "eth_chainId", []interface{}{}, &chainID); err != nil {
		return nil, err
	}
	id, err := quantity(chainID)
	if err != nil || id != 56 {
		return nil, paymentcontract.ErrChainMismatch
	}
	anchor, number, _, err := v.bscBlock(ctx, s, "0x"+strconv.FormatUint(p.AnchorNumber, 16))
	if err != nil {
		return nil, err
	}
	if number != p.AnchorNumber || canonicalHash(anchor.Hash, true) != canonicalHash(p.AnchorHash, true) {
		return nil, paymentcontract.ErrChainMismatch
	}
	var receipt struct {
		TransactionHash  string   `json:"transactionHash"`
		TransactionIndex string   `json:"transactionIndex"`
		Status           string   `json:"status"`
		BlockHash        string   `json:"blockHash"`
		BlockNumber      string   `json:"blockNumber"`
		Logs             []evmLog `json:"logs"`
	}
	if err := v.evmCall(ctx, s, "eth_getTransactionReceipt", []interface{}{txID}, &receipt); err != nil {
		return nil, err
	}
	height, herr := quantity(receipt.BlockNumber)
	position, perr := quantity(receipt.TransactionIndex)
	if herr != nil || perr != nil || receipt.Status != "0x1" || canonicalHash(receipt.TransactionHash, true) != txID || canonicalHash(receipt.BlockHash, true) == "" {
		return nil, paymentcontract.ErrChainMismatch
	}
	var transaction struct {
		Hash             string `json:"hash"`
		BlockHash        string `json:"blockHash"`
		BlockNumber      string `json:"blockNumber"`
		TransactionIndex string `json:"transactionIndex"`
	}
	if err := v.evmCall(ctx, s, "eth_getTransactionByHash", []interface{}{txID}, &transaction); err != nil {
		return nil, err
	}
	if canonicalHash(transaction.Hash, true) != txID || canonicalHash(transaction.BlockHash, true) != canonicalHash(receipt.BlockHash, true) || transaction.BlockNumber != receipt.BlockNumber || transaction.TransactionIndex != receipt.TransactionIndex {
		return nil, paymentcontract.ErrChainMismatch
	}
	block, bn, timestamp, err := v.bscBlock(ctx, s, receipt.BlockNumber)
	if err != nil {
		return nil, err
	}
	if bn != height || canonicalHash(block.Hash, true) != canonicalHash(receipt.BlockHash, true) || position >= uint64(len(block.Transactions)) || canonicalHash(block.Transactions[position], true) != txID {
		return nil, paymentcontract.ErrChainMismatch
	}
	// No fallback from finalized to latest or a guessed confirmation count.
	finalBlock, finalized, finalTime, err := v.bscBlock(ctx, s, "finalized")
	if err != nil {
		return nil, err
	}
	if !v.freshHead(finalTime) {
		return nil, paymentcontract.ErrChainUnavailable
	}
	if finalized < height || finalTime < timestamp {
		return nil, paymentcontract.ErrChainPending
	}
	if finalized < p.AnchorNumber || (finalized == height && (canonicalHash(finalBlock.Hash, true) != canonicalHash(block.Hash, true) || finalTime != timestamp)) {
		return nil, paymentcontract.ErrChainMismatch
	}

	var matches []uint64
	seen := map[uint64]bool{}
	for _, log := range receipt.Logs {
		index, err := quantity(log.LogIndex)
		if err != nil || seen[index] || log.Removed == nil || *log.Removed || canonicalHash(log.TransactionHash, true) != txID ||
			log.TransactionIndex != receipt.TransactionIndex || log.BlockNumber != receipt.BlockNumber || canonicalHash(log.BlockHash, true) != canonicalHash(block.Hash, true) {
			return nil, paymentcontract.ErrChainMismatch
		}
		seen[index] = true
		if evmAddress(log.Address) != e.Contract || len(log.Topics) == 0 || strings.ToLower(log.Topics[0]) != "0x"+transferTopic {
			continue
		}
		if len(log.Topics) != 3 {
			return nil, paymentcontract.ErrChainMismatch
		}
		from, to, amount, ok := decodeTransfer(log.Topics[1], log.Topics[2], log.Data, true)
		if !ok || from == strings.Repeat("0", 40) {
			return nil, paymentcontract.ErrChainMismatch
		}
		if "0x"+from == e.Recipient {
			return nil, paymentcontract.ErrChainAmbiguous
		}
		if "0x"+to == e.Recipient && amount == e.RawAmount {
			matches = append(matches, index)
		}
	}
	if len(matches) == 0 {
		return nil, paymentcontract.ErrChainMismatch
	}
	if len(matches) != 1 {
		return nil, paymentcontract.ErrChainAmbiguous
	}
	if err := v.checkWindow(e, timestamp); err != nil {
		return nil, err
	}
	// Re-read the canonical inclusion block after finality lookup; provider
	// disagreement or a moving fork cannot reuse an earlier matching receipt.
	again, an, at, err := v.bscBlock(ctx, s, receipt.BlockNumber)
	if err != nil {
		return nil, err
	}
	if an != height || at != timestamp || canonicalHash(again.Hash, true) != canonicalHash(block.Hash, true) || position >= uint64(len(again.Transactions)) || canonicalHash(again.Transactions[position], true) != txID {
		return nil, paymentcontract.ErrChainDisagreement
	}
	return &paymentdomain.ChainProof{Network: e.Network, TransactionHash: txID, EventIndex: matches[0], BlockHash: canonicalHash(block.Hash, true), BlockNumber: height, BlockTimeMs: timestamp, Contract: e.Contract, Recipient: e.Recipient, RawAmount: e.RawAmount, Finality: []paymentdomain.SourceFinality{{Name: s.Name, Provider: s.Provider, AnchorNumber: p.AnchorNumber, AnchorHash: canonicalHash(p.AnchorHash, true), Number: finalized, Hash: canonicalHash(finalBlock.Hash, true), TimeMs: finalTime}}}, nil
}

func decodeTransfer(from, to, data string, prefix bool) (string, string, string, bool) {
	values := []string{from, to, data}
	for i, s := range values {
		if prefix {
			if !strings.HasPrefix(s, "0x") {
				return "", "", "", false
			}
			s = s[2:]
		}
		if len(s) != 64 {
			return "", "", "", false
		}
		if _, err := hex.DecodeString(s); err != nil {
			return "", "", "", false
		}
		values[i] = strings.ToLower(s)
	}
	if values[0][:24] != strings.Repeat("0", 24) || values[1][:24] != strings.Repeat("0", 24) {
		return "", "", "", false
	}
	n, _ := new(big.Int).SetString(values[2], 16)
	return values[0][24:], values[1][24:], n.String(), true
}
