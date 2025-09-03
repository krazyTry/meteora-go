package dammV2

import (
	"context"
	"fmt"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func cpAmmRefreshVesting(
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	owner solana.PublicKey,
	vestingAccounts []*solana.AccountMeta,
) (solana.Instruction, error) {
	return cp_amm.NewRefreshVestingInstruction(
		cpammPool,
		position,
		positionNftAccount,
		owner,
		vestingAccounts,
	)
}

func (m *DammV2) GetVestingsByPosition(ctx context.Context, position solana.PublicKey) ([]*cp_amm.Vesting, error) {
	opt := solanago.GenProgramAccountFilter(
		cp_amm.AccountKeyVesting,
		&solanago.Filter{
			Owner:  position,
			Offset: 8,
		},
	)

	outs, err := m.rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*cp_amm.Vesting
	for _, out := range outs {
		obj, err := cp_amm.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		position, ok := obj.(*cp_amm.Vesting)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Vesting) fail")
		}

		list = append(list, position)
	}

	return list, nil
}
