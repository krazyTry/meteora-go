package dynamic_bonding_curve

import (
	"context"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
)

type DynamicBondingCurve struct {
	RPC           *rpc.Client
	PoolAuthority solanago.PublicKey
	Commitment    rpc.CommitmentType
}

func NewDynamicBondingCurve(rpcClient *rpc.Client, commitment rpc.CommitmentType) *DynamicBondingCurve {
	return &DynamicBondingCurve{
		RPC:           rpcClient,
		PoolAuthority: helpers.DeriveDbcPoolAuthority(),
		Commitment:    commitment,
	}
}

func (p *DynamicBondingCurve) PrepareTokenAccounts(ctx context.Context, owner, payer, tokenAMint, tokenBMint, tokenAProgram, tokenBProgram solanago.PublicKey) (ataTokenA, ataTokenB solanago.PublicKey, instructions []solanago.Instruction, err error) {
	instructions = make([]solanago.Instruction, 0)
	ataTokenA, ixA, err := helpers.GetOrCreateATAInstruction(ctx, p.RPC, tokenAMint, owner, payer, tokenAProgram)
	if err != nil {
		return
	}
	ataTokenB, ixB, err := helpers.GetOrCreateATAInstruction(ctx, p.RPC, tokenBMint, owner, payer, tokenBProgram)
	if err != nil {
		return
	}
	if ixA != nil {
		instructions = append(instructions, ixA)
	}
	if ixB != nil {
		instructions = append(instructions, ixB)
	}
	return
}
