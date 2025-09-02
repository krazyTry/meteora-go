package dbc

import (
	"context"
	"fmt"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	token "github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
)

func dbcSwap(m *DBC,
	config solana.PublicKey,
	dbcPool solana.PublicKey,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseVault solana.PublicKey,
	quoteVault solana.PublicKey,
	payer solana.PublicKey,
	referralTokenAccount solana.PublicKey,
	inputTokenAccount solana.PublicKey,
	outputTokenAccount solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	amountIn uint64,
	minOut uint64,
	remainingAccounts []*solana.AccountMeta,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID

	// Params:
	params := dbc.SwapParameters{
		AmountIn:         amountIn,
		MinimumAmountOut: minOut,
	}

	return dbc.NewSwapInstruction(
		params,
		poolAuthority,
		config,
		dbcPool,
		inputTokenAccount,
		outputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		payer,
		tokenBaseProgram,
		tokenQuoteProgram,
		referralTokenAccount,
		eventAuthority,
		program,
		remainingAccounts,
	)
}

func (m *DBC) SwapInstruction(ctx context.Context,
	payer solana.PublicKey,
	owner solana.PublicKey,
	referrer solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
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
		config.PoolFees.BaseFee.BaseFeeMode,
		swapBaseForQuote,
		currentPoint,
		new(big.Int).SetUint64(virtualPool.ActivationPoint),
		new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor),
	)

	inputMint, outputMint, inputMintProgram, outputMintProgram := dbc.PrepareSwapParams(swapBaseForQuote, virtualPool, config)

	var instructions []solana.Instruction

	inputTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner, inputMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	outputTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner, outputMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	baseMint := virtualPool.BaseMint
	quoteMint := config.QuoteMint

	referralTokenAccount := solana.PublicKey{}

	if !referrer.Equals(solana.PublicKey{}) {
		switch config.CollectFeeMode {
		case dbc.CollectFeeModeQuoteToken:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, m.rpcClient, referrer, quoteMint, payer, &instructions)
			if err != nil {
				return nil, err
			}
		case dbc.CollectFeeModeOutputToken:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, m.rpcClient, referrer, baseMint, payer, &instructions)
			if err != nil {
				return nil, err
			}
		}
	}

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

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	baseVault := virtualPool.BaseVault   // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)
	quoteVault := virtualPool.QuoteVault // dbc.DeriveTokenVaultPDA(pool, config.QuoteMint)

	var remainingAccounts []*solana.AccountMeta
	if isRateLimiterApplied {
		remainingAccounts = []*solana.AccountMeta{
			solana.NewAccountMeta(solana.SysVarInstructionsPubkey, false, false),
		}
	}

	swapIx, err := dbcSwap(m,
		virtualPool.Config,
		pool,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		payer,
		referralTokenAccount,
		inputTokenAccount,
		outputTokenAccount,
		inputMintProgram,
		outputMintProgram,
		amountIn.Uint64(),
		minimumAmountOut.Uint64(),
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

	if config.CollectFeeMode == dbc.CollectFeeModeQuoteToken && !referrer.Equals(solana.PublicKey{}) {
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

func (m *DBC) Swap(ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	referrer *solana.Wallet,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {

	instructions, err := m.SwapInstruction(ctx,
		payer.PublicKey(),
		owner.PublicKey(),
		func() solana.PublicKey {
			if referrer == nil {
				return solana.PublicKey{}
			}
			return referrer.PublicKey()
		}(),
		virtualPool,
		config,
		swapBaseForQuote,
		amountIn,
		minimumAmountOut,
		currentPoint,
	)

	if err != nil {
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
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
	}); err != nil {
		return "", err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", err
		}
		return "-", nil
	}

	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}

func (m *DBC) BuyInstruction(ctx context.Context,
	buyer solana.PublicKey,
	referrer solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) ([]solana.Instruction, error) {
	return m.SwapInstruction(ctx, buyer, buyer, referrer, virtualPool, config, false, amountIn, minimumAmountOut, currentPoint)
}

func (m *DBC) Buy(ctx context.Context,
	buyer *solana.Wallet,
	referrer *solana.Wallet,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {
	return m.Swap(ctx, buyer, buyer, referrer, virtualPool, config, false, amountIn, minimumAmountOut, currentPoint)
}

func (m *DBC) SellInstruction(ctx context.Context,
	seller solana.PublicKey,
	referrer solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) ([]solana.Instruction, error) {
	return m.SwapInstruction(ctx, seller, seller, referrer, virtualPool, config, true, amountIn, minimumAmountOut, currentPoint)
}

func (m *DBC) Sell(ctx context.Context,
	seller *solana.Wallet,
	referrer *solana.Wallet,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {
	return m.Swap(ctx, seller, seller, referrer, virtualPool, config, true, amountIn, minimumAmountOut, currentPoint)
}
