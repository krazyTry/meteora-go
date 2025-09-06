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
	Trading          *big.Int
	Protocol         *big.Int
	Referral         *big.Int
}

func getSwapResult(
	poolState *VirtualPool,
	configState *PoolConfig,
	amountIn decimal.Decimal,
	feeMode *FeeMode,
	tradeDirection TradeDirection,
	currentPoint decimal.Decimal,
) (*QuoteResult, error) {

	var (
		actualProtocolFee decimal.Decimal
		actualTradingFee  decimal.Decimal
		actualReferralFee decimal.Decimal
	)

	tradeFeeNumerator, err := getTotalFeeNumeratorFromIncludedFeeAmount(
		configState.PoolFees,
		poolState.VolatilityTracker,
		currentPoint,
		decimal.NewFromUint64(poolState.ActivationPoint),
		amountIn,
		tradeDirection,
	)

	if err != nil {
		return nil, err
	}

	var actualAmountIn decimal.Decimal

	// apply fees on input
	if feeMode.FeesOnInput {

		// feeResultAmount, feeResultTradingFee, feeResultProtocolFee, feeResultReferralFee, err := getFeeOnAmount1(
		// 	amountIn,
		// 	configState.PoolFees,
		// 	feeMode.HasReferral,
		// 	currentPoint,
		// 	decimal.NewFromBigInt(new(big.Int).SetUint64(poolState.ActivationPoint), 0),
		// 	poolState.VolatilityTracker,
		// 	tradeDirection,
		// )

		amountAfterFee, updatedProtocolFee, referralFee, updatedTradingFee, err := getFeeOnAmount(
			tradeFeeNumerator,
			amountIn,
			configState.PoolFees,
			feeMode.HasReferral,
		)

		if err != nil {
			return nil, err
		}

		actualProtocolFee = updatedProtocolFee
		actualTradingFee = updatedTradingFee
		actualReferralFee = referralFee
		actualAmountIn = amountAfterFee
	} else {
		actualAmountIn = amountIn
	}

	// calculate swap amount
	var (
		outputAmount  decimal.Decimal
		nextSqrtPrice binary.Uint128
	)

	if tradeDirection == TradeDirectionBaseToQuote {
		outputAmount, nextSqrtPrice, _, err = getSwapAmountFromBaseToQuote(configState.Curve[:], poolState.SqrtPrice, actualAmountIn)
	} else {
		outputAmount, nextSqrtPrice, _, err = getSwapAmountFromQuoteToBase(configState.Curve[:], poolState.SqrtPrice, actualAmountIn, U128_MAX)
	}
	if err != nil {
		return nil, err
	}

	// apply fees on output if needed
	var actualAmountOut decimal.Decimal
	if feeMode.FeesOnInput {
		actualAmountOut = outputAmount
	} else {

		// feeResultAmount, feeResultTradingFee, feeResultProtocolFee, feeResultReferralFee, err := getFeeOnAmount(
		// 	outputAmount,
		// 	configState.PoolFees,
		// 	feeMode.HasReferral,
		// 	currentPoint,
		// 	decimal.NewFromUint64(poolState.ActivationPoint),
		// 	poolState.VolatilityTracker,
		// 	tradeDirection,
		// )
		amountAfterFee, updatedProtocolFee, referralFee, updatedTradingFee, err := getFeeOnAmount(
			tradeFeeNumerator,
			outputAmount,
			configState.PoolFees,
			feeMode.HasReferral,
		)

		if err != nil {
			return nil, err
		}

		actualProtocolFee = updatedProtocolFee
		actualTradingFee = updatedTradingFee
		actualReferralFee = referralFee
		actualAmountOut = amountAfterFee
	}

	return &QuoteResult{
		AmountOut:        actualAmountOut.BigInt(),
		MinimumAmountOut: actualAmountOut.BigInt(),
		NextSqrtPrice:    nextSqrtPrice,
		Trading:          actualTradingFee.BigInt(),
		Protocol:         actualProtocolFee.BigInt(),
		Referral:         actualReferralFee.BigInt(),
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
