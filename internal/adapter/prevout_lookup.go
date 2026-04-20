package adapter

import (
	"fmt"
	"strings"
	"sync"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
)

type PrevoutValueLookup interface {
	ValueByTxOut(txID string, txIdx uint32) (int64, error)
}

type rpcPrevoutValueLookup struct {
	client *rpcclient.Client
	cache  sync.Map // map[txid:index]int64
}

func NewRPCPrevoutValueLookup(host, user, pass string, httpPostMode, disableTLS bool) (PrevoutValueLookup, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, nil
	}

	connConfig := &rpcclient.ConnConfig{
		Host:                 host,
		User:                 strings.TrimSpace(user),
		Pass:                 strings.TrimSpace(pass),
		HTTPPostMode:         httpPostMode,
		DisableTLS:           disableTLS,
		DisableAutoReconnect: false,
		DisableConnectOnNew:  false,
	}
	client, err := rpcclient.New(connConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("create rpc prevout lookup failed: %w", err)
	}
	return &rpcPrevoutValueLookup{client: client}, nil
}

func (l *rpcPrevoutValueLookup) ValueByTxOut(txID string, txIdx uint32) (int64, error) {
	if l == nil || l.client == nil {
		return 0, fmt.Errorf("rpc prevout lookup is not initialized")
	}

	key := fmt.Sprintf("%s:%d", strings.TrimSpace(txID), txIdx)
	if value, ok := l.cache.Load(key); ok {
		cached, ok := value.(int64)
		if ok {
			return cached, nil
		}
	}

	hash, err := chainhash.NewHashFromStr(txID)
	if err != nil {
		return 0, fmt.Errorf("parse txid failed: %w", err)
	}
	tx, err := l.client.GetRawTransaction(hash)
	if err != nil {
		return 0, fmt.Errorf("get raw transaction failed: %w", err)
	}

	if int(txIdx) >= len(tx.MsgTx().TxOut) {
		return 0, fmt.Errorf("tx output index out of range: txid=%s idx=%d out_len=%d", txID, txIdx, len(tx.MsgTx().TxOut))
	}
	value := tx.MsgTx().TxOut[txIdx].Value
	l.cache.Store(key, value)
	return value, nil
}
