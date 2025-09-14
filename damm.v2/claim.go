package dammV2

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func ClaimPositionFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
) ([]solana.Instruction, error) {
	baseMint := poolState.TokenAMint  // baseMint
	quoteMint := poolState.TokenBMint // solana.WrappedSol

	baseVault := poolState.TokenAVault  // dbc.DeriveTokenVaultPDA(pool, baseMint)
	quoteVault := poolState.TokenBVault // dbc.DeriveTokenVaultPDA(pool, quoteMint)

	tokenBaseProgram := cp_amm.GetTokenProgram(poolState.TokenAFlag)
	tokenQuoteProgram := cp_amm.GetTokenProgram(poolState.TokenBFlag)

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := cp_amm.NewClaimPositionFeeInstruction(
		poolAuthority,
		poolAddress,
		ownerPosition.Position,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		ownerPosition.PositionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		cp_amm.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, claimIx)

	switch {
	case baseMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			tokenBaseAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	case quoteMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}
	return instructions, nil
}

func (m *DammV2) ClaimPositionFee(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	poolStates, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	poolState := poolStates[0]

	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := ClaimPositionFeeInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		poolState.Pool,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func ClaimRewardInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	rewardIndex uint8,
	skipReward uint8,
) ([]solana.Instruction, error) {
	if len(poolState.RewardInfos) < int(rewardIndex) {
		return nil, fmt.Errorf("len(userPosition.PositionState.RewardInfos) < int(rewardIndex)")
	}
	rewardInfo := poolState.RewardInfos[rewardIndex]

	var instructions []solana.Instruction
	userTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, rewardInfo.Mint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := cp_amm.NewClaimRewardInstruction(
		// Params:
		rewardIndex,
		skipReward,

		// Accounts:
		poolAuthority,
		poolAddress,
		ownerPosition.Position,
		rewardInfo.Vault,
		rewardInfo.Mint,
		userTokenAccount,
		ownerPosition.PositionNftAccount,
		owner,
		cp_amm.GetTokenProgram(rewardInfo.RewardTokenFlag),
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	instructions = append(instructions, claimIx)

	if rewardInfo.Mint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			userTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}
	return instructions, nil
}

func (m *DammV2) ClaimReward(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	rewardIndex uint8,
	skipReward uint8,
) (string, error) {
	poolStates, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	poolState := poolStates[0]
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]
	instructions, err := ClaimRewardInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		poolState.Pool,
		rewardIndex,
		skipReward,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func (m *DammV2) GetUnclaimedFee(poolState *cp_amm.Pool, position *cp_amm.Position) (uint64, uint64) {
	feeBaseToken, feeQuoteToken := cp_amm.CalculateUnClaimFee(poolState, position)
	return feeBaseToken.BigInt().Uint64(), feeQuoteToken.BigInt().Uint64()
}

func (m *DammV2) GetUnclaimedRewards(poolState *cp_amm.Pool, position *cp_amm.Position) []uint64 {
	rewards := cp_amm.CalculateUnClaimReward(poolState, position)
	var list []uint64
	for _, v := range rewards {
		list = append(list, v.BigInt().Uint64())
	}
	return list
}
