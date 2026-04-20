package adapter

import (
	"context"
	"encoding/json"
)

type PinRecord struct {
	ID            string          `json:"id"`
	Path          string          `json:"path"`
	MetaID        string          `json:"metaid"`
	CreateMetaID  string          `json:"createMetaId"`
	GlobalMetaID  string          `json:"globalMetaId"`
	CreateAddress string          `json:"createAddress"`
	ChainName     string          `json:"chainName"`
	ContentBody   json.RawMessage `json:"contentBody,omitempty"`
	IsTransfered  bool            `json:"isTransfered"`
}

type MempoolMessage struct {
	Chain   string          `json:"chain"`
	PinList []*PinRecord    `json:"pinList"`
	Tx      json.RawMessage `json:"tx,omitempty"`
}

type ChainZMQAdapter interface {
	Name() string
	Run(ctx context.Context, out chan<- MempoolMessage) error
}
