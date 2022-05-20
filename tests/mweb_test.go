package ltc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/rpcclient"
	"github.com/ltcsuite/ltcd/txscript"
	"github.com/ltcsuite/ltcd/wire"
	"github.com/test-go/testify/require"
)

// NewClient returns a new litecoin client.
func Client(t *testing.T) *rpcclient.Client {
	c, err := rpcclient.New(&rpcclient.ConnConfig{
		HTTPPostMode: true,
		DisableTLS:   true,
		Host:         "127.0.0.1:7000",
		User:         "ltc",
		Pass:         os.Getenv("password"),
	}, nil)
	require.NoError(t, err)
	return c
}

// Transaction maps an entry in the 'ltc' and 'ltc_tx_out' database table.
// Nullable fields have pointer types.
type Transaction struct {
	Hash                 string               `db:"hash" csv:"hash"`
	BlockNumber          *uint64              `db:"block_number" csv:"-"`
	InputsTotalAmount    uint64               `db:"_" csv:"-"`
	OutputsTotalAmount   uint64               `db:"_" csv:"-"`
	Fee                  *uint64              `db:"_" csv:"-"`
	Broadcasted          bool                 `db:"broadcasted" csv:"-"`
	RawTransaction       *string              `db:"rawtx" csv:"rawtx,omitempty"`
	Status               string               `db:"status" csv:"-"`
	Message              *string              `db:"message" csv:"-"`
	InputTransactions    []*InputTransaction  `db:"_" csv:"-"`
	OutputTransactions   []*OutputTransaction `db:"_" csv:"-"`
	Timestamp            *time.Time           `db:"timestamp" csv:"-"`
	CreatedAt            *time.Time           `db:"created_at" csv:"-"`
	CreatedAtBlockNumber *uint64              `db:"created_at_block" csv:"created_at_block,omitempty"`
	BroadcastedAtBlock   *uint64              `db:"broadcasted_at_block" csv:"-"`
	Attributes           map[string]string    `db:"_" csv:"-"`

	BlockHash         *string `db:"_" csv:"-"`
	PreviousBlockHash *string `db:"_" csv:"-"`
}

// InputTransaction maps an entry in the 'ltc_tx_out' database table.
// Nullable fields have pointer types.
type InputTransaction struct {
	Index         uint32  `db:"idx"`
	Address       *string `db:"address"`
	Value         *uint64 `db:"value"`
	PreviousHash  *string `db:"spent_hash"`
	PreviousIndex *uint32 `db:"spent_idx"`
}

// OutputTransaction maps an entry in the 'ltc_tx_out' database table.
// Nullable fields have pointer types.
type OutputTransaction struct {
	Index        uint32     `db:"idx"`
	Address      *string    `db:"address"`
	Value        *uint64    `db:"value"`
	SpentByHash  *string    `db:"spent_hash"`
	SpentByIndex *uint32    `db:"spent_idx"`
	Timestamp    *time.Time `db:"timestamp"`
	BlockNumber  *uint64    `db:"block_number"`
}

// UTXO maps an entry in the 'ltc_tx_out' database table.
// Nullable fields have pointer types.
type UTXO struct {
	Hash           string     `db:"hash" csv:"hash"`
	Index          uint32     `db:"idx" csv:"idx"`
	BlockNumber    *uint64    `db:"block_number" csv:"block_number,omitempty"`
	Address        *string    `db:"address" csv:"address,omitempty"`
	Value          *uint64    `db:"value" csv:"value,omitempty"`
	SpentByHash    *string    `db:"spent_hash" csv:"spent_hash,omitempty"`
	SpentByIndex   *uint32    `db:"spent_idx" csv:"spent_idx,omitempty"`
	SpentBlock     *uint64    `db:"spent_block" csv:"spent_block,omitempty"`
	Timestamp      *time.Time `db:"timestamp" csv:"timestamp,omitempty"`
	SpentTimestamp *time.Time `db:"spent_timestamp" csv:"spent_timestamp,omitempty"`
	CreatedAt      *time.Time `db:"created_at" csv:"created_at,omitempty"`
	Pinned         bool       `db:"pinned" csv:"pinned"`
	SpentPinned    bool       `db:"spent_pinned" csv:"spent_pinned"`
}

func processTransactions(blockNumber uint64, timestamp time.Time, wireTransaction *wire.MsgTx) (*Transaction, []*UTXO, []*UTXO) {
	hash := wireTransaction.TxHash().String()
	transaction := &Transaction{
		Hash:        hash,
		BlockNumber: &blockNumber,
		Timestamp:   &timestamp,
	}
	inputTransactions := []*UTXO{}
	outputTransactions := []*UTXO{}

	for i, txIn := range wireTransaction.TxIn {
		prevHash := txIn.PreviousOutPoint.Hash.String()
		// Coinbase transaction.
		if prevHash == "0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}

		spentByIndex := uint32(i)
		inputTransaction := &UTXO{
			Hash:           prevHash,
			Index:          txIn.PreviousOutPoint.Index,
			SpentBlock:     &blockNumber,
			SpentByHash:    &hash,
			SpentByIndex:   &spentByIndex,
			SpentTimestamp: &timestamp,
		}
		inputTransactions = append(inputTransactions, inputTransaction)
	}

	for i, txout := range wireTransaction.TxOut {
		var address string
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(txout.PkScript, &chaincfg.MainNetParams)
		if err == nil && len(addrs) > 0 {
			address = addrs[0].EncodeAddress()
		}
		value := uint64(txout.Value)
		index := uint32(i)
		outputTransaction := &UTXO{
			Hash:         hash,
			Index:        index,
			Address:      &address,
			BlockNumber:  &blockNumber,
			SpentByIndex: nil,
			Value:        &value,
			Timestamp:    &timestamp,
		}
		outputTransactions = append(outputTransactions, outputTransaction)
	}

	return transaction, inputTransactions, outputTransactions
}

func TestMWEB(t *testing.T) {
	for bn := 2265984; bn < 2265984+100; bn++ {
		c := Client(t)
		h, err := c.GetBlockHash(int64(bn))
		require.NoError(t, err)
		b, err := c.GetBlock(h)
		require.NoError(t, err)
		for i, wireTransaction := range b.Transactions {
			if i == len(b.Transactions)-1 || i%20 == 0 {
				fmt.Println("hash:", wireTransaction.TxHash())
			}
			_, _, outs := processTransactions(uint64(bn), b.Header.Timestamp, wireTransaction)
			for _, output := range outs {
				require.NotNil(t, output.Address)
				if strings.HasPrefix(*output.Address, "ltc1f") || strings.HasPrefix(*output.Address, "ltc1g") {
					fmt.Println("addresses:", *output.Address)
				}
			}
		}
	}
}

func DoubleSha(b []byte) []byte {
	h := sha256.Sum256(b)
	h = sha256.Sum256(h[:])
	return reverse(h[:])
}

func reverse(b []byte) []byte {
	n := len(b)
	for i := 0; i < n/2; i++ {
		b[i], b[n-1-i] = b[n-1-i], b[i]
	}
	return b
}

func TestRev(t *testing.T) {
	s := "718ad33102000000"
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	fmt.Println(hex.EncodeToString(reverse(b)))
}
