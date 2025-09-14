package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/solana/token2022"
	"github.com/shopspring/decimal"
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

func (m *DammV2) SwapQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return SwapQuote(ctx, m.rpcClient, baseMint, swapBaseForQuote, amountIn, slippageBps)
}

// GetQuote
func SwapQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	swapBaseForQuote bool, // buy(quote=>base) sell(base => quote)
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {

	poolStates, err := GetPoolByBaseMint(ctx, rpcClient, baseMint)
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

		var curEpoch uint64
		if curEpoch, err = solanago.GetCurrentEpoch(ctx, rpcClient); err != nil {
			return nil, nil, err
		}
		currentEpoch = &curEpoch

		var transferFeeConfig *token2022.TransferFeeConfig
		transferFeeConfig, err = token2022.GetTransferFeeConfig(ctx, rpcClient, baseMint)
		if err != nil {
			return nil, nil, err
		}

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

	currentPoint, err := solanago.CurrenPoint(ctx, rpcClient, uint8(poolState.Pool.ActivationType))
	if err != nil {
		return nil, nil, err
	}

	var dynamicFeeParams *cp_amm.DynamicFeeParams
	if poolState.PoolFees.DynamicFee.Initialized {
		dynamicFeeParams = &cp_amm.DynamicFeeParams{
			VolatilityAccumulator: poolState.PoolFees.DynamicFee.VolatilityAccumulator.BigInt(),
			BinStep:               new(big.Int).SetUint64(uint64(poolState.PoolFees.DynamicFee.BinStep)),
			VariableFeeControl:    new(big.Int).SetUint64(uint64(poolState.PoolFees.DynamicFee.VariableFeeControl)),
		}
	}

	// currentPoint.Set(big.NewInt(1756971292))
	tradeFeeNumerator := cp_amm.GetFeeNumerator(
		decimal.NewFromBigInt(currentPoint, 0),
		decimal.NewFromUint64(poolState.ActivationPoint),
		decimal.NewFromUint64(uint64(poolState.PoolFees.BaseFee.NumberOfPeriod)),
		decimal.NewFromUint64(poolState.PoolFees.BaseFee.PeriodFrequency),
		poolState.PoolFees.BaseFee.FeeSchedulerMode,
		decimal.NewFromUint64(poolState.PoolFees.BaseFee.CliffFeeNumerator),
		decimal.NewFromUint64(poolState.PoolFees.BaseFee.ReductionFactor),
		dynamicFeeParams,
	)

	amountOut, totalFee, _, err := cp_amm.GetSwapAmount(
		actualAmountIn,
		decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
		decimal.NewFromBigInt(poolState.Liquidity.BigInt(), 0),
		tradeFeeNumerator,
		swapBaseForQuote,
		poolState.CollectFeeMode,
	)
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
		decimal.NewFromBigInt(poolState.SqrtPrice.BigInt(), 0),
		swapBaseForQuote,
		tokens[0].Decimals,
		tokens[1].Decimals,
	)
	if err != nil {
		return nil, nil, err
	}

	return &GetQuoteResult{
		SwapInAmount:     amountIn,
		ConsumedInAmount: actualAmountIn.BigInt(),
		SwapOutAmount:    actualAmountOut.BigInt(),
		MinSwapOutAmount: minSwapOutAmount.BigInt(),
		TotalFee:         totalFee.BigInt(),
		PriceImpact:      priceImpact.BigFloat(),
	}, poolState, nil
}

func (m *DammV2) BuyQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return m.SwapQuote(ctx, baseMint, false, amountIn, slippageBps)
}

func (m *DammV2) SellQuote(
	ctx context.Context,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return m.SwapQuote(ctx, baseMint, true, amountIn, slippageBps)
}

func BuyQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return SwapQuote(ctx, rpcClient, baseMint, false, amountIn, slippageBps)
}

func SellQuote(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64,
) (*GetQuoteResult, *Pool, error) {
	return SwapQuote(ctx, rpcClient, baseMint, true, amountIn, slippageBps)
}
