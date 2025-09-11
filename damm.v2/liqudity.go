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
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

func AddPositionLiquidityInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	bAddBase bool,
	amountIn *big.Int,
	liquidityDelta *big.Int,
	minOutAmount *big.Int,
) ([]solana.Instruction, error) {
	if amountIn.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("amountIn must be greater than 0")
	}

	baseMint := poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	baseVault := poolState.TokenAVault
	quoteVault := poolState.TokenBVault

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, quoteMint, payer, &instructions)
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
			owner,
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
			owner,
			quoteTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			quoteTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	params := cp_amm.AddLiquidityParameters{
		LiquidityDelta:        u128.GenUint128FromString(liquidityDelta.String()),
		TokenAAmountThreshold: tokenBaseAmountThreshold,
		TokenBAmountThreshold: tokenQuoteAmountThreshold,
	}

	liquidityIx, err := cp_amm.NewAddLiquidityInstruction(
		params,
		poolAddress,
		ownerPosition.Position,
		baseTokenAccount,
		quoteTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		ownerPosition.PositionNftAccount,
		owner,
		cp_amm.GetTokenProgram(poolState.TokenAFlag),
		cp_amm.GetTokenProgram(poolState.TokenBFlag),
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	return instructions, nil
}

func (m *DammV2) AddPositionLiquidity(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	owner *solana.Wallet,
	poolState *Pool,
	bAddBase bool,
	amountIn *big.Int,
	liquidityDelta *big.Int,
	minOutAmount *big.Int,
) (string, error) {
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := AddPositionLiquidityInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		poolState.Pool,
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

func RemovePositionLiquidityInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	liquidityDelta *big.Int,
	tokenBaseAmountThreshold *big.Int,
	tokenQuoteAmountThreshold *big.Int,
	vestings []*Vesting,
) ([]solana.Instruction, error) {

	currentPoint, err := solanago.CurrenPoint(ctx, rpcClient, uint8(poolState.ActivationType))
	if err != nil {
		return nil, err
	}

	if err = canUnlockPosition(ownerPosition.PositionState, vestings, currentPoint); err != nil {
		return nil, err
	}

	baseMint := poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	baseVault := poolState.TokenAVault
	quoteVault := poolState.TokenBVault

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, quoteMint, payer, &instructions)
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

		refreshVestingIx, err := cp_amm.NewRefreshVestingInstruction(
			poolAddress,
			ownerPosition.Position,
			ownerPosition.PositionNftAccount,
			owner,
			vestingAccounts,
		)

		if err != nil {
			return nil, err
		}
		instructions = append(instructions, refreshVestingIx)
	}

	liquidityIx, err := cp_amm.NewRemoveLiquidityInstruction(
		cp_amm.RemoveLiquidityParameters{
			LiquidityDelta:        u128.GenUint128FromString(liquidityDelta.String()),
			TokenAAmountThreshold: tokenBaseAmountThreshold.Uint64(),
			TokenBAmountThreshold: tokenQuoteAmountThreshold.Uint64(),
		},
		// Accounts:
		poolAuthority,
		poolAddress,
		ownerPosition.Position,
		baseTokenAccount,
		quoteTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		ownerPosition.PositionNftAccount,
		owner,
		cp_amm.GetTokenProgram(poolState.TokenAFlag),
		cp_amm.GetTokenProgram(poolState.TokenBFlag),
		eventAuthority,
		cp_amm.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}
	return instructions, nil
}

func (m *DammV2) RemovePositionLiquidity(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	owner *solana.Wallet,
	poolState *Pool,
	liquidityDelta *big.Int,
	tokenBaseAmountThreshold *big.Int,
	tokenQuoteAmountThreshold *big.Int,
	vestings []*Vesting,
) (string, error) {
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := RemovePositionLiquidityInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		poolState.Pool,
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

func RemoveAllLiquidityInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	vestings []*Vesting,
) ([]solana.Instruction, error) {

	currentPoint, err := solanago.CurrenPoint(ctx, rpcClient, uint8(poolState.ActivationType))
	if err != nil {
		return nil, err
	}

	if err := canUnlockPosition(ownerPosition.PositionState, vestings, currentPoint); err != nil {
		return nil, err
	}

	baseMint := poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	baseVault := poolState.TokenAVault
	quoteVault := poolState.TokenBVault

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, quoteMint, payer, &instructions)
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
		refreshVestingIx, err := cp_amm.NewRefreshVestingInstruction(
			poolAddress,
			ownerPosition.Position,
			ownerPosition.PositionNftAccount,
			owner,
			vestingAccounts,
		)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, refreshVestingIx)
	}

	var (
		tokenBaseAmountThreshold  uint64
		tokenQuoteAmountThreshold uint64
	)

	liquidityIx, err := cp_amm.NewRemoveAllLiquidityInstruction(
		// Params:
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,

		// Accounts:
		poolAuthority,
		poolAddress,
		ownerPosition.Position,
		baseTokenAccount,
		quoteTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		ownerPosition.PositionNftAccount,
		owner,
		cp_amm.GetTokenProgram(poolState.TokenAFlag),
		cp_amm.GetTokenProgram(poolState.TokenBFlag),
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	return instructions, nil
}
func (m *DammV2) RemoveAllLiquidity(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	vestings []*Vesting,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}
	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := RemoveAllLiquidityInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		poolState.Pool,
		vestings,
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

func (m *DammV2) GetPositionLiquidity(ctx context.Context, baseMint solana.PublicKey, owner solana.PublicKey) (*big.Int, *UserPosition, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return nil, nil, err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner)
	if err != nil {
		return nil, nil, err
	}
	if len(userPositions) == 0 {
		return nil, nil, fmt.Errorf("no matching user_position")
	}

	userPosition = userPositions[0]
	return userPosition.PositionState.UnlockedLiquidity.BigInt(), userPosition, nil
}
