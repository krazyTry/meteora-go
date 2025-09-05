package dynamic_bonding_curve

import (
	"math/big"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type QuoteResult struct {
	AmountOut        *big.Int
	MinimumAmountOut *big.Int
	NextSqrtPrice    binary.Uint128
	Fee              FeeBreakdown
	Price            PriceInfo
}

type FeeBreakdown struct {
	Trading  *big.Int
	Protocol *big.Int
	Referral *big.Int
}

type PriceInfo struct {
	BeforeSwap binary.Uint128
	AfterSwap  binary.Uint128
}

func getSwapResult(
	poolState *VirtualPool,
	configState *PoolConfig,
	amountIn decimal.Decimal,
	feeMode *FeeMode,
	tradeDirection TradeDirection,
	currentPoint decimal.Decimal,
) (*QuoteResult, error) {

	actualProtocolFee := big.NewInt(0)
	actualTradingFee := big.NewInt(0)
	actualReferralFee := big.NewInt(0)

	var actualAmountIn decimal.Decimal

	// apply fees on input
	if feeMode.FeesOnInput {

		feeResultAmount, feeResultTradingFee, feeResultProtocolFee, feeResultReferralFee, err := getFeeOnAmount(
			amountIn,
			configState.PoolFees,
			feeMode.HasReferral,
			currentPoint,
			decimal.NewFromBigInt(new(big.Int).SetUint64(poolState.ActivationPoint), 0),
			poolState.VolatilityTracker,
			tradeDirection,
		)

		if err != nil {
			return nil, err
		}

		actualProtocolFee.Set(feeResultProtocolFee.BigInt())
		actualTradingFee.Set(feeResultTradingFee.BigInt())
		actualReferralFee.Set(feeResultReferralFee.BigInt())
		actualAmountIn = feeResultAmount
	} else {
		actualAmountIn = amountIn
	}

	// calculate swap amount
	var (
		outputAmount  decimal.Decimal
		nextSqrtPrice binary.Uint128
		err           error
	)

	if tradeDirection == TradeDirectionBaseToQuote {
		outputAmount, nextSqrtPrice, err = getSwapAmountFromBaseToQuote(configState.Curve[:], poolState.SqrtPrice, actualAmountIn)
	} else {
		outputAmount, nextSqrtPrice, err = getSwapAmountFromQuoteToBase(configState.Curve[:], poolState.SqrtPrice, actualAmountIn)
	}
	if err != nil {
		return nil, err
	}

	// apply fees on output if needed
	var actualAmountOut decimal.Decimal
	if feeMode.FeesOnInput {
		actualAmountOut = outputAmount
	} else {
		feeResultAmount, feeResultTradingFee, feeResultProtocolFee, feeResultReferralFee, err := getFeeOnAmount(
			outputAmount,
			configState.PoolFees,
			feeMode.HasReferral,
			currentPoint,
			decimal.NewFromBigInt(new(big.Int).SetUint64(poolState.ActivationPoint), 0),
			poolState.VolatilityTracker,
			tradeDirection,
		)
		if err != nil {
			return nil, err
		}

		actualProtocolFee.Set(feeResultProtocolFee.BigInt())
		actualTradingFee.Set(feeResultTradingFee.BigInt())
		actualReferralFee.Set(feeResultReferralFee.BigInt())
		actualAmountOut = feeResultAmount
	}

	return &QuoteResult{
		AmountOut:        actualAmountOut.BigInt(),
		MinimumAmountOut: actualAmountOut.BigInt(),
		NextSqrtPrice:    nextSqrtPrice,
		Fee: FeeBreakdown{
			Trading:  actualTradingFee,
			Protocol: actualProtocolFee,
			Referral: actualReferralFee,
		},
		Price: PriceInfo{
			BeforeSwap: poolState.SqrtPrice,
			AfterSwap:  nextSqrtPrice,
		},
	}, nil
}

func PrepareSwapParams(
	swapBaseForQuote bool,
	poolState *VirtualPool,
	poolConfig *PoolConfig,
) (solana.PublicKey, solana.PublicKey, solana.PublicKey, solana.PublicKey) {
	if swapBaseForQuote {
		return poolState.BaseMint, poolConfig.QuoteMint, GetTokenProgram(poolState.PoolType), GetTokenProgram(poolConfig.QuoteTokenFlag)
	} else {
		return poolConfig.QuoteMint, poolState.BaseMint, GetTokenProgram(poolConfig.QuoteTokenFlag), GetTokenProgram(poolState.PoolType)
	}
}
