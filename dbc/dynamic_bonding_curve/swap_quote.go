package dynamic_bonding_curve

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/u128"

	binary "github.com/gagliardetto/binary"
	"github.com/shopspring/decimal"
)

func getSwapAmountFromQuoteToBase(curve []LiquidityDistributionConfig, currentSqrtPrice binary.Uint128, amountIn, stopSqrtPrice decimal.Decimal) (decimal.Decimal, binary.Uint128, decimal.Decimal, error) {
	if amountIn.IsZero() {
		return N0, currentSqrtPrice, N0, nil
	}

	totalOutput := N0
	currentSqrtPriceLocal := decimal.NewFromBigInt(currentSqrtPrice.BigInt(), 0)
	amountLeft := amountIn

	// iterate through the curve points
	for _, point := range curve {
		pointSqrtPrice := decimal.NewFromBigInt(point.SqrtPrice.BigInt(), 0)
		pointLiquidity := decimal.NewFromBigInt(point.Liquidity.BigInt(), 0)

		// skip if liquidity is zero
		if pointSqrtPrice.IsZero() || pointLiquidity.IsZero() {
			continue
		}

		referenceSqrtPrice := decimal.Min(stopSqrtPrice, pointSqrtPrice)

		if referenceSqrtPrice.Cmp(currentSqrtPriceLocal) > 0 {
			maxAmountIn := getDeltaAmountQuoteUnsigned(
				currentSqrtPriceLocal,
				pointSqrtPrice,
				pointLiquidity,
				true,
			)

			if amountLeft.Cmp(maxAmountIn) < 0 {
				nextSqrtPrice, err := getNextSqrtPriceFromInput(
					currentSqrtPriceLocal,
					pointLiquidity,
					amountLeft,
					false,
				)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, decimal.Decimal{}, err
				}
				outputAmount, err := getDeltaAmountBaseUnsigned(
					currentSqrtPriceLocal,
					nextSqrtPrice,
					pointLiquidity,
					false,
				)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, decimal.Decimal{}, err
				}
				totalOutput = totalOutput.Add(outputAmount)
				currentSqrtPriceLocal = nextSqrtPrice
				amountLeft = N0
				break
			} else {
				nextSqrtPrice := referenceSqrtPrice

				outputAmount, err := getDeltaAmountBaseUnsigned(
					currentSqrtPriceLocal,
					nextSqrtPrice,
					pointLiquidity,
					false)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, decimal.Decimal{}, err
				}
				totalOutput = totalOutput.Add(outputAmount)
				currentSqrtPriceLocal = nextSqrtPrice
				amountLeft = amountLeft.Sub(maxAmountIn)

				if nextSqrtPrice.Equal(stopSqrtPrice) {
					break
				}
			}
		}
	}

	return totalOutput, u128.GenUint128FromString(currentSqrtPriceLocal.String()), amountLeft, nil
}

func getSwapAmountFromBaseToQuote(curve []LiquidityDistributionConfig, sqrtStartPrice binary.Uint128, amountIn decimal.Decimal) (decimal.Decimal, binary.Uint128, decimal.Decimal, error) {

	if amountIn.IsZero() {
		return N0, sqrtStartPrice, N0, nil
	}

	totalOutput := decimal.Zero
	currentSqrtPriceLocal := decimal.NewFromBigInt(sqrtStartPrice.BigInt(), 0)
	amountLeft := amountIn

	// for i := range curve {
	for i := len(curve) - 2; i >= 0; i-- {

		if curve[i].SqrtPrice.BigInt().Cmp(big.NewInt(0)) == 0 || curve[i].Liquidity.BigInt().Cmp(big.NewInt(0)) == 0 {
			continue
		}
		cp := curve[i]
		currentSqrtPrice := decimal.NewFromBigInt(cp.SqrtPrice.BigInt(), 0)
		cp1 := curve[i+1]
		currentLiquidity := decimal.NewFromBigInt(cp1.Liquidity.BigInt(), 0)

		if currentSqrtPrice.Cmp(currentSqrtPriceLocal) < 0 {
			maxAmountIn, err := getDeltaAmountBaseUnsigned(
				currentSqrtPrice,
				currentSqrtPriceLocal,
				currentLiquidity,
				true,
			)
			if err != nil {
				return decimal.Decimal{}, binary.Uint128{}, decimal.Decimal{}, err
			}

			if amountLeft.Cmp(maxAmountIn) < 0 {

				nextSqrt, err := getNextSqrtPriceFromInput(
					currentSqrtPriceLocal,
					currentLiquidity,
					amountLeft,
					true,
				)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, decimal.Decimal{}, err
				}

				outputAmount := getDeltaAmountQuoteUnsigned(
					nextSqrt,
					currentSqrtPriceLocal,
					currentLiquidity,
					true,
				)
				totalOutput = totalOutput.Add(outputAmount)
				currentSqrtPriceLocal = nextSqrt
				amountLeft = decimal.Zero
				break

			} else {

				nextSqrt := currentSqrtPrice
				outputAmount := getDeltaAmountQuoteUnsigned(
					nextSqrt,
					currentSqrtPriceLocal,
					currentLiquidity,
					true,
				)
				totalOutput = totalOutput.Add(outputAmount)
				currentSqrtPriceLocal = nextSqrt
				amountLeft = amountLeft.Sub(maxAmountIn)

			}
		}
	}

	zero := N0

	if amountLeft.Cmp(zero) > 0 {

		nextSqrt, err := getNextSqrtPriceFromInput(
			currentSqrtPriceLocal,
			decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0),
			amountLeft,
			true,
		)
		if err != nil {
			return decimal.Decimal{}, binary.Uint128{}, decimal.Decimal{}, err
		}

		outputAmount := getDeltaAmountQuoteUnsigned(
			nextSqrt,
			currentSqrtPriceLocal,
			decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0),
			false,
		)
		totalOutput = totalOutput.Add(outputAmount)
		currentSqrtPriceLocal = nextSqrt
	}

	return totalOutput, u128.GenUint128FromString(currentSqrtPriceLocal.String()), N0, nil
}

func GetSwapAmountFromQuote(configState *PoolConfig, amountIn decimal.Decimal, slippageBps uint64) (decimal.Decimal, error) {

	inAmount := amountIn

	tradeDirection := TradeDirectionQuoteToBase // buying base with quote

	hasReferral := false

	// get fee mode
	feeMode := getFeeMode(configState.CollectFeeMode, tradeDirection, hasReferral)

	// baseFeeNumerator = CliffFeeNumerator
	baseFeeNumerator := decimal.NewFromUint64(uint64(configState.PoolFees.BaseFee.CliffFeeNumerator))

	// tradeFeeNumerator = min(baseFeeNumerator, MAX_FEE_NUMERATOR)
	tradeFeeNumerator := decimal.Min(baseFeeNumerator, MAX_FEE_NUMERATOR)

	// apply fees on input if needed
	if feeMode.FeesOnInput {

		// tradingFee = amountIn * tradeFeeNumerator / FEE_DENOMINATOR
		tradingFee := inAmount.Mul(tradeFeeNumerator).Div(FEE_DENOMINATOR)

		// actualAmountIn = amountIn - tradingFee
		inAmount = inAmount.Sub(tradingFee)

	}

	// calculate swap amount
	amountOut, _, _, err := getSwapAmountFromQuoteToBase(configState.Curve[:], configState.SqrtStartPrice, inAmount, U128_MAX)
	if err != nil {
		return decimal.Decimal{}, err
	}

	if !feeMode.FeesOnInput {
		// tradingFee = outputAmount * tradeFeeNumerator / FEE_DENOMINATOR
		tradingFee := amountOut.Mul(tradeFeeNumerator).Div(FEE_DENOMINATOR)

		// actualAmountOut = outputAmount - tradingFee
		amountOut = amountOut.Sub(tradingFee)

	}

	if slippageBps > 0 {

		// slippageFactor = 10000 - slippageBps
		slippageFactor := N10000.Sub(decimal.NewFromUint64(uint64(slippageBps)))
		// denominator = 10000
		denominator := N10000
		// minAmountOut = amountOut * slippageFactor / denominator
		minAmountOut := amountOut.Mul(slippageFactor).Div(denominator)

		amountOut = minAmountOut

	}

	return amountOut, nil
}

func SwapQuote(
	poolState *VirtualPool,
	config *PoolConfig,
	swapBaseForQuote bool,
	amountIn decimal.Decimal,
	slippageBps decimal.Decimal,
	hasReferral bool,
	currentPoint decimal.Decimal,
) (*QuoteResult, error) {

	if amountIn.IsZero() {
		return nil, errors.New("amount is zero")
	}

	var tradeDirection TradeDirection
	if swapBaseForQuote {
		tradeDirection = TradeDirectionBaseToQuote
	} else {
		tradeDirection = TradeDirectionQuoteToBase
	}

	feeMode := getFeeMode(config.CollectFeeMode, tradeDirection, hasReferral)

	result, err := getSwapResult(poolState, config, amountIn, feeMode, tradeDirection, currentPoint)
	if err != nil {
		return nil, err
	}

	if slippageBps.IsZero() {
		result.MinimumAmountOut = result.AmountOut
		return result, nil
	}

	slippageFactor := N10000.Sub(slippageBps)
	// denominator = 10000
	denominator := N10000

	// minAmountOut = amountOut * slippageFactor / denominator

	minAmountOut := decimal.NewFromBigInt(result.AmountOut, 0).Mul(slippageFactor).Div(denominator)
	result.MinimumAmountOut = minAmountOut.BigInt()

	return result, nil
}
