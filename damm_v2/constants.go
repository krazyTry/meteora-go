package dammv2

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go/rpc"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

var (
	CpAmmProgramID = dammv2gen.ProgramID
)

func CurrentPointForActivation(ctx context.Context, client *rpc.Client, commitment rpc.CommitmentType, activationType ActivationType) *big.Int {
	slot, _ := client.GetSlot(ctx, commitment)
	if activationType == ActivationTypeSlot {
		return new(big.Int).SetUint64(slot)
	}
	bt, _ := client.GetBlockTime(ctx, slot)
	if bt != nil {
		return big.NewInt(int64(*bt))
	}
	return big.NewInt(0)
}
