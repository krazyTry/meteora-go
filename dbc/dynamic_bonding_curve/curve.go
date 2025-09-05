package dynamic_bonding_curve

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/krazyTry/meteora-go/u128"

	"github.com/shopspring/decimal"
)

// nth uses the Newton-Raphson algorithm to calculate the y-th root
// Returns a decimal.Decimal, keeping scale decimal places
func nth(x decimal.Decimal, y float64, scale int32) (decimal.Decimal, error) {

	n := int64(1.0 / y)

	if n <= 0 {
		return decimal.Decimal{}, errors.New("n must be positive")
	}
	if x.IsNegative() && n%2 == 0 {
		return decimal.Decimal{}, errors.New("cannot take even root of negative number")
	}
	if x.IsZero() {
		return decimal.Zero, nil
	}

	f, _ := x.Float64()
	initGuess := decimal.NewFromFloat(math.Pow(f, 1/float64(n)))

	guess := initGuess
	last := decimal.Zero

	maxIter := 200
	epsilon := decimal.New(1, -int32(scale)) // 10^-scale

	for i := 0; i < maxIter; i++ {
		// guess = ((n-1)*guess + x/(guess^(n-1))) / n
		guessPow := guess.Pow(decimal.NewFromInt(n - 1))
		if guessPow.IsZero() {
			return decimal.Decimal{}, errors.New("division by zero in iteration")
		}

		term1 := guess.Mul(decimal.NewFromInt(n - 1))
		term2 := x.Div(guessPow)
		next := term1.Add(term2).Div(decimal.NewFromInt(n))

		if next.Sub(guess).Abs().LessThan(epsilon) {
			return next.Round(int32(scale)), nil
		}
		last = guess
		guess = next
	}

	return last.Round(int32(scale)), nil
}

func mulDiv(x, y, denominator decimal.Decimal, roundUp bool) (decimal.Decimal, error) {
	one := decimal.NewFromInt(1)

	if denominator.IsZero() {
		return decimal.Zero, errors.New("MulDivDecimal: division by zero")
	}

	if denominator.Equal(one) || x.IsZero() || y.IsZero() {
		return x.Mul(y), nil
	}

	prod := x.Mul(y)

	if roundUp {
		// (x*y + (denominator - 1)) / denominator
		return prod.Add(denominator.Sub(one)).Div(denominator).Floor(), nil
		// return prod.Add(denominator.Sub(one)).Div(denominator), nil
	} else {
		//  x*y / denominator
		// return prod.DivRound(denominator, 48), nil
		return prod.Div(denominator).Ceil(), nil
		// return prod.Div(denominator), nil
	}
}

func decimalSqrt(x decimal.Decimal) decimal.Decimal {
	if x.Sign() < 0 {
		panic("sqrt on negative decimal")
	}
	// f, _ := new(big.Float).SetString(x.String())
	// s := new(big.Float).Sqrt(f)
	s := new(big.Float).SetPrec(200).Sqrt(x.BigFloat().SetPrec(200))
	out, _ := decimal.NewFromString(s.Text('f', -1))
	return out
}

func truncateSig(d decimal.Decimal, sig int) decimal.Decimal {
	return d.Round(int32(sig - len(d.Coefficient().String())))
}

func getDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity decimal.Decimal, roundUp bool) (decimal.Decimal, error) {
	if liquidity.IsZero() {
		return decimal.Zero, nil
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

	// denominator = 1 << (RESOLUTION*2)
	denominator := decimal.NewFromInt(2).Pow(decimal.NewFromInt(int64(RESOLUTION * 2)))

	if roundUp {
		// numerator = prod + denominator - 1 (ceiling division)
		numerator := prod.Add(denominator).Sub(decimal.NewFromInt(1))
		return numerator.DivRound(denominator, 48).Floor()
	} else {
		return prod.DivRound(denominator, 48).Floor()
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

	// nextSqrtPrice = liquidity * sqrtPrice / denominator, 向上舍入
	return mulDiv(liquidity, sqrtPrice, denominator, true)
}

func getNextSqrtPriceFromAmountQuoteRoundingDown(sqrtPrice, liquidity, amount decimal.Decimal) decimal.Decimal {
	if amount.IsZero() {
		return sqrtPrice
	}

	shifted := amount.Mul(decimal.NewFromInt(2).Pow(decimal.NewFromInt(int64(RESOLUTION * 2))))

	quotient := shifted.Div(liquidity).Floor()

	// nextSqrtPrice = sqrtPrice + quotient
	return sqrtPrice.Add(quotient)
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
	if baseAmount.Equal(decimal.Zero) {
		return decimal.Zero
	}

	// priceDelta = sqrtMaxPrice - sqrtPrice
	priceDelta := sqrtMaxPrice.Sub(sqrtPrice)

	// prod = baseAmount * sqrtPrice * sqrtMaxPrice
	prod := baseAmount.Mul(sqrtPrice).Mul(sqrtMaxPrice)

	// liquidity = prod / priceDelta
	liquidity := prod.DivRound(priceDelta, 48).Floor()

	return liquidity
}

func convertDecimalToBN(value decimal.Decimal) decimal.Decimal {
	return value.Floor()
}

// ConvertToLamports
func convertToLamports(amount any, tokenDecimal TokenDecimal) decimal.Decimal {
	var amt decimal.Decimal

	switch v := amount.(type) {
	case string:
		amt, _ = decimal.NewFromString(v)
	case float64:
		amt = decimal.NewFromFloat(v)
	case int64:
		amt = decimal.NewFromInt(v)
	case int:
		amt = decimal.NewFromInt(int64(v))
	default:
		panic("unsupported amount type")
	}

	valueInLamports := amt.Mul(decimal.New(1, int32(tokenDecimal)))

	return convertDecimalToBN(valueInLamports)
}

func getSqrtPriceFromPrice(decimalPrice decimal.Decimal, tokenADecimal, tokenBDecimal TokenDecimal) decimal.Decimal {

	decimalsAdjustment := decimal.New(1, int32(tokenADecimal)-int32(tokenBDecimal)) // 10^(tokenADecimal - tokenBDecimal)

	adjusted := decimalPrice.DivRound(decimalsAdjustment, 25)

	sqrtAdjusted := decimalSqrt(adjusted)

	q64 := sqrtAdjusted.Mul(decimal.NewFromInt(2).Pow(decimal.NewFromInt(64)))

	return q64.Floor()
}

func getMigrationBaseToken(
	migrationQuoteAmount, sqrtMigrationPrice decimal.Decimal,
	migrationOption MigrationOption,
) (decimal.Decimal, error) {

	switch migrationOption {
	case MigrationOptionMETDAMM:
		// price = sqrtMigrationPrice^2
		price := sqrtMigrationPrice.Mul(sqrtMigrationPrice)

		// quote = migrationQuoteAmount << 128 -> migrationQuoteAmount * 2^128
		twoPow128 := decimal.NewFromInt(2).Pow(decimal.NewFromInt(128))
		quote := migrationQuoteAmount.Mul(twoPow128)

		// divmod: div = ceil(quote / price)
		div := quote.Div(price).Ceil()

		return div, nil
	case MigrationOptionMETDAMMV2:

		liquidity, err := getInitialLiquidityFromDeltaQuote(
			migrationQuoteAmount,
			decimal.NewFromBigInt(MIN_SQRT_PRICE, 0),
			sqrtMigrationPrice,
		)
		if err != nil {
			return decimal.Zero, err
		}

		baseAmount, err := getDeltaAmountBaseUnsigned(
			sqrtMigrationPrice,
			decimal.NewFromBigInt(MAX_SQRT_PRICE, 0),
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
		return decimal.Zero, errors.New("price delta cannot be zero")
	}

	// quoteAmountShifted = quoteAmount << 128 -> quoteAmount * 2^128
	quoteAmountShifted := quoteAmount.Mul(decimal.NewFromInt(2).Pow(decimal.NewFromInt(128)))

	// liquidity = quoteAmountShifted / priceDelta
	liquidity := quoteAmountShifted.DivRound(priceDelta, 48).Floor()
	return liquidity, nil
}

// getTotalVestingAmount
func getTotalVestingAmount(lockedVesting *LockedVesting) decimal.Decimal {
	// totalVestingAmount = cliffUnlockAmount + amountPerPeriod * numberOfPeriod
	amountPerPeriod := decimal.NewFromUint64(lockedVesting.AmountPerPeriod)
	numberOfPeriod := decimal.NewFromUint64(lockedVesting.NumberOfPeriod)
	cliffUnlockAmount := decimal.NewFromUint64(lockedVesting.CliffUnlockAmount)

	totalVesting := amountPerPeriod.Mul(numberOfPeriod).Add(cliffUnlockAmount)
	return totalVesting
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
	denominator := swapAmount.Mul(decimal.NewFromInt(100).Sub(migrationFeePercentDecimal)).Div(decimal.NewFromInt(100))

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
	buffer := swapBaseAmount.Mul(decimal.NewFromFloat(0.25))
	swapAmountBuffer := swapBaseAmount.Add(buffer)

	// maxBaseAmountOnCurve = getBaseTokenForSwap(...)
	maxBaseAmountOnCurve, err := getBaseTokenForSwap(sqrtStartPrice, decimal.NewFromBigInt(MAX_SQRT_PRICE, 0), curve)
	if err != nil {
		return decimal.Zero, err
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

	sqrtRatio := decimalSqrt(initialMarketCap.DivRound(migrationMarketCap, 21)).Truncate(21)

	totalVestingAmount := getTotalVestingAmount(lockedVesting)

	vestingPercentage := totalVestingAmount.Mul(decimal.NewFromInt(100)).Div(totalTokenSupply)

	leftoverPercentage := totalLeftover.Mul(decimal.NewFromInt(100)).Div(totalTokenSupply)

	numerator := decimal.NewFromInt(100).Mul(sqrtRatio).Sub(vestingPercentage.Add(leftoverPercentage).Mul(sqrtRatio)).Round(18)

	denominator := decimal.NewFromInt(1).Add(sqrtRatio).Round(18)

	return numerator.DivRound(denominator, 13)
}

// GetMigrationQuoteAmount
func getMigrationQuoteAmount(migrationMarketCap decimal.Decimal, percentageSupplyOnMigration decimal.Decimal) decimal.Decimal {
	return migrationMarketCap.Mul(percentageSupplyOnMigration).DivRound(decimal.NewFromInt(100), 18)
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
		Div(decimal.NewFromInt(BASIS_POINT_MAX.Int64())).
		Add(decimal.NewFromInt(1))

	// Q64: sqrt(priceRatio) * 2^64
	sqrtPriceRatioQ64 := decimalSqrt(priceRatio).Round(19).Mul(decimal.NewFromInt(2).Pow(decimal.NewFromInt(64))).Floor()
	// 2️⃣ deltaBinId = (sqrtPriceRatioQ64 - ONE_Q64) / BIN_STEP_BPS_U128_DEFAULT * 2
	deltaBinId := sqrtPriceRatioQ64.Sub(decimal.NewFromBigInt(ONE_Q64, 0)).Div(decimal.NewFromBigInt(BIN_STEP_BPS_U128_DEFAULT.BigInt(), 0)).Floor().Mul(decimal.NewFromInt(2))
	// 3️⃣ maxVolatilityAccumulator = deltaBinId * BASIS_POINT_MAX
	maxVolatilityAccumulator := deltaBinId.Mul(decimal.NewFromBigInt(BASIS_POINT_MAX, 0))
	// 4️⃣ squareVfaBin = (maxVolatilityAccumulator * BIN_STEP_BPS_DEFAULT)^2
	squareVfaBin := maxVolatilityAccumulator.Mul(decimal.NewFromBigInt(BIN_STEP_BPS_DEFAULT, 0)).Pow(decimal.NewFromInt(2))
	// baseFeeNumerator
	baseFeeNumerator := bpsToFeeNumerator(baseFeeBps)

	// maxDynamicFeeNumerator = baseFeeNumerator * 20 / 100
	maxDynamicFeeNumerator := baseFeeNumerator.Mul(decimal.NewFromInt(20)).Div(decimal.NewFromInt(100))

	// vFee = maxDynamicFeeNumerator * 100_000_000_000 - 99_999_999_999
	vFee := maxDynamicFeeNumerator.Mul(decimal.NewFromInt(100_000_000_000)).Sub(decimal.NewFromInt(99_999_999_999))

	// variableFeeControl = vFee / squareVfaBin
	variableFeeControl := vFee.Div(squareVfaBin).Floor()

	return &DynamicFeeParameters{
		BinStep:                  uint16(BIN_STEP_BPS_DEFAULT.Int64()),
		BinStepU128:              BIN_STEP_BPS_U128_DEFAULT,
		FilterPeriod:             DYNAMIC_FEE_FILTER_PERIOD_DEFAULT,
		DecayPeriod:              DYNAMIC_FEE_DECAY_PERIOD_DEFAULT,
		ReductionFactor:          DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT,
		MaxVolatilityAccumulator: uint32(maxVolatilityAccumulator.BigInt().Uint64()),
		VariableFeeControl:       uint32(variableFeeControl.BigInt().Uint64()),
	}
}

// bpsToFeeNumerator
func bpsToFeeNumerator(bps int64) decimal.Decimal {
	return decimal.NewFromInt(bps).Mul(decimal.NewFromBigInt(FEE_DENOMINATOR, 0)).Div(decimal.NewFromBigInt(BASIS_POINT_MAX, 0))
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
	a1 := decimal.NewFromInt(1).DivRound(p0, 36).Sub(decimal.NewFromInt(1).DivRound(p1, 37))

	// b1 = 1/p1 - 1/p2
	b1 := decimal.NewFromInt(1).DivRound(p1, 37).Sub(decimal.NewFromInt(1).DivRound(p2, 37))

	c1 := swapAmount

	// a2 = p1 - p0
	a2 := p1.Sub(p0)

	// b2 = p2 - p1
	b2 := p2.Sub(p1)

	c2 := truncateSig(migrationQuoteThreshold.Mul(truncateSig(decimal.NewFromInt(2).Pow(decimal.NewFromInt(128)), 20)), 20)

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
			AmountPerPeriod:                convertToLamports(1, tokenBaseDecimal).BigInt().Uint64(),
			CliffDurationFromMigrationTime: big.NewInt(cliffDurationFromMigrationTime).Uint64(),
			Frequency:                      1,
			NumberOfPeriod:                 1,
			CliffUnlockAmount:              convertToLamports(totalLockedVestingAmount-1, tokenBaseDecimal).BigInt().Uint64(),
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
		AmountPerPeriod:                convertToLamports(roundedAmountPerPeriod, tokenBaseDecimal).BigInt().Uint64(),
		CliffDurationFromMigrationTime: uint64(cliffDurationFromMigrationTime),
		Frequency:                      uint64(periodFrequency),
		NumberOfPeriod:                 uint64(numberOfVestingPeriod),
		CliffUnlockAmount:              convertToLamports(adjustedCliffUnlockAmount, tokenBaseDecimal).BigInt().Uint64(),
	}, nil
}

func getMigrationQuoteAmountFromMigrationQuoteThreshold(
	migrationQuoteThreshold decimal.Decimal,
	migrationFeePercent uint8,
) decimal.Decimal {
	migrationQuoteAmount := migrationQuoteThreshold.
		Mul(decimal.NewFromInt(100).Sub(decimal.NewFromUint64(uint64(migrationFeePercent)))).
		Div(decimal.NewFromInt(100))
	return migrationQuoteAmount
}

func getMigrationQuoteThresholdFromMigrationQuoteAmount(
	migrationQuoteAmount decimal.Decimal,
	migrationFeePercent uint8,
) decimal.Decimal {
	migrationQuoteThreshold := migrationQuoteAmount.
		Mul(decimal.NewFromInt(100)).
		Div(decimal.NewFromInt(100).Sub(decimal.NewFromUint64(uint64(migrationFeePercent))))

	return migrationQuoteThreshold
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
	if feeIncrementNumerator.Cmp(decimal.NewFromBigInt(FEE_DENOMINATOR, 0)) >= 0 {
		panic("Fee increment numerator must be less than FEE_DENOMINATOR")
	}

	deltaNumerator := decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0).Sub(cliffFeeNumerator)
	maxIndex := deltaNumerator.Div(feeIncrementNumerator)
	if maxIndex.Cmp(decimal.NewFromInt(1)) < 0 {
		panic("Fee increment is too large for the given base fee")
	}

	if cliffFeeNumerator.Cmp(decimal.NewFromBigInt(MIN_FEE_NUMERATOR, 0)) < 0 ||
		cliffFeeNumerator.Cmp(decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0)) > 0 {
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
		// FeeSchedulerExponential
		ratio := minBaseFeeNumerator.InexactFloat64() / maxBaseFeeNumerator.InexactFloat64()
		decayBase := math.Pow(ratio, 1.0/float64(numberOfPeriod))
		reductionFactor = decimal.NewFromInt(int64(float64(BASIS_POINT_MAX.Int64()) * (1 - decayBase)))
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
		return decimal.Zero, err
	}

	swapBaseAmount, err := getBaseTokenForSwap(sqrtStartPrice, sqrtMigrationPrice, curve)
	if err != nil {
		return decimal.Zero, err
	}

	swapBaseAmountBuffer, err := getSwapAmountWithBuffer(swapBaseAmount, sqrtStartPrice, curve)
	if err != nil {
		return decimal.Zero, err
	}

	migrationQuoteAmount := getMigrationQuoteAmountFromMigrationQuoteThreshold(migrationQuoteThreshold, migrationFeePercent)

	migrationBaseAmount, err := getMigrationBaseToken(convertDecimalToBN(migrationQuoteAmount), sqrtMigrationPrice, migrationOption)
	if err != nil {
		return decimal.Zero, err
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
		return decimal.Zero, errors.New("curve is empty")
	}

	totalAmount := getDeltaAmountQuoteUnsigned(nextSqrtPrice, decimal.NewFromBigInt(curve[0].SqrtPrice.BigInt(), 0), decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0), true)

	if totalAmount.GreaterThan(migrationThreshold) {

		var err error
		nextSqrtPrice, err = getNextSqrtPriceFromInput(nextSqrtPrice, decimal.NewFromBigInt(curve[0].Liquidity.BigInt(), 0), migrationThreshold, false)
		if err != nil {
			return decimal.Zero, err
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
					return decimal.Zero, err
				}
				amountLeft = decimal.Zero
				break
			} else {
				amountLeft = amountLeft.Sub(maxAmount)
				nextSqrtPrice = decimal.NewFromBigInt(curve[i].SqrtPrice.BigInt(), 0)
			}
		}

		if !amountLeft.IsZero() {
			return decimal.Zero, errors.New("Not enough liquidity, migrationThreshold: " + migrationThreshold.String() + "  amountLeft: " + amountLeft.String())
		}
	}

	return nextSqrtPrice, nil
}
