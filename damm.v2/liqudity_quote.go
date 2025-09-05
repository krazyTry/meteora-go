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

func (m *DammV2) GetDepositQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	bAddBase bool,
	amountIn *big.Int,
) (*DepositQuote, *Pool, error) {
	return GetDepositQuote(ctx, m.rpcClient, baseMint, bAddBase, amountIn)
}

// GetDepositQuote Calculate the deposit quote for the liquidity pool

func GetDepositQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	bAddBase bool,
	amountIn *big.Int,
) (*DepositQuote, *Pool, error) {

	poolState, err := GetPoolByBaseMint(ctx, rpcClient, baseMint)
	if err != nil {
		return nil, nil, err
	}

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

func (m *DammV2) GetWithdrawQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	liquidityDelta *big.Int,
) (*WithdrawQuote, *Pool, error) {
	return GetWithdrawQuote(ctx, m.rpcClient, baseMint, liquidityDelta)
}

// getWithdrawQuote
func GetWithdrawQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	liquidityDelta *big.Int,
) (*WithdrawQuote, *Pool, error) {
	poolState, err := GetPoolByBaseMint(ctx, rpcClient, baseMint)
	if err != nil {
		return nil, nil, err
	}

	amountA := cp_amm.GetAmountAFromLiquidityDelta(decimal.NewFromBigInt(liquidityDelta, 0), decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0), decimal.NewFromBigInt(poolState.SqrtMaxPrice.BigInt(), 0), false)
	amountB := cp_amm.GetAmountBFromLiquidityDelta(decimal.NewFromBigInt(liquidityDelta, 0), decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0), decimal.NewFromBigInt(poolState.SqrtMinPrice.BigInt(), 0), false)

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
