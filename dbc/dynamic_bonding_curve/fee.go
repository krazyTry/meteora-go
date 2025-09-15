package dynamic_bonding_curve

import (
	"errors"

	"github.com/shopspring/decimal"
)

// FeeMode represents the fee collection mode configuration
type FeeMode struct {
	FeesOnInput     bool // Whether fees are collected on input tokens
	FeesOnBaseToken bool // Whether fees are collected on base tokens
	HasReferral     bool // Whether referral fees are enabled
}

// getFeeMode creates a FeeMode configuration based on fee collection mode, trade direction, and referral status
func getFeeMode(collectFeeMode CollectFeeMode, tradeDirection TradeDirection, hasReferral bool) *FeeMode {
	return &FeeMode{
		FeesOnInput:     tradeDirection == TradeDirectionQuoteToBase && collectFeeMode == CollectFeeModeQuoteToken,
		FeesOnBaseToken: tradeDirection == TradeDirectionQuoteToBase && collectFeeMode == CollectFeeModeOutputToken,
		HasReferral:     hasReferral,
	}
}

// getVariableFeeNumerator calculates the variable fee numerator based on dynamic fee configuration and volatility
func getVariableFeeNumerator(
	dynamicFee DynamicFeeConfig,
	volatilityTracker VolatilityTracker,
) decimal.Decimal {

	if dynamicFee.Initialized == 0 {
		return N0
	}

	// 1. volatilityTimesBinStep = volatilityAccumulator * binStep
	volatilityTimesBinStep := decimal.NewFromBigInt(volatilityTracker.VolatilityAccumulator.BigInt(), 0).Mul(
		decimal.NewFromUint64(uint64(dynamicFee.BinStep)),
	)

	// squareVfaBin = (volatilityTimesBinStep)^2
	squareVfaBin := volatilityTimesBinStep.Mul(volatilityTimesBinStep)

	// 2. vFee = squareVfaBin * variableFeeControl
	vFee := squareVfaBin.Mul(decimal.NewFromUint64(uint64(dynamicFee.VariableFeeControl)))

	// 3. scaledVFee = (vFee + offset) / scalingFactor
	return vFee.Add(N99_999_999_999).Div(N100_000_000_000)
}

// getTotalFeeNumerator calculates the total fee numerator by combining base fee and variable fee
func getTotalFeeNumerator(
	baseFeeNumerator decimal.Decimal,
	dynamicFee DynamicFeeConfig,
	volatilityTracker VolatilityTracker,
) decimal.Decimal {
	// 1. variableFeeNumerator
	variableFeeNumerator := getVariableFeeNumerator(dynamicFee, volatilityTracker)

	// 2. totalFeeNumerator = variableFeeNumerator + baseFeeNumerator
	totalFeeNumerator := variableFeeNumerator.Add(baseFeeNumerator)

	// 3. capped = min(totalFeeNumerator, MAX_FEE_NUMERATOR)
	if totalFeeNumerator.Cmp(MAX_FEE_NUMERATOR) > 0 {
		return MAX_FEE_NUMERATOR
	}

	return totalFeeNumerator
}

// getTotalFeeNumeratorFromIncludedFeeAmount calculates the total fee numerator from an included fee amount
func getTotalFeeNumeratorFromIncludedFeeAmount(
	poolFees PoolFeesConfig,
	volatilityTracker VolatilityTracker,
	currentPoint decimal.Decimal,
	activationPoint decimal.Decimal,
	includedFeeAmount decimal.Decimal,
	tradeDirection TradeDirection,
) (decimal.Decimal, error) {

	var baseFeeNumerator decimal.Decimal

	switch poolFees.BaseFee.BaseFeeMode {
	case BaseFeeModeFeeSchedulerLinear, BaseFeeModeFeeSchedulerExponential:

		cliffFeeNumerator := decimal.NewFromUint64(poolFees.BaseFee.CliffFeeNumerator)
		numberOfPeriod := decimal.NewFromUint64(uint64(poolFees.BaseFee.FirstFactor))
		periodFrequency := decimal.NewFromUint64(poolFees.BaseFee.SecondFactor)
		reductionFactor := decimal.NewFromUint64(poolFees.BaseFee.ThirdFactor)
		feeSchedulerMode := poolFees.BaseFee.BaseFeeMode

		baseFee, err := getBaseFeeNumerator7(cliffFeeNumerator, numberOfPeriod, periodFrequency, reductionFactor, feeSchedulerMode, currentPoint, activationPoint)
		if err != nil {
			return decimal.Decimal{}, err
		}
		baseFeeNumerator = baseFee

	case BaseFeeModeRateLimiter:
		cliffFeeNumerator := decimal.NewFromUint64(poolFees.BaseFee.CliffFeeNumerator)
		feeIncrementBps := decimal.NewFromUint64(uint64(poolFees.BaseFee.FirstFactor))
		maxLimiterDuration := decimal.NewFromUint64(poolFees.BaseFee.SecondFactor)
		referenceAmount := decimal.NewFromUint64(poolFees.BaseFee.ThirdFactor)

		if isRateLimiterApplied(
			currentPoint,
			activationPoint,
			tradeDirection,
			maxLimiterDuration,
			referenceAmount,
			feeIncrementBps,
		) {
			baseFee, err := getFeeNumeratorFromIncludedAmount(
				cliffFeeNumerator,
				referenceAmount,
				feeIncrementBps,
				includedFeeAmount,
			)
			if err != nil {
				return decimal.Decimal{}, err
			}
			baseFeeNumerator = baseFee
		} else {
			baseFeeNumerator = cliffFeeNumerator
		}

	default:
		return decimal.Decimal{}, errors.New("invalid base fee mode")
	}

	// 3. Return totalFee
	return getTotalFeeNumerator(
		baseFeeNumerator,
		poolFees.DynamicFee,
		volatilityTracker,
	), nil
}

// isZeroRateLimiter checks if all rate limiter parameters are zero
func isZeroRateLimiter(
	referenceAmount decimal.Decimal,
	maxLimiterDuration decimal.Decimal,
	feeIncrementBps decimal.Decimal,
) bool {
	return referenceAmount.IsZero() && maxLimiterDuration.IsZero() && feeIncrementBps.IsZero()
}

// isRateLimiterApplied determines if rate limiting should be applied based on current conditions
func isRateLimiterApplied(
	currentPoint decimal.Decimal,
	activationPoint decimal.Decimal,
	tradeDirection TradeDirection,
	maxLimiterDuration decimal.Decimal,
	referenceAmount decimal.Decimal,
	feeIncrementBps decimal.Decimal,
) bool {
	// 1. If all RateLimiter parameters are zero, do not apply rate limiting
	if isZeroRateLimiter(referenceAmount, maxLimiterDuration, feeIncrementBps) {
		return false
	}

	// 2. Only handle quote -> base trade direction
	if tradeDirection == TradeDirectionBaseToQuote {
		return false
	}

	// 3. Calculate the last effective RateLimiter point
	lastEffectiveRateLimiterPoint := activationPoint.Add(maxLimiterDuration)

	// 4. Check if currentPoint <= lastEffectiveRateLimiterPoint
	return currentPoint.Cmp(lastEffectiveRateLimiterPoint) <= 0
}

// toNumerator converts basis points to numerator using the given fee denominator
func toNumerator(bps, feeDenominator decimal.Decimal) (decimal.Decimal, error) {
	numerator, err := mulDiv(bps, feeDenominator, BASIS_POINT_MAX, false)
	if err != nil {
		return decimal.Decimal{}, err
	}
	return numerator, nil
}

// getMaxIndex1 calculates the maximum index for fee increment calculations
func getMaxIndex1(cliffFeeNumerator, feeIncrementBps decimal.Decimal) (decimal.Decimal, error) {
	// Check if cliffFeeNumerator exceeds the maximum value
	if cliffFeeNumerator.Cmp(MAX_FEE_NUMERATOR) > 0 {
		return decimal.Decimal{}, errors.New("cliff fee numerator exceeds maximum fee numerator")
	}

	// deltaNumerator = MAX_FEE_NUMERATOR - cliffFeeNumerator
	deltaNumerator := MAX_FEE_NUMERATOR.Sub(cliffFeeNumerator)

	// feeIncrementNumerator = toNumerator(feeIncrementBps, FEE_DENOMINATOR)
	feeIncrementNumerator, err := toNumerator(feeIncrementBps, FEE_DENOMINATOR)
	if err != nil {
		return decimal.Decimal{}, err
	}

	// Check if feeIncrementNumerator is zero
	if feeIncrementNumerator.Sign() == 0 {
		return decimal.Decimal{}, errors.New("fee increment numerator cannot be zero")
	}

	// Return deltaNumerator / feeIncrementNumerator
	return deltaNumerator.Div(feeIncrementNumerator), nil
}

// getFeeNumeratorFromIncludedAmount translates the TS version to Go
func getFeeNumeratorFromIncludedAmount(
	cliffFeeNumerator decimal.Decimal,
	referenceAmount decimal.Decimal,
	feeIncrementBps decimal.Decimal,
	includedFeeAmount decimal.Decimal,
) (decimal.Decimal, error) {
	// if (includedFeeAmount <= referenceAmount) return cliffFeeNumerator
	if includedFeeAmount.Cmp(referenceAmount) <= 0 {
		return cliffFeeNumerator, nil
	}

	c := cliffFeeNumerator
	diff := includedFeeAmount.Sub(referenceAmount)

	a := diff.Div(referenceAmount) // integer division
	b := diff.Mod(referenceAmount)

	maxIndex, err := getMaxIndex1(cliffFeeNumerator, feeIncrementBps)
	if err != nil {
		return decimal.Decimal{}, err
	}

	i, err := toNumerator(feeIncrementBps, FEE_DENOMINATOR)
	if err != nil {
		return decimal.Decimal{}, err
	}

	x0 := referenceAmount
	// one := big.NewInt(1)
	// two := big.NewInt(2)

	var tradingFeeNumerator decimal.Decimal

	if a.Cmp(maxIndex) < 0 {
		// numerator1 = c + c*a + i*a*(a+1)/2
		numerator1 := c.Add(c.Mul(a)).Add(i.Mul(a).Mul(a.Add(N1)).Div(N2))

		// numerator2 = c + i*(a+1)
		numerator2 := c.Add(i.Mul(a.Add(N1)))
		// numerator2 := new(big.Int).Add(c, new(big.Int).Mul(i, new(big.Int).Add(a, one)))

		// firstFee = x0 * numerator1
		firstFee := x0.Mul(numerator1)
		// firstFee := new(big.Int).Mul(x0, numerator1)

		// secondFee = b * numerator2
		secondFee := b.Mul(numerator2)
		// secondFee := new(big.Int).Mul(b, numerator2)

		tradingFeeNumerator = firstFee.Add(secondFee)

	} else {
		// numerator1 = c + c*maxIndex + i*maxIndex*(maxIndex+1)/2
		numerator1 := c.Add(c.Mul(maxIndex)).Add(i.Mul(maxIndex).Mul(maxIndex.Add(N1)).Div(N2))

		// numerator2 = MAX_FEE_NUMERATOR
		numerator2 := MAX_FEE_NUMERATOR

		// firstFee = x0 * numerator1
		firstFee := x0.Mul(numerator1)
		// firstFee := new(big.Int).Mul(x0, numerator1)

		// d = a - maxIndex
		d := a.Sub(maxIndex)

		// leftAmount = d*x0 + b
		leftAmount := d.Mul(x0).Add(b)
		// leftAmount := new(big.Int).Add(new(big.Int).Mul(d, x0), b)

		// secondFee = leftAmount * numerator2
		secondFee := leftAmount.Mul(numerator2)

		tradingFeeNumerator = firstFee.Add(secondFee)
	}

	denominator := FEE_DENOMINATOR

	// tradingFee = (tradingFeeNumerator + denominator - 1) / denominator
	tradingFee := tradingFeeNumerator.Add(denominator).Sub(N1).Div(denominator)

	// feeNumerator = tradingFee * FEE_DENOMINATOR / includedFeeAmount
	feeNumerator, err := mulDiv(
		tradingFee,
		FEE_DENOMINATOR,
		includedFeeAmount,
		true,
	)
	if err != nil {
		return decimal.Decimal{}, errors.New("calculation overflow in getFeeNumeratorFromIncludedAmount")
	}

	return feeNumerator, nil
}

// getExcludedFeeAmount calculates the amount after excluding fees and the trading fee
func getExcludedFeeAmount(
	tradeFeeNumerator decimal.Decimal,
	includedFeeAmount decimal.Decimal,
) (decimal.Decimal, decimal.Decimal, error) {
	// tradingFee = mulDiv(includedFeeAmount * tradeFeeNumerator / FEE_DENOMINATOR, Rounding.Up)
	tradingFee, err := mulDiv(
		includedFeeAmount,
		tradeFeeNumerator,
		FEE_DENOMINATOR,
		true,
	)
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, err
	}

	// excludedFeeAmount = includedFeeAmount - tradingFee
	excludedFeeAmount := includedFeeAmount.Sub(tradingFee)

	return excludedFeeAmount, tradingFee, nil
}

// getFeeOnAmount calculates fee distribution including protocol fees, referral fees, and trading fees
func getFeeOnAmount(
	tradeFeeNumerator decimal.Decimal,
	amount decimal.Decimal,
	poolFees PoolFeesConfig,
	hasReferral bool,
) (amountAfterFee, updatedProtocolFee, referralFee, updatedTradingFee decimal.Decimal, err error) {
	// 1. First calculate the amount after removing trading fees
	amountAfterFee, tradingFee, err := getExcludedFeeAmount(tradeFeeNumerator, amount)
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
	}

	// 2. Calculate protocol fee (protocolFee = tradingFee * protocolFeePercent / 100)
	protocolFee, err := mulDiv(
		tradingFee,
		decimal.NewFromUint64(uint64(poolFees.ProtocolFeePercent)),
		N100,
		false,
	)
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
	}

	// 3. Update trading fee (subtract protocol fee)
	updatedTradingFee = tradingFee.Sub(protocolFee)

	// 4. Calculate referral reward (referralFee = protocolFee * referralFeePercent / 100)
	if hasReferral {
		referralFee, err = mulDiv(
			protocolFee,
			decimal.NewFromUint64(uint64(poolFees.ReferralFeePercent)),
			N100,
			false,
		)
		if err != nil {
			return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
		}
	} else {
		referralFee = N0
	}

	// 5. Update protocol fee (protocolFee - referralFee)
	updatedProtocolFee = protocolFee.Sub(referralFee)

	// 6. Return results
	return amountAfterFee, updatedProtocolFee, referralFee, updatedTradingFee, nil
}
