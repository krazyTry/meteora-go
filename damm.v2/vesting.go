package dammV2

import (
	"context"
	"fmt"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// GetVestingsByPosition Retrieves all vesting accounts associated with a position.
// It depends on the GetVestingsByPosition function.
//
// Example:
//
// liquidityDelta, position, _ := meteoraDammV2.GetPositionLiquidity(ctx, baseMint, poolPartner.PublicKey())
//
// vestings, _ := meteoraDammV2.GetVestingsByPosition(
//
//	ctx,
//	position.Position, // position
//
// )
func (m *DammV2) GetVestingsByPosition(ctx context.Context, position solana.PublicKey) ([]*Vesting, error) {
	return GetVestingsByPosition(ctx, m.rpcClient, position)
}

// GetVestingsByPosition Retrieves all vesting accounts associated with a position.
//
// Example:
//
// liquidityDelta, position, _ := meteoraDammV2.GetPositionLiquidity(ctx, baseMint, poolPartner.PublicKey())
//
// vestings, _ := GetVestingsByPosition(
//
//	ctx,
//	rpcClient,
//	position.Position, // position
//
// )
func GetVestingsByPosition(
	ctx context.Context,
	rpcClient *rpc.Client,
	position solana.PublicKey,
) ([]*Vesting, error) {
	opt := solanago.GenProgramAccountFilter(
		cp_amm.AccountKeyVesting,
		&solanago.Filter{
			Owner:  position,
			Offset: 8,
		},
	)

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*Vesting
	for _, out := range outs {
		obj, err := cp_amm.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		vesting, ok := obj.(*cp_amm.Vesting)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Vesting) fail")
		}

		list = append(list, &Vesting{
			Vesting:      out.Pubkey,
			VestingState: vesting,
		})
	}

	return list, nil
}
