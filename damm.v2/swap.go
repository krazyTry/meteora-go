package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// SwapInstruction generates the instruction needed to swap
//
// Example:
//
// result, poolState, _ := SwapQuote( ctx, rpcClient, baseMint, false, amountIn, slippageBps)
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
//	poolState.Address, // damm v2 pool address
//	poolState.Pool, // damm v2 pool state
//	swapBaseForQuote, // buy(quote=>base) sell(base => quote)
//	amountIn, // amount to spend on selling or buying
//	result.MinSwapOutAmount,  // minimum amount to receive after accounting for slippage
//
// )
func SwapInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	owner solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) ([]solana.Instruction, error) {
	if amountIn.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("amountIn must be greater than 0")
	}

	var instructions []solana.Instruction

	inputMint, outputMint, inputMintProgram, outputMintProgram := cp_amm.PrepareSwapParams(swapBaseForQuote, poolState)

	inputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, inputMint, owner, &instructions)
	if err != nil {
		return nil, err
	}

	outputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, outputMint, owner, &instructions)
	if err != nil {
		return nil, err
	}

	baseMint := poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	referralTokenAccount := solana.PublicKey{}

	if !referrer.Equals(solana.PublicKey{}) {
		switch poolState.CollectFeeMode {
		case cp_amm.CollectFeeModeOnlyA:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, rpcClient, referrer, baseMint, owner, &instructions)
			if err != nil {
				return nil, err
			}
		case cp_amm.CollectFeeModeOnlyB:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, rpcClient, referrer, quoteMint, owner, &instructions)
			if err != nil {
				return nil, err
			}
		}
	}

	switch {
	case inputMint.Equals(solana.WrappedSol):
		// wrap SOL by transferring lamports into the WSOL ATA
		wrapSOLIx := system.NewTransferInstruction(
			amountIn.Uint64(),
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

	baseVault := poolState.TokenAVault
	quoteVault := poolState.TokenBVault

	swapIx, err := cp_amm.NewSwapInstruction(
		// Params:
		cp_amm.SwapParameters{
			AmountIn:         amountIn.Uint64(),
			MinimumAmountOut: minimumAmountOut.Uint64(),
		},

		// Accounts:
		poolAuthority,
		poolAddress,
		inputTokenAccount,
		outputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		owner,
		inputMintProgram,
		outputMintProgram,
		referralTokenAccount,
		eventAuthority,
		cp_amm.ProgramID,
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

	if !referrer.Equals(solana.PublicKey{}) && poolState.CollectFeeMode == cp_amm.CollectFeeModeOnlyB {
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

// Swap swaps between base and quote or quote and base on the Damm v2
// It depends on the SwapInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// result, poolState, _ := SwapQuote( ctx, rpcClient, baseMint, false, amountIn, slippageBps)
//
// instructions, _ := meteoraDammV2.Swap(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	owner, // owner account
//	nil, // referral account, contact meteora
//	poolState.Address, // damm v2 pool address
//	poolState.Pool, // damm v2 pool state
//	swapBaseForQuote, // buy(quote=>base) sell(base => quote)
//	amountIn, // amount to spend on selling or buying
//	result.MinSwapOutAmount,  // minimum amount to receive after accounting for slippage
//
// )
func (m *DammV2) Swap(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	owner *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) (string, error) {

	instructions, err := SwapInstruction(
		ctx,
		m.rpcClient,
		owner.PublicKey(),
		func() solana.PublicKey {
			if referrer == nil {
				return solana.PublicKey{}
			}
			return referrer.PublicKey()
		}(),
		poolAddress,
		poolState,
		swapBaseForQuote,
		amountIn,
		minimumAmountOut,
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

// BuyInstruction generates the instruction needed to buy
//
// Example:
//
// result, poolState, _ := BuyQuote(ctx, rpcClient, baseMint, amountIn, slippageBps)
//
// instructions, _ := BuyInstruction(
//
//	ctx,
//	m.rpcClient,
//	buyer.PublicKey(), // payer account
//	referrer, // referral account, contact meteora
//	poolState.Address, // damm v2 pool address
//	poolState.Pool, // damm v2 pool state
//	amountIn, // amount to spend on buying
//	result.MinSwapOutAmount, // minimum amount to receive after accounting for slippage
//
// )
func BuyInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	buyer solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) ([]solana.Instruction, error) {
	return SwapInstruction(ctx, rpcClient, buyer, referrer, poolAddress, poolState, false, amountIn, minimumAmountOut)
}

// Buy buys base tokens using quote tokens on the Dynamic Bonding Curve.
// It depends on the BuyInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// result, poolState, _ := BuyQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// sig, _ := meteoraDammV2.Buy(
//
//	ctx,
//	wsClient,
//	ownerWallet, // buyer
//	nil, // referral account, contact meteora
//	poolState.Address, // damm v2 pool address
//	poolState.VirtualPool, // damm v2 pool state
//	amountIn, // amount to spend on buying
//	result.MinSwapOutAmount, // minimum amount to receive after accounting for slippage
//
// )
func (m *DammV2) Buy(
	ctx context.Context,
	wsClient *ws.Client,
	buyer *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) (string, error) {
	return m.Swap(
		ctx,
		wsClient,
		buyer,
		buyer,
		referrer,
		poolAddress,
		poolState,
		false,
		new(big.Int).Sub(amountIn, new(big.Int).SetUint64(rentExemptFee+transferFee)),
		minimumAmountOut,
	)
}

// SellInstruction generates the instruction needed to sell
//
// Example:
//
// result, poolState, _ := SellQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// instructions, _ := SellInstruction(
//
//	ctx,
//	m.rpcClient,
//	seller.PublicKey(), // payer account
//	referrer, // referral account, contact meteora
//	poolState.Address, // damm v2 pool address
//	poolState.Pool, // damm v2 pool state
//	amountIn, // amount to spend on selling
//	result.MinSwapOutAmount, // minimum amount to receive after accounting for slippage
//
// )
func SellInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	seller solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) ([]solana.Instruction, error) {
	return SwapInstruction(ctx, rpcClient, seller, referrer, poolAddress, poolState, true, amountIn, minimumAmountOut)
}

// Sell sells base tokens to receive quote tokens on the Dynamic Bonding Curve.
// It depends on the SellInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// result, poolState, _ := SellQuote(ctx, rpcClient, baseMint, amountIn, slippageBps, false)
//
// sig, _ := meteoraDammV2.Sell(
//
//	ctx,
//	wsClient,
//	ownerWallet, // seller
//	nil, // referral account, contact meteora
//	poolState.Address, // damm v2 pool address
//	poolState.Pool, // damm v2 pool state
//	amountIn, // amount to spend on selling
//	result.MinSwapOutAmount, // minimum amount to receive after accounting for slippage
//
// )
func (m *DammV2) Sell(
	ctx context.Context,
	wsClient *ws.Client,
	seller *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) (string, error) {
	return m.Swap(
		ctx,
		wsClient,
		seller,
		seller,
		referrer,
		poolAddress,
		poolState,
		true,
		amountIn,
		minimumAmountOut,
	)
}
