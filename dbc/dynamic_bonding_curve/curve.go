package dynamic_bonding_curve

import (
	"errors"
	"fmt"
	"math/big"

	dmath "github.com/krazyTry/meteora-go/decimal_math"

	"github.com/krazyTry/meteora-go/u128"

	"github.com/shopspring/decimal"
)

func mulDiv(x, y, denominator decimal.Decimal, roundUp bool) (decimal.Decimal, error) {

	if denominator.IsZero() {
		return N0, errors.New("MulDivDecimal: division by zero")
	}

	if denominator.Equal(N1) || x.IsZero() || y.IsZero() {
		return x.Mul(y), nil
	}

	prod := x.Mul(y)

	if roundUp {
		return prod.Add(denominator.Sub(N1)).Div(denominator).Floor(), nil
	} else {
		return prod.Div(denominator).Floor(), nil
	}
}

func truncateSig(d decimal.Decimal, sig int) decimal.Decimal {
	return d.Round(int32(sig - len(d.Coefficient().String())))
}

func getDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity decimal.Decimal, roundUp bool) (decimal.Decimal, error) {
	if liquidity.IsZero() {
		return N0, nil
	}

	if lowerSqrtPrice.IsZero() || upperSqrtPrice.IsZero() {
		return decimal.Decimal{}, errors.New("sqrt price cannot be zero")
	}

	// numerator = upper - lower
	numerator := upperSqrtPrice.Sub(lowerSqrtPrice)
	// denominator = lower * upper
	denominator := lowerSqrtPrice.Mul(upperSqrtPrice)

	// result = liquidity * numerator / denominator
	return mulDiv(liquidity, numerator, denominator, roundUp)
}

// getDeltaAmountQuoteUnsigned  Δb = L(√P_upper - √P_lower)
func getDeltaAmountQuoteUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity decimal.Decimal, roundUp bool) decimal.Decimal {
	// ΔsqrtPrice = upper - lower
	deltaSqrtPrice := upperSqrtPrice.Sub(lowerSqrtPrice)

	// prod = liquidity * deltaSqrtPrice
	prod := liquidity.Mul(deltaSqrtPrice)

	if roundUp {
		// numerator = prod + denominator - 1 (ceiling division)
		numerator := prod.Add(Q128).Sub(N1)
		return numerator.DivRound(Q128, 38).Floor()
	} else {
		return prod.DivRound(Q128, 38).Floor()
	}
}

func getNextSqrtPriceFromAmountBaseRoundingUp(sqrtPrice, liquidity, amount decimal.Decimal) (decimal.Decimal, error) {
	if amount.IsZero() {
		return sqrtPrice, nil
	}

	// product = amount * sqrtPrice
	product := amount.Mul(sqrtPrice)

	// denominator = liquidity + product
	denominator := liquidity.Add(product)

	return mulDiv(liquidity, sqrtPrice, denominator, true)
}

func getNextSqrtPriceFromAmountQuoteRoundingDown(sqrtPrice, liquidity, amount decimal.Decimal) decimal.Decimal {
	if amount.IsZero() {
		return sqrtPrice
	}

	shifted := amount.Mul(Q128)

	quotient := shifted.Div(liquidity)

	return sqrtPrice.Add(quotient).Floor()
}

func getNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn decimal.Decimal, baseForQuote bool) (decimal.Decimal, error) {
	if sqrtPrice.IsZero() || liquidity.IsZero() {
		return decimal.Decimal{}, errors.New("price or liquidity cannot be zero")
	}

	if baseForQuote {
		return getNextSqrtPriceFromAmountBaseRoundingUp(sqrtPrice, liquidity, amountIn)
	} else {
		return getNextSqrtPriceFromAmountQuoteRoundingDown(sqrtPrice, liquidity, amountIn), nil
	}
}

// getInitialLiquidityFromDeltaBase calculates initial liquidity given base amount and price ranges.
func getInitialLiquidityFromDeltaBase(baseAmount, sqrtMaxPrice, sqrtPrice decimal.Decimal) decimal.Decimal {
	if baseAmount.IsZero() {
		return N0
	}

	// priceDelta = sqrtMaxPrice - sqrtPrice
	priceDelta := sqrtMaxPrice.Sub(sqrtPrice)

	// prod = baseAmount * sqrtPrice * sqrtMaxPrice
	prod := baseAmount.Mul(sqrtPrice).Mul(sqrtMaxPrice)

	// liquidity = prod / priceDelta
	liquidity := prod.DivRound(priceDelta, 38).Floor()

	return liquidity
}

func convertDecimalToBN(value decimal.Decimal) decimal.Decimal {
	return value.Floor()
}

// ConvertToLamports
func convertToLamports(amount decimal.Decimal, tokenDecimal TokenDecimal) decimal.Decimal {
	valueInLamports := amount.Mul(decimal.New(1, int32(tokenDecimal)))
	return convertDecimalToBN(valueInLamports)
}

func getSqrtPriceFromPrice(decimalPrice decimal.Decimal, tokenADecimal, tokenBDecimal TokenDecimal) decimal.Decimal {

	decimalsAdjustment := decimal.New(1, int32(tokenADecimal)-int32(tokenBDecimal)) // 10^(tokenADecimal - tokenBDecimal)

	adjusted := decimalPrice.Div(decimalsAdjustment)

	return dmath.Sqrt(adjusted, 70).Mul(Q64).Floor()
}

func getMigrationBaseToken(
	migrationQuoteAmount, sqrtMigrationPrice decimal.Decimal,
	migrationOption MigrationOption,
) (decimal.Decimal, error) {

	switch migrationOption {
	case MigrationOptionMETDAMM:
		// price = sqrtMigrationPrice^2
		price := sqrtMigrationPrice.Mul(sqrtMigrationPrice)

		// quote = migrationQuoteAmount << 128
		quote := dmath.Lsh(migrationQuoteAmount, 128)

		// divmod: div = ceil(quote / price)
		return quote.Div(price).Ceil(), nil
	case MigrationOptionMETDAMMV2:
		liquidity, err := getInitialLiquidityFromDeltaQuote(
			migrationQuoteAmount,
			MIN_SQRT_PRICE,
			sqrtMigrationPrice,
		)
		if err != nil {
			return N0, err
		}

		baseAmount, err := getDeltaAmountBaseUnsigned(
			sqrtMigrationPrice,
			MAX_SQRT_PRICE,
			liquidity,
			true,
		)
		if err != nil {
			return decimal.Zero, err
		}

		return baseAmount, nil

	default:
		return decimal.Zero, errors.New("invalid migration option")
	}
}

// getInitialLiquidityFromDeltaQuote
// liquidity = (quoteAmount << 128) / (sqrtPrice - sqrtMinPrice)
func getInitialLiquidityFromDeltaQuote(quoteAmount, sqrtMinPrice, sqrtPrice decimal.Decimal) (decimal.Decimal, error) {
	// priceDelta = sqrtPrice - sqrtMinPrice
	priceDelta := sqrtPrice.Sub(sqrtMinPrice)

	if priceDelta.IsZero() {
		return N0, errors.New("price delta cannot be zero")
	}

	// quoteAmountShifted = quoteAmount << 128
	quoteAmountShifted := dmath.Lsh(quoteAmount, 128)

	// liquidity = quoteAmountShifted / priceDelta
	liquidity := quoteAmountShifted.DivRound(priceDelta, 38).Floor()
	return liquidity, nil
}

// getTotalVestingAmount
func getTotalVestingAmount(lockedVesting *LockedVesting) decimal.Decimal {
	// totalVestingAmount = cliffUnlockAmount + amountPerPeriod * numberOfPeriod
	amountPerPeriod := decimal.NewFromUint64(lockedVesting.AmountPerPeriod)
	numberOfPeriod := decimal.NewFromUint64(lockedVesting.NumberOfPeriod)
	cliffUnlockAmount := decimal.NewFromUint64(lockedVesting.CliffUnlockAmount)

	return amountPerPeriod.Mul(numberOfPeriod).Add(cliffUnlockAmount)
}

// CurveStep
type CurveStep struct {
	SqrtPrice *big.Int
	Liquidity *big.Int
}

// FirstCurveResult
type FirstCurveResult struct {
	SqrtStartPrice decimal.Decimal
	Curve          []LiquidityDistributionParameters
}

// getFirstCurve
func getFirstCurve(
	migrationSqrtPrice, migrationBaseAmount, swapAmount, migrationQuoteThreshold decimal.Decimal,
	migrationFeePercent uint8,
) (*FirstCurveResult, error) {
	migrationFeePercentDecimal := decimal.NewFromUint64(uint64(migrationFeePercent))

	// denominator = swapAmount * (1 - migrationFeePercent/100)
	denominator := swapAmount.Mul(N1.Sub(migrationFeePercentDecimal.Div(N100)))

	// sqrtStartPrice = migrationSqrtPrice * migrationBaseAmount / denominator
	sqrtStartPriceDecimal := migrationSqrtPrice.Mul(migrationBaseAmount).Div(denominator)

	sqrtStartPrice := sqrtStartPriceDecimal.Floor()
	liquidity, err := getLiquidity(swapAmount, migrationQuoteThreshold, sqrtStartPrice, migrationSqrtPrice)
	if err != nil {
		return nil, err
	}

	return &FirstCurveResult{
		SqrtStartPrice: sqrtStartPrice,
		Curve: []LiquidityDistributionParameters{
			{
				SqrtPrice: u128.GenUint128FromString(migrationSqrtPrice.String()),
				Liquidity: u128.GenUint128FromString(liquidity.String()),
			},
		},
	}, nil
}

// getLiquidity
func getLiquidity(baseAmount, quoteAmount, minSqrtPrice, maxSqrtPrice decimal.Decimal) (decimal.Decimal, error) {
	liquidityFromBase := getInitialLiquidityFromDeltaBase(baseAmount, maxSqrtPrice, minSqrtPrice)

	liquidityFromQuote, err := getInitialLiquidityFromDeltaQuote(quoteAmount, minSqrtPrice, maxSqrtPrice)
	if err != nil {
		return decimal.Decimal{}, err
	}

	if liquidityFromBase.Cmp(liquidityFromQuote) < 0 {
		return liquidityFromBase, nil
	}
	return liquidityFromQuote, nil
}

// GetSwapAmountWithBuffer 给 swapAmount 加 25% buffer 并限制最大值
func getSwapAmountWithBuffer(
	swapBaseAmount decimal.Decimal,
	sqrtStartPrice decimal.Decimal,
	curve []LiquidityDistributionParameters,
) (decimal.Decimal, error) {
	// buffer = swapBaseAmount * 0.25
	buffer := swapBaseAmount.Mul(N025)
	swapAmountBuffer := swapBaseAmount.Add(buffer)

	// maxBaseAmountOnCurve = getBaseTokenForSwap(...)
	maxBaseAmountOnCurve, err := getBaseTokenForSwap(sqrtStartPrice, MAX_SQRT_PRICE, curve)
	if err != nil {
		return N0, err
	}

	if swapAmountBuffer.Cmp(maxBaseAmountOnCurve) > 0 {
		return maxBaseAmountOnCurve, nil
	}
	return swapAmountBuffer, nil
}

// GetPercentageSupplyOnMigration
func getPercentageSupplyOnMigration(
	initialMarketCap decimal.Decimal,
	migrationMarketCap decimal.Decimal,
	lockedVesting *LockedVesting,
	totalLeftover decimal.Decimal,
	totalTokenSupply decimal.Decimal,
) decimal.Decimal {

	sqrtRatio := dmath.Sqrt(initialMarketCap.Div(migrationMarketCap), 60)

	totalVestingAmount := getTotalVestingAmount(lockedVesting)

	vestingPercentage := totalVestingAmount.Mul(N100).Div(totalTokenSupply)

	leftoverPercentage := totalLeftover.Mul(N100).Div(totalTokenSupply)

	numerator := N100.Mul(sqrtRatio).Sub(vestingPercentage.Add(leftoverPercentage).Mul(sqrtRatio))

	denominator := N1.Add(sqrtRatio)

	return numerator.DivRound(denominator, 13)
}

// GetMigrationQuoteAmount
func getMigrationQuoteAmount(migrationMarketCap decimal.Decimal, percentageSupplyOnMigration decimal.Decimal) decimal.Decimal {
	return migrationMarketCap.Mul(percentageSupplyOnMigration).DivRound(N100, 18)
}

func getBaseTokenForSwap(
	sqrtStartPrice, sqrtMigrationPrice decimal.Decimal,
	curve []LiquidityDistributionParameters,
) (decimal.Decimal, error) {
	totalAmount := decimal.Zero

	for i := 0; i < len(curve); i++ {
		var lowerSqrtPrice decimal.Decimal
		if i == 0 {
			lowerSqrtPrice = sqrtStartPrice
		} else {
			lowerSqrtPrice = decimal.NewFromBigInt(curve[i-1].SqrtPrice.BigInt(), 0)
		}

		curveSqrtPrice := decimal.NewFromBigInt(curve[i].SqrtPrice.BigInt(), 0)
		if curveSqrtPrice.Cmp(sqrtMigrationPrice) > 0 {
			// deltaAmount = getDeltaAmountBaseUnsigned(lower, sqrtMigrationPrice, liquidity, true)
			deltaAmount, err := getDeltaAmountBaseUnsigned(lowerSqrtPrice, sqrtMigrationPrice, decimal.NewFromBigInt(curve[i].Liquidity.BigInt(), 0), true)
			if err != nil {
				return decimal.Zero, err
			}
			totalAmount = totalAmount.Add(deltaAmount)
			break
		} else {
			deltaAmount, err := getDeltaAmountBaseUnsigned(lowerSqrtPrice, curveSqrtPrice, decimal.NewFromBigInt(curve[i].Liquidity.BigInt(), 0), true)
			if err != nil {
				return decimal.Zero, err
			}
			totalAmount = totalAmount.Add(deltaAmount)
		}
	}

	return totalAmount, nil
}

func getSqrtPriceFromMarketCap(
	marketCap float64,
	totalSupply float64,
	tokenBaseDecimal TokenDecimal,
	tokenQuoteDecimal TokenDecimal,
) decimal.Decimal {

	price := decimal.NewFromFloat(marketCap).Div(decimal.NewFromFloat(totalSupply))

	return getSqrtPriceFromPrice(price, tokenBaseDecimal, tokenQuoteDecimal)
}

func getDynamicFeeParams(baseFeeBps, maxPriceChangeBps int64) *DynamicFeeParameters {
	if maxPriceChangeBps > MAX_PRICE_CHANGE_BPS_DEFAULT {
		panic(fmt.Sprintf(
			"maxPriceChangeBps (%d bps) must be less than or equal to %d",
			maxPriceChangeBps,
			MAX_PRICE_CHANGE_BPS_DEFAULT,
		))
	}

	priceRatio := decimal.NewFromInt(maxPriceChangeBps).
		Div(BASIS_POINT_MAX).
		Add(decimal.NewFromInt(1))

	// Q64: sqrt(priceRatio) * 2^64
	sqrtPriceRatioQ64 := dmath.Sqrt(priceRatio, 200).Round(19).Mul(Q64).Floor()
	// 2️⃣ deltaBinId = (sqrtPriceRatioQ64 - ONE_Q64) / BIN_STEP_BPS_U128_DEFAULT * 2
	deltaBinId := sqrtPriceRatioQ64.Sub(Q64).Div(decimal.NewFromBigInt(BIN_STEP_BPS_U128_DEFAULT.BigInt(), 0)).Floor().Mul(N2)
	// 3️⃣ maxVolatilityAccumulator = deltaBinId * BASIS_POINT_MAX
	maxVolatilityAccumulator := deltaBinId.Mul(BASIS_POINT_MAX)
	// 4️⃣ squareVfaBin = (maxVolatilityAccumulator * BIN_STEP_BPS_DEFAULT)^2
	squareVfaBin := maxVolatilityAccumulator.Mul(BIN_STEP_BPS_DEFAULT).Pow(N2)
	// baseFeeNumerator
	baseFeeNumerator := bpsToFeeNumerator(baseFeeBps)

	// maxDynamicFeeNumerator = baseFeeNumerator * 20 / 100
	maxDynamicFeeNumerator := baseFeeNumerator.Mul(decimal.NewFromInt(20)).Div(N100)

	// vFee = maxDynamicFeeNumerator * 100_000_000_000 - 99_999_999_999
	vFee := maxDynamicFeeNumerator.Mul(N100_000_000_000).Sub(N99_999_999_999)

	// variableFeeControl = vFee / squareVfaBin
	variableFeeControl := vFee.Div(squareVfaBin).Floor()

	return &DynamicFeeParameters{
		BinStep:                  uint16(BIN_STEP_BPS_DEFAULT.BigInt().Int64()),
		BinStepU128:              u128.GenUint128FromString(BIN_STEP_BPS_U128_DEFAULT.String()),
		FilterPeriod:             DYNAMIC_FEE_FILTER_PERIOD_DEFAULT,
		DecayPeriod:              DYNAMIC_FEE_DECAY_PERIOD_DEFAULT,
		ReductionFactor:          DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT,
		MaxVolatilityAccumulator: uint32(maxVolatilityAccumulator.BigInt().Uint64()),
		VariableFeeControl:       uint32(variableFeeControl.BigInt().Uint64()),
	}
}

// bpsToFeeNumerator
func bpsToFeeNumerator(bps int64) decimal.Decimal {
	return decimal.NewFromInt(bps).Mul(FEE_DENOMINATOR).Div(BASIS_POINT_MAX)
}

type TwoCurveResult struct {
	IsOk           bool
	SqrtStartPrice decimal.Decimal
	Curve          []LiquidityDistributionParameters
}

func getTwoCurve(
	migrationSqrtPrice, midSqrtPrice, initialSqrtPrice, swapAmount, migrationQuoteThreshold decimal.Decimal,
) TwoCurveResult {
	p0 := initialSqrtPrice
	p1 := midSqrtPrice
	p2 := migrationSqrtPrice

	// a1 = 1/p0 - 1/p1
	a1 := N1.DivRound(p0, 36).Sub(N1.DivRound(p1, 37))

	// b1 = 1/p1 - 1/p2
	b1 := N1.DivRound(p1, 37).Sub(N1.DivRound(p2, 37))

	c1 := swapAmount

	// a2 = p1 - p0
	a2 := p1.Sub(p0)

	// b2 = p2 - p1
	b2 := p2.Sub(p1)

	c2 := truncateSig(migrationQuoteThreshold.Mul(truncateSig(Q128, 20)), 20)

	// l0 = (c1*b2 - c2*b1) / (a1*b2 - a2*b1)
	numeratorL0 := truncateSig(c1.Mul(b2).Sub(c2.Mul(b1)).Floor(), 20)

	denominatorL0 := a1.Mul(b2).Sub(a2.Mul(b1)).Round(18)

	l0 := truncateSig(numeratorL0.Div(denominatorL0).Floor(), 20)

	// l1 = (c1*a2 - c2*a1) / (b1*a2 - b2*a1)
	numeratorL1 := truncateSig(c1.Mul(a2).Sub(c2.Mul(a1)).Floor(), 21)
	denominatorL1 := b1.Mul(a2).Sub(b2.Mul(a1)).Round(18)
	l1 := truncateSig(numeratorL1.Div(denominatorL1).Floor(), 20)

	if l0.IsNegative() || l1.IsNegative() {
		return TwoCurveResult{
			IsOk:           false,
			SqrtStartPrice: decimal.Zero,
			Curve:          []LiquidityDistributionParameters{},
		}
	}

	return TwoCurveResult{
		IsOk:           true,
		SqrtStartPrice: initialSqrtPrice,
		Curve: []LiquidityDistributionParameters{
			{
				SqrtPrice: u128.GenUint128FromString(midSqrtPrice.String()),
				Liquidity: u128.GenUint128FromString(l0.Floor().String()),
			},
			{
				SqrtPrice: u128.GenUint128FromString(migrationSqrtPrice.String()),
				Liquidity: u128.GenUint128FromString(l1.Floor().String()),
			},
		},
	}
}

func getLockedVestingParams(
	totalLockedVestingAmount int64,
	numberOfVestingPeriod int64,
	cliffUnlockAmount int64,
	totalVestingDuration int64,
	cliffDurationFromMigrationTime int64,
	tokenBaseDecimal TokenDecimal,
) (LockedVesting, error) {
	if totalLockedVestingAmount == 0 {
		return LockedVesting{
			AmountPerPeriod:                0,
			CliffDurationFromMigrationTime: 0,
			Frequency:                      0,
			NumberOfPeriod:                 0,
			CliffUnlockAmount:              0,
		}, nil
	}

	if totalLockedVestingAmount == cliffUnlockAmount {
		return LockedVesting{
			AmountPerPeriod:                convertToLamports(N1, tokenBaseDecimal).BigInt().Uint64(),
			CliffDurationFromMigrationTime: uint64(cliffDurationFromMigrationTime),
			Frequency:                      1,
			NumberOfPeriod:                 1,
			CliffUnlockAmount:              convertToLamports(decimal.NewFromInt(totalLockedVestingAmount-1), tokenBaseDecimal).BigInt().Uint64(),
		}, nil
	}

	if numberOfVestingPeriod <= 0 {
		return LockedVesting{}, fmt.Errorf("total periods must be greater than zero")
	}

	if totalVestingDuration <= 0 {
		return LockedVesting{}, fmt.Errorf("total vesting duration must be greater than zero")
	}

	if cliffUnlockAmount > totalLockedVestingAmount {
		return LockedVesting{}, fmt.Errorf("cliff unlock amount cannot be greater than total locked vesting amount")
	}

	// amount_per_period = (total_locked_vesting_amount - cliff_unlock_amount) / number_of_period
	amountPerPeriod := (totalLockedVestingAmount - cliffUnlockAmount) / numberOfVestingPeriod
	roundedAmountPerPeriod := amountPerPeriod

	totalPeriodicAmount := roundedAmountPerPeriod * numberOfVestingPeriod
	remainder := totalLockedVestingAmount - (cliffUnlockAmount + totalPeriodicAmount)
	adjustedCliffUnlockAmount := cliffUnlockAmount + remainder

	periodFrequency := totalVestingDuration / numberOfVestingPeriod

	return LockedVesting{
		AmountPerPeriod:                convertToLamports(decimal.NewFromInt(roundedAmountPerPeriod), tokenBaseDecimal).BigInt().Uint64(),
		CliffDurationFromMigrationTime: uint64(cliffDurationFromMigrationTime),
		Frequency:                      uint64(periodFrequency),
		NumberOfPeriod:                 uint64(numberOfVestingPeriod),
		CliffUnlockAmount:              convertToLamports(decimal.NewFromInt(adjustedCliffUnlockAmount), tokenBaseDecimal).BigInt().Uint64(),
	}, nil
}

func getMigrationQuoteAmountFromMigrationQuoteThreshold(
	migrationQuoteThreshold decimal.Decimal,
	migrationFeePercent uint8,
) decimal.Decimal {
	return migrationQuoteThreshold.Mul(N100.Sub(decimal.NewFromUint64(uint64(migrationFeePercent)))).Div(N100)
}

func getMigrationQuoteThresholdFromMigrationQuoteAmount(
	migrationQuoteAmount decimal.Decimal,
	migrationFeePercent uint8,
) decimal.Decimal {
	return migrationQuoteAmount.
		Mul(N100).
		Div(N100.Sub(decimal.NewFromUint64(uint64(migrationFeePercent))))
}

func getBaseFeeParams(baseFeeParams BaseFeeParams, tokenQuoteDecimal TokenDecimal, activationType ActivationType) (BaseFeeParameters, error) {
	if baseFeeParams.BaseFeeMode == BaseFeeModeRateLimiter {
		if baseFeeParams.RateLimiterParam == nil {
			return BaseFeeParameters{}, fmt.Errorf("rate limiter parameters are required for RateLimiter mode")
		}
		r := baseFeeParams.RateLimiterParam
		return getRateLimiterParams(r.BaseFeeBps, r.FeeIncrementBps, r.ReferenceAmount, r.MaxLimiterDuration, tokenQuoteDecimal, activationType)
	} else {
		if baseFeeParams.FeeSchedulerParam == nil {
			panic("Fee scheduler parameters are required for FeeScheduler mode")
		}
		f := baseFeeParams.FeeSchedulerParam
		return getFeeSchedulerParams(f.StartingFeeBps, f.EndingFeeBps, baseFeeParams.BaseFeeMode, f.NumberOfPeriod, f.TotalDuration)
	}
}

func getRateLimiterParams(
	baseFeeBps int64,
	feeIncrementBps int64,
	referenceAmount int,
	maxLimiterDuration int,
	tokenQuoteDecimal TokenDecimal,
	activationType ActivationType,
) (BaseFeeParameters, error) {

	cliffFeeNumerator := bpsToFeeNumerator(baseFeeBps)
	feeIncrementNumerator := bpsToFeeNumerator(feeIncrementBps)

	if baseFeeBps <= 0 || feeIncrementBps <= 0 || referenceAmount <= 0 || maxLimiterDuration <= 0 {
		panic("All rate limiter parameters must be greater than zero")
	}
	if baseFeeBps > MAX_FEE_BPS {
		panic(fmt.Sprintf("Base fee (%d bps) exceeds maximum allowed value of %d bps", baseFeeBps, MAX_FEE_BPS))
	}
	if feeIncrementBps > MAX_FEE_BPS {
		panic(fmt.Sprintf("Fee increment (%d bps) exceeds maximum allowed value of %d bps", feeIncrementBps, MAX_FEE_BPS))
	}
	if feeIncrementNumerator.Cmp(FEE_DENOMINATOR) >= 0 {
		panic("Fee increment numerator must be less than FEE_DENOMINATOR")
	}

	deltaNumerator := MAX_FEE_NUMERATOR.Sub(cliffFeeNumerator)
	maxIndex := deltaNumerator.Div(feeIncrementNumerator)
	if maxIndex.Cmp(N1) < 0 {
		panic("Fee increment is too large for the given base fee")
	}

	if cliffFeeNumerator.Cmp(MIN_FEE_NUMERATOR) < 0 ||
		cliffFeeNumerator.Cmp(MAX_FEE_NUMERATOR) > 0 {
		panic("Base fee must be between 0.01% and 99%")
	}

	maxDuration := 0
	if activationType == ActivationTypeSlot {
		maxDuration = MAX_RATE_LIMITER_DURATION_IN_SLOTS
	} else {
		maxDuration = MAX_RATE_LIMITER_DURATION_IN_SECONDS
	}

	if maxLimiterDuration > maxDuration {
		panic(fmt.Sprintf("Max duration exceeds maximum allowed value of %d", maxDuration))
	}

	referenceAmountInLamports := convertToLamports(decimal.NewFromInt(int64(referenceAmount)), tokenQuoteDecimal)

	return BaseFeeParameters{
		CliffFeeNumerator: cliffFeeNumerator.BigInt().Uint64(),
		FirstFactor:       uint16(feeIncrementBps),
		SecondFactor:      uint64(maxLimiterDuration),
		ThirdFactor:       referenceAmountInLamports.BigInt().Uint64(),
		BaseFeeMode:       BaseFeeModeRateLimiter,
	}, nil
}

func getFeeSchedulerParams(
	startingBaseFeeBps int64,
	endingBaseFeeBps int64,
	baseFeeMode BaseFeeMode,
	numberOfPeriod uint16,
	totalDuration uint16,
) (BaseFeeParameters, error) {

	if startingBaseFeeBps == endingBaseFeeBps {
		if numberOfPeriod != 0 || totalDuration != 0 {
			return BaseFeeParameters{}, fmt.Errorf("numberOfPeriod and totalDuration must both be zero")
		}
		return BaseFeeParameters{
			CliffFeeNumerator: bpsToFeeNumerator(startingBaseFeeBps).BigInt().Uint64(),
			FirstFactor:       0,
			SecondFactor:      0,
			ThirdFactor:       0,
			BaseFeeMode:       BaseFeeModeFeeSchedulerLinear,
		}, nil
	}

	if numberOfPeriod <= 0 {
		panic("Total periods must be greater than zero")
	}

	if startingBaseFeeBps > MAX_FEE_BPS {
		panic(fmt.Sprintf("startingBaseFeeBps (%d bps) exceeds maximum allowed value of %d bps", startingBaseFeeBps, MAX_FEE_BPS))
	}

	if endingBaseFeeBps > startingBaseFeeBps {
		panic("endingBaseFeeBps bps must be less than or equal to startingBaseFeeBps bps")
	}

	if numberOfPeriod == 0 || totalDuration == 0 {
		panic("numberOfPeriod and totalDuration must both greater than zero")
	}

	maxBaseFeeNumerator := bpsToFeeNumerator(startingBaseFeeBps)
	minBaseFeeNumerator := bpsToFeeNumerator(endingBaseFeeBps)
	periodFrequency := uint64(totalDuration / numberOfPeriod)

	var reductionFactor decimal.Decimal
	if baseFeeMode == BaseFeeModeFeeSchedulerLinear {
		reductionFactor = maxBaseFeeNumerator.Sub(minBaseFeeNumerator).Div(decimal.NewFromUint64(uint64(numberOfPeriod)))
	} else {
		decayBase := dmath.Pow(minBaseFeeNumerator.Div(maxBaseFeeNumerator), N1.DivRound(decimal.NewFromUint64(uint64(numberOfPeriod)), 18), 18)
		reductionFactor = BASIS_POINT_MAX.Mul(N1.Sub(decayBase))
	}

	return BaseFeeParameters{
		CliffFeeNumerator: maxBaseFeeNumerator.BigInt().Uint64(),
		FirstFactor:       numberOfPeriod,
		SecondFactor:      periodFrequency,
		ThirdFactor:       reductionFactor.BigInt().Uint64(),
		BaseFeeMode:       baseFeeMode,
	}, nil
}

func getMigratedPoolFeeParams(
	migrationOption MigrationOption,
	migrationFeeOption MigrationFeeOption,
	migratedPoolFee *MigratedPoolFee,
) MigratedPoolFee {

	defaultFeeParams := MigratedPoolFee{
		CollectFeeMode: 0,
		DynamicFee:     0,
		PoolFeeBps:     0,
	}

	if migrationOption == MigrationOptionMETDAMM {
		return defaultFeeParams
	}

	if migrationOption == MigrationOptionMETDAMMV2 {
		if migrationFeeOption == MigrationFeeCustomizable && migratedPoolFee != nil {
			return *migratedPoolFee
		}

		return defaultFeeParams
	}

	return defaultFeeParams
}

// getTotalSupplyFromCurve
func getTotalSupplyFromCurve(
	migrationQuoteThreshold decimal.Decimal,
	sqrtStartPrice decimal.Decimal,
	curve []LiquidityDistributionParameters,
	lockedVesting *LockedVesting,
	migrationOption MigrationOption,
	leftover decimal.Decimal,
	migrationFeePercent uint8,
) (decimal.Decimal, error) {

	sqrtMigrationPrice, err := getMigrationThresholdPrice(migrationQuoteThreshold, sqrtStartPrice, curve)
	if err != nil {
		return N0, err
	}

	swapBaseAmount, err := getBaseTokenForSwap(sqrtStartPrice, sqrtMigrationPrice, curve)
	if err != nil {
		return N0, err
	}

	swapBaseAmountBuffer, err := getSwapAmountWithBuffer(swapBaseAmount, sqrtStartPrice, curve)
	if err != nil {
		return N0, err
	}

	migrationQuoteAmount := getMigrationQuoteAmountFromMigrationQuoteThreshold(migrationQuoteThreshold, migrationFeePercent)

	migrationBaseAmount, err := getMigrationBaseToken(convertDecimalToBN(migrationQuoteAmount), sqrtMigrationPrice, migrationOption)
	if err != nil {
		return N0, err
	}

	totalVestingAmount := getTotalVestingAmount(lockedVesting)

	minimumBaseSupplyWithBuffer := swapBaseAmountBuffer.Add(migrationBaseAmount).Add(totalVestingAmount).Add(leftover)

	return minimumBaseSupplyWithBuffer, nil
}

// getMigrationThresholdPrice
func getMigrationThresholdPrice(
	migrationThreshold decimal.Decimal,
	sqrtStartPrice decimal.Decimal,
	curve []LiquidityDistributionParameters,
) (decimal.Decimal, error) {
	nextSqrtPrice := sqrtStartPrice

	if len(curve) == 0 {
		return N0, errors.New("curve is empty")
	}

	totalAmount := getDeltaAmountQuoteUnsigned(nextSqrtPrice, decimal.NewFromBigInt(curve[0].SqrtPrice.BigInt(), 0), decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0), true)

	if totalAmount.GreaterThan(migrationThreshold) {

		var err error
		nextSqrtPrice, err = getNextSqrtPriceFromInput(nextSqrtPrice, decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0), migrationThreshold, false)
		if err != nil {
			return N0, err
		}

	} else {

		amountLeft := migrationThreshold.Sub(totalAmount)
		nextSqrtPrice = decimal.NewFromBigInt(curve[0].SqrtPrice.BigInt(), 0)

		for i := 1; i < len(curve); i++ {
			maxAmount := getDeltaAmountQuoteUnsigned(nextSqrtPrice, decimal.NewFromBigInt(curve[i].SqrtPrice.BigInt(), 0), decimal.NewFromBigInt(curve[i].Liquidity.BigInt(), 0), true)

			if maxAmount.Cmp(amountLeft) > 0 {
				var err error
				nextSqrtPrice, err = getNextSqrtPriceFromInput(nextSqrtPrice, decimal.NewFromBigInt(curve[i].Liquidity.BigInt(), 0), amountLeft, false)
				if err != nil {
					return N0, err
				}
				amountLeft = N0
				break
			} else {
				amountLeft = amountLeft.Sub(maxAmount)
				nextSqrtPrice = decimal.NewFromBigInt(curve[i].SqrtPrice.BigInt(), 0)
			}
		}

		if !amountLeft.IsZero() {
			return N0, errors.New("Not enough liquidity, migrationThreshold: " + migrationThreshold.String() + "  amountLeft: " + amountLeft.String())
		}
	}

	return nextSqrtPrice, nil
}
