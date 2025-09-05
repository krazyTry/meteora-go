package dbc

import (
	"context"
	"fmt"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/shopspring/decimal"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	token "github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func SwapInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) ([]solana.Instruction, error) {
	// check if rate limiter is applied if:
	// 1. rate limiter mode
	// 2. swap direction is QuoteToBase
	// 3. current point is greater than activation point
	// 4. current point is less than activation point + maxLimiterDuration
	isRateLimiterApplied := dbc.CheckRateLimiterApplied(
		configState.PoolFees.BaseFee.BaseFeeMode,
		swapBaseForQuote,
		decimal.NewFromBigInt(currentPoint, 0),
		decimal.NewFromUint64(poolState.ActivationPoint),
		decimal.NewFromUint64(configState.PoolFees.BaseFee.SecondFactor),
	)

	inputMint, outputMint, inputMintProgram, outputMintProgram := dbc.PrepareSwapParams(swapBaseForQuote, poolState, configState)

	var instructions []solana.Instruction

	inputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, inputMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	outputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, outputMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	baseMint := poolState.BaseMint
	quoteMint := configState.QuoteMint

	referralTokenAccount := solana.PublicKey{}

	if !referrer.Equals(solana.PublicKey{}) {
		switch configState.CollectFeeMode {
		case dbc.CollectFeeModeQuoteToken:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, rpcClient, referrer, quoteMint, payer, &instructions)
			if err != nil {
				return nil, err
			}
		case dbc.CollectFeeModeOutputToken:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, rpcClient, referrer, baseMint, payer, &instructions)
			if err != nil {
				return nil, err
			}
		}
	}

	// TODO 逻辑复杂 值得优化
	switch {
	case inputMint.Equals(solana.WrappedSol):
		if amountIn.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		totalAmount := amountIn.Uint64() // + rentExemptAmount

		// wrap SOL by transferring lamports into the WSOL ATA

		wrapSOLIx := system.NewTransferInstruction(
			totalAmount,
			owner,
			inputTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			inputTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	case outputMint.Equals(solana.WrappedSol):
	}

	baseVault := poolState.BaseVault   // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)
	quoteVault := poolState.QuoteVault // dbc.DeriveTokenVaultPDA(pool, config.QuoteMint)

	var remainingAccounts []*solana.AccountMeta
	if isRateLimiterApplied {
		remainingAccounts = []*solana.AccountMeta{
			solana.NewAccountMeta(solana.SysVarInstructionsPubkey, false, false),
		}
	}

	params := dbc.SwapParameters{
		AmountIn:         amountIn.Uint64(),
		MinimumAmountOut: minimumAmountOut.Uint64(),
	}

	swapIx, err := dbc.NewSwapInstruction(
		params,
		poolAuthority,
		poolState.Config,
		poolAddress,
		inputTokenAccount,
		outputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		payer,
		inputMintProgram,
		outputMintProgram,
		referralTokenAccount,
		eventAuthority,
		dbc.ProgramID,
		remainingAccounts,
	)

	if err != nil {
		return nil, err
	}
	instructions = append(instructions, swapIx)

	switch {
	case inputMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			inputTokenAccount,
			owner,
			owner,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	case outputMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			outputTokenAccount,
			owner,
			owner,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if configState.CollectFeeMode == dbc.CollectFeeModeQuoteToken && !referrer.Equals(solana.PublicKey{}) {
		unwrapIx := token.NewCloseAccountInstruction(
			referralTokenAccount,
			referrer,
			referrer,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	return instructions, nil
}

func (m *DBC) Swap(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {

	instructions, err := SwapInstruction(ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		func() solana.PublicKey {
			if referrer == nil {
				return solana.PublicKey{}
			}
			return referrer.PublicKey()
		}(),
		poolAddress,
		poolState,
		configState,
		swapBaseForQuote,
		amountIn,
		minimumAmountOut,
		currentPoint,
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
			case referrer != nil && key.Equals(referrer.PublicKey()):
				return &referrer.PrivateKey
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

func BuyInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	buyer solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) ([]solana.Instruction, error) {
	return SwapInstruction(
		ctx,
		rpcClient,
		buyer,
		buyer,
		referrer,
		poolAddress,
		poolState,
		configState,
		false,
		amountIn,
		minimumAmountOut,
		currentPoint,
	)
}

func (m *DBC) Buy(
	ctx context.Context,
	buyer *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {
	return m.Swap(
		ctx,
		buyer,
		buyer,
		referrer,
		poolAddress,
		poolState,
		configState,
		false,
		amountIn,
		minimumAmountOut,
		currentPoint,
	)
}

func SellInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	seller solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) ([]solana.Instruction, error) {
	return SwapInstruction(
		ctx,
		rpcClient,
		seller,
		seller,
		referrer,
		poolAddress,
		poolState,
		configState,
		true,
		amountIn,
		minimumAmountOut,
		currentPoint,
	)
}

func (m *DBC) Sell(
	ctx context.Context,
	seller *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {
	return m.Swap(
		ctx,
		seller,
		seller,
		referrer,
		poolAddress,
		poolState,
		configState,
		true,
		amountIn,
		minimumAmountOut,
		currentPoint,
	)
}
