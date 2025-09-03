package dammV2

import (
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

type DammV2 struct {
	wsClient  *ws.Client
	rpcClient *rpc.Client

	poolCreator *solana.Wallet

	poolAuthority  solana.PublicKey
	eventAuthority solana.PublicKey
}

func NewDammV2(
	wsClient *ws.Client,
	rpcClient *rpc.Client,
	poolCreator *solana.Wallet,
) (*DammV2, error) {

	poolAuthority, err := cp_amm.DerivePoolAuthorityPDA()
	if err != nil {
		return nil, err
	}

	eventAuthority, err := cp_amm.DeriveEventAuthorityPDA()
	if err != nil {
		return nil, err
	}

	m := &DammV2{
		wsClient:       wsClient,
		rpcClient:      rpcClient,
		poolCreator:    poolCreator,
		poolAuthority:  poolAuthority,
		eventAuthority: eventAuthority,
	}

	return m, nil
}
