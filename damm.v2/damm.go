package dammV2

import (
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	poolAuthority  solana.PublicKey
	eventAuthority solana.PublicKey

	rentExemptFee = uint64(2_039_280)
	transferFee   = uint64(5000) // 0.000005 SOL
)

// Init performs initialization.
// It completes the generation of poolAuthority, eventAuthority in the damm v2 pool.
func init() {
	var err error
	poolAuthority, err = cp_amm.DerivePoolAuthorityPDA()
	if err != nil {
		panic(err)
	}

	eventAuthority, err = cp_amm.DeriveEventAuthorityPDA()
	if err != nil {
		panic(err)
	}
}

type DammV2 struct {
	rpcClient   *rpc.Client
	poolCreator *solana.Wallet
}

func NewDammV2(
	rpcClient *rpc.Client,
	opts ...Option,
) *DammV2 {
	o := &DammV2{
		rpcClient: rpcClient,
	}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

type Option func(*DammV2)

func WithCreator(poolCreator *solana.Wallet) Option {
	return func(dbc *DammV2) {
		dbc.poolCreator = poolCreator
	}
}
