package dammV2

import (
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

var (
	poolAuthority  solana.PublicKey
	eventAuthority solana.PublicKey

	transferFee = uint64(5000) // 0.000005 SOL
)

func Init() error {
	var err error
	poolAuthority, err = cp_amm.DerivePoolAuthorityPDA()
	if err != nil {
		return err
	}

	eventAuthority, err = cp_amm.DeriveEventAuthorityPDA()
	if err != nil {
		return err
	}
	return nil
}

type DammV2 struct {
	wsClient  *ws.Client
	rpcClient *rpc.Client

	poolCreator *solana.Wallet
}

func NewDammV2(
	wsClient *ws.Client,
	rpcClient *rpc.Client,
	poolCreator *solana.Wallet,
) (*DammV2, error) {

	m := &DammV2{
		wsClient:    wsClient,
		rpcClient:   rpcClient,
		poolCreator: poolCreator,
	}
	return m, nil
}
