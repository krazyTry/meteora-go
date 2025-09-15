package dammV2

import (
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	poolAuthority  solana.PublicKey
	eventAuthority solana.PublicKey

	transferFee = uint64(5000) // 0.000005 SOL
)

// Init performs initialization.
// It completes the generation of poolAuthority, eventAuthority in the damm v2 pool.
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
	rpcClient   *rpc.Client
	poolCreator *solana.Wallet
}

func NewDammV2(
	rpcClient *rpc.Client,
	poolCreator *solana.Wallet,
) (*DammV2, error) {
	return &DammV2{
		rpcClient:   rpcClient,
		poolCreator: poolCreator,
	}, nil
}
