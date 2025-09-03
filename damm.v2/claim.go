package dammV2

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func cpAmmClaimPositionFee(
	m *DammV2,
	cpammPool solana.PublicKey,
	owner solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseVault solana.PublicKey,
	quoteVault solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewClaimPositionFeeInstruction(
		poolAuthority,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) ClaimPositionFeeInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
) ([]solana.Instruction, error) {
	baseMint := virtualPool.TokenAMint  // baseMint
	quoteMint := virtualPool.TokenBMint // solana.WrappedSol

	baseVault := virtualPool.TokenAVault  // dbc.DeriveTokenVaultPDA(pool, baseMint)
	quoteVault := virtualPool.TokenBVault // dbc.DeriveTokenVaultPDA(pool, quoteMint)

	tokenBaseProgram := cp_amm.GetTokenProgram(virtualPool.TokenAFlag)
	tokenQuoteProgram := cp_amm.GetTokenProgram(virtualPool.TokenBFlag)

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := cpAmmClaimPositionFee(
		m,
		virtualPool.Address,
		owner.PublicKey(),
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		tokenBaseProgram,
		tokenQuoteProgram,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, claimIx)

	switch {
	case baseMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			tokenBaseAccount,
			owner.PublicKey(),
			owner.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	case quoteMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			owner.PublicKey(),
			owner.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}
	return instructions, nil
}

func (m *DammV2) ClaimPositionFee(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := m.ClaimPositionFeeInstruction(
		ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
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

func cpAmmClaimReward(
	m *DammV2,
	// Params:
	rewardIndex uint8,
	skipReward uint8,

	// Accounts:
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	rewardVault solana.PublicKey,
	rewardMint solana.PublicKey,
	userTokenAccount solana.PublicKey,
	positionNftAccount solana.PublicKey,
	owner solana.PublicKey,
	tokenProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewClaimRewardInstruction(
		// Params:
		rewardIndex,
		skipReward,

		// Accounts:
		poolAuthority,
		cpammPool,
		position,
		rewardVault,
		rewardMint,
		userTokenAccount,
		positionNftAccount,
		owner,
		tokenProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) ClaimRewardInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
	rewardIndex uint8,
	skipReward uint8,
) ([]solana.Instruction, error) {
	if len(virtualPool.RewardInfos) < int(rewardIndex) {
		return nil, fmt.Errorf("len(userPosition.PositionState.RewardInfos) < int(rewardIndex)")
	}
	rewardInfo := virtualPool.RewardInfos[rewardIndex]

	var instructions []solana.Instruction
	userTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), rewardInfo.Mint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := cpAmmClaimReward(m,
		rewardIndex,
		skipReward,
		virtualPool.Address,
		ownerPosition.Position,
		rewardInfo.Vault,
		rewardInfo.Mint,
		userTokenAccount,
		ownerPosition.PositionNftAccount,
		owner.PublicKey(),
		cp_amm.GetTokenProgram(rewardInfo.RewardTokenFlag),
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, claimIx)

	if rewardInfo.Mint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			userTokenAccount,
			owner.PublicKey(),
			owner.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}
	return instructions, nil
}

func (m *DammV2) ClaimReward(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	rewardIndex uint8,
	skipReward uint8,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]
	instructions, err := m.ClaimRewardInstruction(
		ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
		rewardIndex,
		skipReward,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
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

func (m *DammV2) GetUnclaimedFee(virtualPool *cp_amm.Pool, position *cp_amm.Position) (uint64, uint64) {
	feeBaseToken, feeQuoteToken := cp_amm.CalculateUnClaimFee(virtualPool, position)
	return feeBaseToken.Uint64(), feeQuoteToken.Uint64()
}

func (m *DammV2) GetUnclaimedRewards(virtualPool *cp_amm.Pool, position *cp_amm.Position) []uint64 {
	rewards := cp_amm.CalculateUnClaimReward(virtualPool, position)
	var list []uint64
	for _, v := range rewards {
		list = append(list, v.Uint64())
	}
	return list
}
