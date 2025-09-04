package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/u128"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
)

func cpAmmAddLiquidity(
	m *DammV2,
	// delta liquidity
	liquidityDelta *big.Int,
	// maximum token a amount
	tokenBaseAmountThreshold uint64,
	// maximum token b amount
	tokenQuoteAmountThreshold uint64,

	// Accounts:
	owner solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	params := cp_amm.AddLiquidityParameters{
		LiquidityDelta:        u128.GenUint128FromString(liquidityDelta.String()),
		TokenAAmountThreshold: tokenBaseAmountThreshold,
		TokenBAmountThreshold: tokenQuoteAmountThreshold,
	}

	return cp_amm.NewAddLiquidityInstruction(
		params,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseMint,
		tokenQuoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) AddPositionLiquidityInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
	bAddBase bool,
	amountIn *big.Int,
	liquidityDelta *big.Int,
	minOutAmount *big.Int,
) ([]solana.Instruction, error) {
	if amountIn.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("amountIn must be greater than 0")
	}

	baseMint := virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	baseVault := virtualPool.TokenAVault
	quoteVault := virtualPool.TokenBVault

	var instructions []solana.Instruction

	// cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	// if err != nil {
	// 	return nil, err
	// }
	cpammPool := virtualPool.Address

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	var (
		tokenBaseAmountThreshold  uint64
		tokenQuoteAmountThreshold uint64
	)

	if bAddBase {
		tokenBaseAmountThreshold = amountIn.Uint64()      //cp_amm.U64_MAX
		tokenQuoteAmountThreshold = minOutAmount.Uint64() //minOutAmount.Uint64()
	} else {
		tokenBaseAmountThreshold = minOutAmount.Uint64() // minOutAmount.Uint64()
		tokenQuoteAmountThreshold = amountIn.Uint64()    // cp_amm.U64_MAX
	}

	if baseMint.Equals(solana.WrappedSol) {
		// wrap SOL by transferring lamports into the WSOL ATA
		wrapSOLIx := system.NewTransferInstruction(
			tokenBaseAmountThreshold,
			owner.PublicKey(),
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		// wrap SOL by transferring lamports into the WSOL ATA
		wrapSOLIx := system.NewTransferInstruction(
			tokenQuoteAmountThreshold,
			owner.PublicKey(),
			quoteTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			quoteTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	liquidityIx, err := cpAmmAddLiquidity(m,
		liquidityDelta,
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,
		owner.PublicKey(),
		cpammPool,
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		baseTokenAccount,
		quoteTokenAccount,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
		cp_amm.GetTokenProgram(virtualPool.TokenBFlag),
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			owner.PublicKey(),
			owner.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			owner.PublicKey(),
			owner.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	return instructions, nil
}

func (m *DammV2) AddPositionLiquidity(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	bAddBase bool,
	amountIn *big.Int,
	liquidityDelta *big.Int,
	minOutAmount *big.Int,
) (string, error) {
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := m.AddPositionLiquidityInstruction(ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
		bAddBase,
		amountIn,
		liquidityDelta,
		minOutAmount,
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

func cpAmmRemoveLiquidity(
	m *DammV2,
	// delta liquidity
	liquidityDelta *big.Int,
	// maximum token a amount
	tokenBaseAmountThreshold uint64,
	// maximum token b amount
	tokenQuoteAmountThreshold uint64,

	// Accounts:
	owner solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewRemoveLiquidityInstruction(
		cp_amm.RemoveLiquidityParameters{
			LiquidityDelta:        u128.GenUint128FromString(liquidityDelta.String()),
			TokenAAmountThreshold: tokenBaseAmountThreshold,
			TokenBAmountThreshold: tokenQuoteAmountThreshold,
		},
		// Accounts:
		poolAuthority,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseMint,
		tokenQuoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) RemovePositionLiquidityInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
	liquidityDelta *big.Int,
	tokenBaseAmountThreshold *big.Int,
	tokenQuoteAmountThreshold *big.Int,
	vestings []*Vesting,
) ([]solana.Instruction, error) {

	currentPoint, err := solanago.CurrenPoint(ctx, m.rpcClient, uint8(virtualPool.ActivationType))
	if err != nil {
		return nil, err
	}

	if err = canUnlockPosition(ownerPosition.PositionState, vestings, currentPoint); err != nil {
		return nil, err
	}

	baseMint := virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	baseVault := virtualPool.TokenAVault
	quoteVault := virtualPool.TokenBVault

	var instructions []solana.Instruction

	// cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	// if err != nil {
	// 	return nil, err
	// }
	cpammPool := virtualPool.Address

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	if len(vestings) > 0 {
		var vestingAccounts []*solana.AccountMeta
		for _, v := range vestings {
			vestingAccounts = []*solana.AccountMeta{
				solana.NewAccountMeta(v.Vesting, false, false),
			}
		}
		refreshVestingIx, err := cpAmmRefreshVesting(cpammPool, ownerPosition.Position, ownerPosition.PositionNftAccount, owner.PublicKey(), vestingAccounts)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, refreshVestingIx)
	}

	liquidityIx, err := cpAmmRemoveLiquidity(m,
		liquidityDelta,
		tokenBaseAmountThreshold.Uint64(),
		tokenQuoteAmountThreshold.Uint64(),
		owner.PublicKey(),
		cpammPool,
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		baseTokenAccount,
		quoteTokenAccount,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
		cp_amm.GetTokenProgram(virtualPool.TokenBFlag),
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	unwrapIx := token.NewCloseAccountInstruction(
		quoteTokenAccount,
		owner.PublicKey(),
		owner.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, unwrapIx)
	return instructions, nil
}

func (m *DammV2) RemovePositionLiquidity(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	liquidityDelta *big.Int,
	tokenBaseAmountThreshold *big.Int,
	tokenQuoteAmountThreshold *big.Int,
	vestings []*Vesting,
) (string, error) {
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := m.RemovePositionLiquidityInstruction(ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
		liquidityDelta,
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,
		vestings,
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

func cpAmmRemoveAllLiquidity(
	m *DammV2,
	// Params:
	tokenBaseAmountThreshold uint64,
	tokenQuoteAmountThreshold uint64,

	// Accounts:
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	positionNftAccount solana.PublicKey,
	owner solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewRemoveAllLiquidityInstruction(
		// Params:
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,

		// Accounts:
		poolAuthority,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseMint,
		tokenQuoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) RemoveAllLiquidityInstruction(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	ownerPosition *UserPosition,
	virtualPool *Pool,
	vestings []*Vesting,
) ([]solana.Instruction, error) {

	currentPoint, err := solanago.CurrenPoint(ctx, m.rpcClient, uint8(virtualPool.ActivationType))
	if err != nil {
		return nil, err
	}

	if err := canUnlockPosition(ownerPosition.PositionState, vestings, currentPoint); err != nil {
		return nil, err
	}

	baseMint := virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	baseVault := virtualPool.TokenAVault
	quoteVault := virtualPool.TokenBVault

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	if len(vestings) > 0 {
		var vestingAccounts []*solana.AccountMeta
		for _, v := range vestings {
			vestingAccounts = []*solana.AccountMeta{
				solana.NewAccountMeta(v.Vesting, false, false),
			}
		}
		refreshVestingIx, err := cpAmmRefreshVesting(virtualPool.Address, ownerPosition.Position, ownerPosition.PositionNftAccount, owner.PublicKey(), vestingAccounts)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, refreshVestingIx)
	}

	var (
		tokenBaseAmountThreshold  uint64
		tokenQuoteAmountThreshold uint64
	)
	liquidityIx, err := cpAmmRemoveAllLiquidity(m,
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,
		virtualPool.Address,
		ownerPosition.Position,
		baseTokenAccount,
		quoteTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		ownerPosition.PositionNftAccount,
		owner.PublicKey(),
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
		cp_amm.GetTokenProgram(virtualPool.TokenBFlag),
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	// unwrapIx := token.NewCloseAccountInstruction(
	// 	quoteTokenAccount,
	// 	owner.PublicKey(),
	// 	owner.PublicKey(),
	// 	[]solana.PublicKey{},
	// ).Build()
	// instructions = append(instructions, unwrapIx)
	return instructions, nil
}
func (m *DammV2) RemoveAllLiquidity(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	vestings []*Vesting,
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

	instructions, err := m.RemoveAllLiquidityInstruction(
		ctx,
		payer,
		owner,
		userPosition,
		virtualPool,
		vestings,
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

func (m *DammV2) GetPositionLiquidity(ctx context.Context, baseMint solana.PublicKey, owner solana.PublicKey) (*big.Int, *UserPosition, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return nil, nil, err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, virtualPool.Address, owner)
	if err != nil {
		return nil, nil, err
	}
	if len(userPositions) == 0 {
		return nil, nil, fmt.Errorf("no matching user_position")
	}

	userPosition = userPositions[0]
	return userPosition.PositionState.UnlockedLiquidity.BigInt(), userPosition, nil
}
