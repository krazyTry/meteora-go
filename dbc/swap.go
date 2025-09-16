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
	"github.com/gagliardetto/solana-go/rpc/ws"
)

// SwapInstruction generates the instruction needed to swap
//
// Example:
//
// result, poolState, configState, currentPoint, _ := SwapQuote(ctx, rpcClient, baseMint, false, amountIn, slippageBps, false)
//
// instructions, _ := SwapInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	owner.PublicKey(), // owner account
//	func() solana.PublicKey {
//		if referrer == nil {
//			return solana.PublicKey{}
//		}
//		return referrer.PublicKey()
//	}(), // referral account, contact meteora
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	swapBaseForQuote, // buy(quote=>base) sell(base => quote)
//	amountIn, // amount to spend on selling or buying
//	result.MinimumAmountOut, // minimum amount to receive after accounting for slippage
//	currentPoint,
//
// )
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

	switch {
	case inputMint.Equals(solana.WrappedSol):

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

	baseVault := poolState.BaseVault
	quoteVault := poolState.QuoteVault

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
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	case outputMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			outputTokenAccount,
			owner,
			owner,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if configState.CollectFeeMode == dbc.CollectFeeModeQuoteToken && !referrer.Equals(solana.PublicKey{}) {
		unwrapIx := token.NewCloseAccountInstruction(
			referralTokenAccount,
			referrer,
			referrer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	return instructions, nil
}

// Swap swaps between base and quote or quote and base on the Dynamic Bonding Curve.
// It depends on the SwapInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// result, poolState, configState, currentPoint, _ := SwapQuote(ctx, rpcClient, baseMint, false, amountIn, slippageBps, false)
//
// instructions, _ := m.Swap(
//
//	ctx,
//	wsClient,
//	payer.PublicKey(), // payer account
//	owner.PublicKey(), // owner account
//	nil, // referral account, contact meteora
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	swapBaseForQuote, // buy(quote=>base) sell(base => quote)
//	amountIn, // amount to spend on selling or buying
//	result.MinimumAmountOut, // minimum amount to receive after accounting for slippage
//	currentPoint,
//
// )
func (m *DBC) Swap(
	ctx context.Context,
	wsClient *ws.Client,
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
		wsClient,
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

// BuyInstruction generates the instruction needed to buy
//
// Example:
//
// result, poolState, configState, currentPoint, _ := BuyQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// instructions, _ := BuyInstruction(
//
//	ctx,
//	m.rpcClient,
//	buyer.PublicKey(), // payer account
//	referrer, // referral account, contact meteora
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	amountIn, // amount to spend on buying
//	result.MinimumAmountOut, // minimum amount to receive after accounting for slippage
//	currentPoint,
//
// )
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

// Buy buys base tokens using quote tokens on the Dynamic Bonding Curve.
// It depends on the BuyInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// result, poolState, configState, currentPoint, _ := BuyQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// sig, _ := meteoraDBC.Buy(
//
//	ctx,
//	wsClient,
//	ownerWallet, // buyer
//	nil, // referral account, contact meteora
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	amountIn, // amount to spend on buying
//	result.MinimumAmountOut, // minimum amount to receive after accounting for slippage
//	currentPoint,
//
// )
func (m *DBC) Buy(
	ctx context.Context,
	wsClient *ws.Client,
	buyer *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {
	// rentExemptFee, err := solanago.GetRentExempt(ctx, m.rpcClient)
	// if err != nil {
	// 	return "", err
	// }

	lamportsSOL, err := solanago.SOLBalance(ctx, m.rpcClient, buyer.PublicKey())
	if err != nil {
		return "", err
	}

	if lamportsSOL < rentExemptFee+transferFee {
		return "", fmt.Errorf("buyer sol must be greater than %v", (rentExemptFee+transferFee)/1e9)
	}

	if amountIn.Cmp(new(big.Int).SetUint64(lamportsSOL)) > 0 {
		return "", fmt.Errorf("amountIn must be greater than %v SOL", (rentExemptFee+transferFee+1)/1e9)
	}

	return m.Swap(
		ctx,
		wsClient,
		buyer,
		buyer,
		referrer,
		poolAddress,
		poolState,
		configState,
		false,
		new(big.Int).Sub(amountIn, new(big.Int).SetUint64(rentExemptFee+transferFee)),
		minimumAmountOut,
		currentPoint,
	)
}

// SellInstruction generates the instruction needed to sell
//
// Example:
//
// result, poolState, configState, currentPoint, _ := SellQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// instructions, _ := SellInstruction(
//
//	ctx,
//	m.rpcClient,
//	seller.PublicKey(), // payer account
//	referrer, // referral account, contact meteora
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	amountIn, // amount to spend on selling
//	result.MinimumAmountOut, // minimum amount to receive after accounting for slippage
//	currentPoint,
//
// )
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

// Sell sells base tokens to receive quote tokens on the Dynamic Bonding Curve.
// It depends on the SellInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// result, poolState, configState, currentPoint, _ := SellQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// sig, _ := meteoraDBC.Sell(
//
//	ctx,
//	wsClient,
//	ownerWallet, // seller
//	nil, // referral account, contact meteora
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	amountIn, // amount to spend on selling
//	result.MinimumAmountOut, // minimum amount to receive after accounting for slippage
//	currentPoint,
//
// )
func (m *DBC) Sell(
	ctx context.Context,
	wsClient *ws.Client,
	seller *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
	currentPoint *big.Int,
) (string, error) {
	lamportsMINT, err := solanago.MintBalance(ctx, m.rpcClient, seller.PublicKey(), poolState.BaseMint)
	if err != nil {
		return "", err
	}

	if amountIn.Cmp(new(big.Int).SetUint64(lamportsMINT)) > 0 {
		return "", fmt.Errorf("insufficient token balance")
	}

	// rentExemptFee, err := solanago.GetRentExempt(ctx, m.rpcClient)
	// if err != nil {
	// 	return "", err
	// }

	lamportsSOL, err := solanago.SOLBalance(ctx, m.rpcClient, seller.PublicKey())
	if err != nil {
		return "", err
	}

	if lamportsSOL < rentExemptFee+transferFee {
		return "", fmt.Errorf("seller sol must be greater than %v", (rentExemptFee+transferFee)/1e9)
	}

	return m.Swap(
		ctx,
		wsClient,
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
