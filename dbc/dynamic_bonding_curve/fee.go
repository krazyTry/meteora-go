package dynamic_bonding_curve

import (
	"github.com/shopspring/decimal"
)

type FeeMode struct {
	FeesOnInput     bool
	FeesOnBaseToken bool
	HasReferral     bool
}

func getFeeMode(collectFeeMode CollectFeeMode, tradeDirection TradeDirection, hasReferral bool) *FeeMode {
	return &FeeMode{
		FeesOnInput:     tradeDirection == TradeDirectionQuoteToBase && collectFeeMode == CollectFeeModeQuoteToken,
		FeesOnBaseToken: tradeDirection == TradeDirectionQuoteToBase && collectFeeMode == CollectFeeModeOutputToken,
		HasReferral:     hasReferral,
	}
}

// getVariableFee
func getVariableFee(dynamicFee DynamicFeeConfig, volatilityTracker VolatilityTracker) decimal.Decimal {
	if dynamicFee.Initialized == 0 || decimal.NewFromBigInt(volatilityTracker.VolatilityAccumulator.BigInt(), 0).Equal(decimal.Zero) {
		return decimal.Zero
	}

	// volatilityTimesBinStep = volatilityAccumulator * binStep
	volatilityTimesBinStep := decimal.NewFromBigInt(volatilityTracker.VolatilityAccumulator.BigInt(), 0).Mul(
		decimal.NewFromInt(int64(dynamicFee.BinStep)),
	)

	// squared = (volatilityTimesBinStep)^2
	squared := volatilityTimesBinStep.Mul(volatilityTimesBinStep)

	// vFee = squared * variableFeeControl
	vFee := squared.Mul(decimal.NewFromInt(int64(dynamicFee.VariableFeeControl)))

	// scaleFactor = 100_000_000_000
	scaleFactor := decimal.NewFromInt(100_000_000_000)

	// numerator = vFee + (scaleFactor - 1)
	numerator := vFee.Add(scaleFactor.Sub(decimal.NewFromInt(1)))

	// return numerator / scaleFactor
	return numerator.Div(scaleFactor)
}

func getBaseFeeNumerator(
	baseFee BaseFeeConfig,
	tradeDirection TradeDirection,
	currentPoint, activationPoint, inputAmount decimal.Decimal, // inputAmount 可以为零值 decimal.Zero
) decimal.Decimal {

	baseFeeMode := baseFee.BaseFeeMode
	cliffFeeNumerator := decimal.NewFromInt(int64(baseFee.CliffFeeNumerator))
	thirdFactor := decimal.NewFromInt(int64(baseFee.ThirdFactor))
	firstFactor := decimal.NewFromInt(int64(baseFee.FirstFactor))
	secondFactor := decimal.NewFromInt(int64(baseFee.SecondFactor))

	if baseFeeMode == BaseFeeModeRateLimiter {
		feeIncrementBps := firstFactor

		isBaseToQuote := tradeDirection == TradeDirectionBaseToQuote

		isRateLimiterApplied := CheckRateLimiterApplied(
			baseFeeMode,
			isBaseToQuote,
			currentPoint.BigInt(),
			activationPoint.BigInt(),
			secondFactor.BigInt(),
		)

		if currentPoint.LessThan(activationPoint) {
			return cliffFeeNumerator
		}

		// lastEffectivePoint = activationPoint + maxLimiterDuration
		lastEffectivePoint := activationPoint.Add(secondFactor)
		if currentPoint.GreaterThan(lastEffectivePoint) {
			return cliffFeeNumerator
		}

		if inputAmount.IsZero() {
			return cliffFeeNumerator
		}

		if isRateLimiterApplied {
			return getFeeNumeratorOnRateLimiter(
				cliffFeeNumerator,
				thirdFactor,
				feeIncrementBps,
				inputAmount,
			)
		} else {
			return cliffFeeNumerator
		}
	} else {
		numberOfPeriod := firstFactor
		periodFrequency := secondFactor

		if periodFrequency.IsZero() {
			return cliffFeeNumerator
		}

		var period decimal.Decimal
		if currentPoint.LessThan(activationPoint) {
			period = numberOfPeriod
		} else {
			elapsedPoints := currentPoint.Sub(activationPoint)
			periodCount := elapsedPoints.Div(periodFrequency).Floor()
			if periodCount.GreaterThan(numberOfPeriod) {
				period = numberOfPeriod
			} else {
				period = periodCount
			}
		}

		if baseFeeMode == BaseFeeModeFeeSchedulerLinear {
			return getFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, thirdFactor, period)
		} else {
			return getFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, thirdFactor, period)
		}
	}
}

func getFeeOnAmount(
	amount decimal.Decimal,
	poolFees PoolFeesConfig,
	isReferral bool,
	currentPoint decimal.Decimal,
	activationPoint decimal.Decimal,
	volatilityTracker VolatilityTracker,
	tradeDirection TradeDirection,
) (amountAfterFee, tradingFeeAfterProtocol, protocolFeeAfterReferral, referralFee decimal.Decimal, err error) {

	// 1. base fee
	var inputAmount decimal.Decimal
	if poolFees.BaseFee.BaseFeeMode == BaseFeeModeRateLimiter {
		inputAmount = amount
	}
	baseFeeNumerator := getBaseFeeNumerator(poolFees.BaseFee, tradeDirection, currentPoint, activationPoint, inputAmount)

	// 2. add dynamic fee if enabled
	totalFeeNumerator := baseFeeNumerator
	if poolFees.DynamicFee.Initialized != 0 {
		variableFee := getVariableFee(poolFees.DynamicFee, volatilityTracker)
		totalFeeNumerator = totalFeeNumerator.Add(variableFee)
	}

	// 3. cap at MAX_FEE_NUMERATOR
	if totalFeeNumerator.GreaterThan(decimal.NewFromInt(MAX_FEE_NUMERATOR.Int64())) {
		totalFeeNumerator = decimal.NewFromInt(MAX_FEE_NUMERATOR.Int64())
	}

	// 4. trading fee: tradingFee = amount * totalFeeNumerator / FEE_DENOMINATOR
	tradingFee, err := mulDiv(amount, totalFeeNumerator, decimal.NewFromInt(FEE_DENOMINATOR.Int64()), true)
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
	}

	amountAfterFee = amount.Sub(tradingFee)

	// 5. protocol fee
	protocolFee, err := mulDiv(tradingFee, decimal.NewFromInt(int64(poolFees.ProtocolFeePercent)), decimal.NewFromInt(100), false)
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
	}
	tradingFeeAfterProtocol = tradingFee.Sub(protocolFee)

	// 6. referral fee
	referralFee = decimal.Zero
	if isReferral {
		referralFee, err = mulDiv(protocolFee, decimal.NewFromInt(int64(poolFees.ReferralFeePercent)), decimal.NewFromInt(100), false)
		if err != nil {
			return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
		}
	}

	// 7. protocol fee after referral
	protocolFeeAfterReferral = protocolFee.Sub(referralFee)

	return amountAfterFee, tradingFeeAfterProtocol, protocolFeeAfterReferral, referralFee, nil
}
