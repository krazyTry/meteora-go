package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/solana/token2022"

	"github.com/gagliardetto/solana-go"
)

// DepositQuote
type DepositQuote struct {
	ActualInputAmount   *big.Int // Actual input amount (after deducting fees)
	ConsumedInputAmount *big.Int // Original input amount
	LiquidityDelta      *big.Int // Liquidity to be added to the pool
	OutputAmount        *big.Int // Calculated amount of the other token
	MinOutAmount        *big.Int
}

// GetDepositQuote Calculate the deposit quote for the liquidity pool
func (m *DammV2) GetDepositQuote(ctx context.Context,
	baseMint solana.PublicKey,
	bAddBase bool,
	amountIn *big.Int,
	slippageBps uint64,
) (*DepositQuote, *Pool, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return nil, nil, err
	}

	baseMint = virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	tokens, err := solanago.GetMultipleToken(ctx, m.rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, nil, err
	}

	if tokens[0] == nil || tokens[1] == nil {
		return nil, nil, fmt.Errorf("baseMint or quoteMint error")
	}

	actualAmountIn := amountIn

	var currentEpoch *uint64
	if tokens[0].Owner.Equals(solana.Token2022ProgramID) {
		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, m.rpcClient, baseMint)
		if err != nil {
			return nil, nil, err
		}

		var curEpoch uint64
		if curEpoch, err = solanago.GetCurrentEpoch(ctx, m.rpcClient); err != nil {
			return nil, nil, err
		}
		currentEpoch = &curEpoch

		actualAmountIn, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			amountIn,
			baseMint,
			*currentEpoch,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	liquidityDelta, amountOut, err := cp_amm.GetDepositQuote(virtualPool.Pool, actualAmountIn, bAddBase)
	if err != nil {
		return nil, nil, err
	}

	actualAmountOut := new(big.Int).Set(amountOut)

	if tokens[1].Owner.Equals(solana.Token2022ProgramID) {
		if currentEpoch == nil {
			var curEpoch uint64
			if curEpoch, err = solanago.GetCurrentEpoch(ctx, m.rpcClient); err != nil {
				return nil, nil, err
			}
			currentEpoch = &curEpoch
		}

		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, m.rpcClient, quoteMint)
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
	minOutAmount := cp_amm.GetMinAmountWithSlippage(actualAmountOut, slippageBps)

	return &DepositQuote{
		ActualInputAmount:   actualAmountIn,
		ConsumedInputAmount: amountIn,
		LiquidityDelta:      liquidityDelta,
		OutputAmount:        actualAmountOut,
		MinOutAmount:        minOutAmount,
	}, virtualPool, nil
}

// WithdrawQuote
type WithdrawQuote struct {
	LiquidityDelta *big.Int
	OutAmountA     *big.Int
	OutAmountB     *big.Int
}

// getWithdrawQuote
func (m *DammV2) GetWithdrawQuote(ctx context.Context,
	baseMint solana.PublicKey,
	liquidityDelta *big.Int,
) (*big.Int, *big.Int, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return nil, nil, err
	}

	amountA := cp_amm.GetAmountAFromLiquidityDelta(liquidityDelta, virtualPool.SqrtPrice.BigInt(), virtualPool.SqrtMaxPrice.BigInt(), false)
	amountB := cp_amm.GetAmountBFromLiquidityDelta(liquidityDelta, virtualPool.SqrtPrice.BigInt(), virtualPool.SqrtMinPrice.BigInt(), false)

	baseMint = virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	tokens, err := solanago.GetMultipleToken(ctx, m.rpcClient, baseMint, quoteMint)
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
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, m.rpcClient, baseMint)
		if err != nil {
			return nil, nil, err
		}

		var curEpoch uint64
		if curEpoch, err = solanago.GetCurrentEpoch(ctx, m.rpcClient); err != nil {
			return nil, nil, err
		}
		currentEpoch = &curEpoch

		outAmountA, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			amountA,
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
			if curEpoch, err = solanago.GetCurrentEpoch(ctx, m.rpcClient); err != nil {
				return nil, nil, err
			}
			currentEpoch = &curEpoch
		}

		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, m.rpcClient, quoteMint)
		if err != nil {
			return nil, nil, err
		}

		outAmountB, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			amountA,
			quoteMint,
			*currentEpoch,
		)

		if err != nil {
			return nil, nil, err
		}
	}

	return outAmountA, outAmountB, nil
}
