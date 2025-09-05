package dynamic_bonding_curve

import (
	"errors"

	"github.com/krazyTry/meteora-go/u128"

	binary "github.com/gagliardetto/binary"
	"github.com/shopspring/decimal"
)

func getSwapAmountFromQuoteToBase(curve []LiquidityDistributionConfig, sqrtStartPrice binary.Uint128, amountIn decimal.Decimal) (decimal.Decimal, binary.Uint128, error) {
	if amountIn.IsZero() {
		return decimal.Zero, sqrtStartPrice, nil
	}

	totalOutput := decimal.Zero
	sqrtPrice := decimal.NewFromBigInt(sqrtStartPrice.BigInt(), 0)
	amountLeft := amountIn

	// iterate through the curve points
	for _, point := range curve {
		pointSqrtPrice := decimal.NewFromBigInt(point.SqrtPrice.BigInt(), 0)
		pointLiquidity := decimal.NewFromBigInt(point.Liquidity.BigInt(), 0)

		// skip if liquidity is zero
		if pointSqrtPrice.IsZero() || pointLiquidity.IsZero() {
			continue
		}

		if pointSqrtPrice.Cmp(sqrtPrice) > 0 {
			maxAmountIn := getDeltaAmountQuoteUnsigned(sqrtPrice, pointSqrtPrice, pointLiquidity, true)

			if amountLeft.Cmp(maxAmountIn) < 0 {
				nextSqrtPrice, err := getNextSqrtPriceFromInput(sqrtPrice, pointLiquidity, amountLeft, false)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, err
				}
				outputAmount, err := getDeltaAmountBaseUnsigned(sqrtPrice, nextSqrtPrice, pointLiquidity, true)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, err
				}
				totalOutput = totalOutput.Add(outputAmount)
				sqrtPrice = nextSqrtPrice
				amountLeft = decimal.Zero

				break
			} else {
				nextSqrtPrice := pointSqrtPrice

				outputAmount, err := getDeltaAmountBaseUnsigned(sqrtPrice, nextSqrtPrice, pointLiquidity, true)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, err
				}
				totalOutput = totalOutput.Add(outputAmount)
				sqrtPrice = nextSqrtPrice
				amountLeft = amountLeft.Sub(maxAmountIn)
			}
		}
	}

	if !amountLeft.IsZero() {
		return decimal.Decimal{}, binary.Uint128{}, errors.New("not enough liquidity to process the entire amount")
	}

	return totalOutput, u128.GenUint128FromString(sqrtPrice.String()), nil
}

func getSwapAmountFromBaseToQuote(curve []LiquidityDistributionConfig, sqrtStartPrice binary.Uint128, amountIn decimal.Decimal) (decimal.Decimal, binary.Uint128, error) {

	if amountIn.IsZero() {
		return decimal.Zero, sqrtStartPrice, nil
	}

	totalOutput := decimal.Zero
	sqrtPrice := decimal.NewFromBigInt(sqrtStartPrice.BigInt(), 0)
	amountLeft := amountIn

	for i := range curve {
		cp := curve[i]
		currentSqrtPrice := decimal.NewFromBigInt(cp.SqrtPrice.BigInt(), 0)
		currentLiquidity := decimal.NewFromBigInt(cp.Liquidity.BigInt(), 0)

		if currentSqrtPrice.IsZero() || currentLiquidity.IsZero() {
			continue
		}

		if currentSqrtPrice.Cmp(sqrtPrice) < 0 {

			if i+1 < len(curve) {
				currentLiquidity = decimal.NewFromBigInt(curve[i+1].Liquidity.BigInt(), 0)
			}

			if currentLiquidity.IsZero() {
				continue
			}

			maxAmountIn, err := getDeltaAmountBaseUnsigned(currentSqrtPrice, sqrtPrice, currentLiquidity, true)
			if err != nil {
				return decimal.Decimal{}, binary.Uint128{}, err
			}

			if amountLeft.Cmp(maxAmountIn) < 0 {

				nextSqrt, err := getNextSqrtPriceFromInput(sqrtPrice, currentLiquidity, amountLeft, true)
				if err != nil {
					return decimal.Decimal{}, binary.Uint128{}, err
				}

				outputAmount := getDeltaAmountQuoteUnsigned(nextSqrt, sqrtPrice, currentLiquidity, true)
				totalOutput = totalOutput.Add(outputAmount)
				sqrtPrice = nextSqrt
				amountLeft = decimal.Zero
				break

			} else {

				nextSqrt := currentSqrtPrice
				outputAmount := getDeltaAmountQuoteUnsigned(nextSqrt, sqrtPrice, currentLiquidity, true)
				totalOutput = totalOutput.Add(outputAmount)
				sqrtPrice = nextSqrt
				amountLeft = amountLeft.Sub(maxAmountIn)

			}
		}
	}

	zero := decimal.Zero

	if amountLeft.Cmp(zero) > 0 && decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0).Cmp(zero) > 0 {

		nextSqrt, err := getNextSqrtPriceFromInput(sqrtPrice, decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0), amountLeft, true)
		if err != nil {
			return decimal.Decimal{}, binary.Uint128{}, err
		}

		outputAmount := getDeltaAmountQuoteUnsigned(nextSqrt, sqrtPrice, decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0), true)
		totalOutput = totalOutput.Add(outputAmount)
		sqrtPrice = nextSqrt

	}

	return totalOutput, u128.GenUint128FromString(sqrtPrice.String()), nil
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
	tradeFeeNumerator := decimal.Min(baseFeeNumerator, decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0))

	// apply fees on input if needed
	if feeMode.FeesOnInput {

		// tradingFee = amountIn * tradeFeeNumerator / FEE_DENOMINATOR
		tradingFee := inAmount.Mul(tradeFeeNumerator).Div(decimal.NewFromBigInt(FEE_DENOMINATOR, 0))

		// actualAmountIn = amountIn - tradingFee
		inAmount = inAmount.Sub(tradingFee)

	}

	// calculate swap amount
	amountOut, _, err := getSwapAmountFromQuoteToBase(configState.Curve[:], configState.SqrtStartPrice, inAmount)
	if err != nil {
		return decimal.Decimal{}, err
	}

	if !feeMode.FeesOnInput {
		// tradingFee = outputAmount * tradeFeeNumerator / FEE_DENOMINATOR
		tradingFee := amountOut.Mul(tradeFeeNumerator).Div(decimal.NewFromBigInt(FEE_DENOMINATOR, 0))

		// actualAmountOut = outputAmount - tradingFee
		amountOut = amountOut.Sub(tradingFee)

	}

	if slippageBps > 0 {

		// slippageFactor = 10000 - slippageBps
		slippageFactor := decimal.NewFromInt(10000).Sub(decimal.NewFromUint64(uint64(slippageBps)))
		// denominator = 10000
		denominator := decimal.NewFromInt(10000)
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

	slippageFactor := decimal.NewFromInt(10000).Sub(slippageBps)
	// denominator = 10000
	denominator := decimal.NewFromInt(10000)

	// minAmountOut = amountOut * slippageFactor / denominator

	minAmountOut := decimal.NewFromBigInt(result.AmountOut, 0).Mul(slippageFactor).Div(denominator)
	result.MinimumAmountOut = minAmountOut.BigInt()

	return result, nil
}
