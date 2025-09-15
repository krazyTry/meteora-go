package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/solana/token2022"
	"github.com/shopspring/decimal"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// DepositQuote
type DepositQuote struct {
	ActualInputAmount   *big.Int // Actual input amount (after deducting fees)
	ConsumedInputAmount *big.Int // Original input amount
	LiquidityDelta      *big.Int // Liquidity to be added to the pool
	OutputAmount        *big.Int // Calculated amount of the other token
}

// GetDepositQuote
// It depends on the GetDepositQuote function.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// amountIn := new(big.Int).SetUint64(10_000_000)
//
// quote, virtualPool, _ := meteoraDammV2.GetDepositQuote(
//
//	ctx,
//	baseMint,
//	true, // true baseMintToken or false quoteMintToken
//	amountIn, // planned amount to add
//
// )
func (m *DammV2) GetDepositQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	bAddBase bool, // true baseMintToken or false quoteMintToken
	amountIn *big.Int,
) (*DepositQuote, *Pool, error) {
	return GetDepositQuote(ctx, m.rpcClient, baseMint, bAddBase, amountIn)
}

// GetDepositQuote Calculates the deposit quote for adding liquidity to a pool based on a single token input.
// This function is an example function. It only reads the 0th position of poolStates. For multi-pool scenarios, you need to implement it yourself.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// amountIn := new(big.Int).SetUint64(10_000_000)
//
// quote, virtualPool, _ := GetDepositQuote(
//
//	ctx,
//	rpcClient,
//	baseMint,
//	true, // true baseMintToken or false quoteMintToken
//	amountIn, // planned amount to add
//
// )
func GetDepositQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	bAddBase bool,
	amountIn *big.Int,
) (*DepositQuote, *Pool, error) {

	poolStates, err := GetPoolsByBaseMint(ctx, rpcClient, baseMint)
	if err != nil {
		return nil, nil, err
	}
	poolState := poolStates[0]

	baseMint = poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	tokens, err := solanago.GetMultipleToken(ctx, rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, nil, err
	}

	if tokens[0] == nil || tokens[1] == nil {
		return nil, nil, fmt.Errorf("baseMint or quoteMint error")
	}

	actualAmountIn := decimal.NewFromBigInt(amountIn, 0)

	var currentEpoch *uint64
	if tokens[0].Owner.Equals(solana.Token2022ProgramID) {
		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, rpcClient, baseMint)
		if err != nil {
			return nil, nil, err
		}

		var curEpoch uint64
		if curEpoch, err = solanago.GetCurrentEpoch(ctx, rpcClient); err != nil {
			return nil, nil, err
		}
		currentEpoch = &curEpoch

		actualAmountIn, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			actualAmountIn,
			baseMint,
			*currentEpoch,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	liquidityDelta, amountOut, err := cp_amm.GetDepositQuote(poolState.Pool, actualAmountIn, bAddBase)
	if err != nil {
		return nil, nil, err
	}

	actualAmountOut := amountOut

	if tokens[1].Owner.Equals(solana.Token2022ProgramID) {
		if currentEpoch == nil {
			var curEpoch uint64
			if curEpoch, err = solanago.GetCurrentEpoch(ctx, rpcClient); err != nil {
				return nil, nil, err
			}
			currentEpoch = &curEpoch
		}

		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, rpcClient, quoteMint)
		if err != nil {
			return nil, nil, err
		}

		actualAmountOut, _, err = cp_amm.CalculateTransferFeeIncludedAmount(
			transferFeeConfig,
			actualAmountOut,
			quoteMint,
			*currentEpoch,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	return &DepositQuote{
		ActualInputAmount:   actualAmountIn.BigInt(),
		ConsumedInputAmount: amountIn,
		LiquidityDelta:      liquidityDelta.BigInt(),
		OutputAmount:        actualAmountOut.BigInt(),
	}, poolState, nil
}

// WithdrawQuote
type WithdrawQuote struct {
	OutBaseAmount  *big.Int
	OutQuoteAmount *big.Int
}

// GetWithdrawQuote
// It depends on the GetWithdrawQuote function.
//
// Example:
//
// liquidityDelta, position, _ := meteoraDammV2.GetPositionLiquidity(ctx, baseMint, poolPartner.PublicKey())
//
// liquidityDelta = new(big.Int).Div(liquidityDelta, big.NewInt(2))
//
// quote, virtualPool, _ := meteoraDammV2.GetWithdrawQuote(
//
//	ctx,
//	baseMint,
//	liquidityDelta, // liquidity to be removed
//
// )
func (m *DammV2) GetWithdrawQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	liquidityDelta *big.Int,
) (*WithdrawQuote, *Pool, error) {
	return GetWithdrawQuote(ctx, m.rpcClient, baseMint, liquidityDelta)
}

// GetWithdrawQuote Calculates the withdrawal quote for removing liquidity from a pool.
// This function is an example function. It only reads the 0th element of poolState. For scenarios with multiple pools, you need to implement it yourself.
//
// Example:
//
// liquidityDelta, position, _ := meteoraDammV2.GetPositionLiquidity(ctx, baseMint, poolPartner.PublicKey())
//
// liquidityDelta = new(big.Int).Div(liquidityDelta, big.NewInt(2))
//
// quote, virtualPool, _ := meteoraDammV2.GetWithdrawQuote(
//
//	ctx,
//	baseMint,
//	liquidityDelta, // liquidity to be removed
//
// )
func GetWithdrawQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	liquidityDelta *big.Int,
) (*WithdrawQuote, *Pool, error) {
	poolStates, err := GetPoolsByBaseMint(ctx, rpcClient, baseMint)
	if err != nil {
		return nil, nil, err
	}
	poolState := poolStates[0]

	amountA := cp_amm.GetAmountAFromLiquidityDelta(
		decimal.NewFromBigInt(liquidityDelta, 0),
		decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
		decimal.NewFromBigInt(poolState.SqrtMaxPrice.BigInt(), 0),
		false,
	)
	amountB := cp_amm.GetAmountBFromLiquidityDelta(
		decimal.NewFromBigInt(liquidityDelta, 0),
		decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
		decimal.NewFromBigInt(poolState.SqrtMinPrice.BigInt(), 0),
		false,
	)

	baseMint = poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	tokens, err := solanago.GetMultipleToken(ctx, rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, nil, err
	}

	if tokens[0] == nil || tokens[1] == nil {
		return nil, nil, fmt.Errorf("baseMint or quoteMint error")
	}

	outAmountA := amountA

	var currentEpoch *uint64
	if tokens[0].Owner.Equals(solana.Token2022ProgramID) {
		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, rpcClient, baseMint)
		if err != nil {
			return nil, nil, err
		}

		var curEpoch uint64
		if curEpoch, err = solanago.GetCurrentEpoch(ctx, rpcClient); err != nil {
			return nil, nil, err
		}
		currentEpoch = &curEpoch

		outAmountA, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			outAmountA,
			baseMint,
			*currentEpoch,
		)

		if err != nil {
			return nil, nil, err
		}
	}

	outAmountB := amountB
	if tokens[1].Owner.Equals(solana.Token2022ProgramID) {
		if currentEpoch == nil {
			var curEpoch uint64
			if curEpoch, err = solanago.GetCurrentEpoch(ctx, rpcClient); err != nil {
				return nil, nil, err
			}
			currentEpoch = &curEpoch
		}

		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, rpcClient, quoteMint)
		if err != nil {
			return nil, nil, err
		}

		outAmountB, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			outAmountB,
			quoteMint,
			*currentEpoch,
		)

		if err != nil {
			return nil, nil, err
		}
	}

	return &WithdrawQuote{
		OutBaseAmount:  outAmountA.BigInt(),
		OutQuoteAmount: outAmountB.BigInt(),
	}, poolState, nil
}
