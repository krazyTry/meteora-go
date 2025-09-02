package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/solana/token2022"
)

// GetQuoteResult
type GetQuoteResult struct {
	SwapInAmount     *big.Int
	ConsumedInAmount *big.Int
	SwapOutAmount    *big.Int
	MinSwapOutAmount *big.Int
	TotalFee         *big.Int
	PriceImpact      *big.Float
}

// GetQuote
func (m *DammV2) SwapQuote(ctx context.Context,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {

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

		var curEpoch uint64
		if curEpoch, err = solanago.GetCurrentEpoch(ctx, m.rpcClient); err != nil {
			return nil, nil, err
		}
		currentEpoch = &curEpoch

		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, m.rpcClient, baseMint)
		if err != nil {
			return nil, nil, err
		}

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

	currentPoint, err := solanago.CurrenPoint(ctx, m.rpcClient, uint8(virtualPool.Pool.ActivationType))
	if err != nil {
		return nil, nil, err
	}

	var dynamicFeeParams *cp_amm.DynamicFeeParams
	if virtualPool.PoolFees.DynamicFee.Initialized {
		dynamicFeeParams = &cp_amm.DynamicFeeParams{
			VolatilityAccumulator: virtualPool.PoolFees.DynamicFee.VolatilityAccumulator.BigInt(),
			BinStep:               big.NewInt(int64(virtualPool.PoolFees.DynamicFee.BinStep)),
			VariableFeeControl:    big.NewInt(int64(virtualPool.PoolFees.DynamicFee.VariableFeeControl)),
		}
	}

	tradeFeeNumerator := cp_amm.GetFeeNumerator(
		currentPoint,
		new(big.Int).SetUint64(virtualPool.ActivationPoint),
		int64(virtualPool.PoolFees.BaseFee.NumberOfPeriod),
		new(big.Int).SetUint64(virtualPool.PoolFees.BaseFee.PeriodFrequency),
		virtualPool.PoolFees.BaseFee.FeeSchedulerMode,
		new(big.Int).SetUint64(virtualPool.PoolFees.BaseFee.CliffFeeNumerator),
		new(big.Int).SetUint64(virtualPool.PoolFees.BaseFee.ReductionFactor),
		dynamicFeeParams,
	)

	amountOut, totalFee, _, err := cp_amm.GetSwapAmount(
		actualAmountIn,
		virtualPool.SqrtPrice.BigInt(),
		virtualPool.Liquidity.BigInt(),
		tradeFeeNumerator,
		swapBaseForQuote,
		virtualPool.CollectFeeMode,
	)
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

		actualAmountOut, _, err = cp_amm.CalculateTransferFeeExcludedAmount(
			transferFeeConfig,
			amountOut,
			quoteMint,
			*currentEpoch,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	minSwapOutAmount := cp_amm.GetMinAmountWithSlippage(actualAmountOut, slippageBps)

	priceImpact, err := cp_amm.GetPriceImpact(
		actualAmountIn,
		actualAmountOut,
		virtualPool.SqrtPrice.BigInt(),
		swapBaseForQuote,
		tokens[0].Decimals,
		tokens[1].Decimals,
	)
	if err != nil {
		return nil, nil, err
	}

	return &GetQuoteResult{
		SwapInAmount:     amountIn,
		ConsumedInAmount: actualAmountIn,
		SwapOutAmount:    actualAmountOut,
		MinSwapOutAmount: minSwapOutAmount,
		TotalFee:         totalFee,
		PriceImpact:      priceImpact,
	}, virtualPool, nil
}

func (m *DammV2) BuyQuote(ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return m.SwapQuote(ctx, baseMint, false, amountIn, slippageBps)
}

func (m *DammV2) SellQuote(ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return m.SwapQuote(ctx, baseMint, true, amountIn, slippageBps)
}
