package cp_amm

import (
	"errors"
	"fmt"
	"math/big"

	dmath "github.com/krazyTry/meteora-go/decimal_math"
	"github.com/krazyTry/meteora-go/u128"

	"github.com/shopspring/decimal"
)

// getBaseFeeNumerator
func getBaseFeeNumerator(
	feeSchedulerMode FeeSchedulerMode,
	cliffFeeNumerator decimal.Decimal,
	period decimal.Decimal,
	reductionFactor decimal.Decimal,
) decimal.Decimal {

	var feeNumerator decimal.Decimal

	if feeSchedulerMode == FeeSchedulerModeLinear {
		feeNumerator = cliffFeeNumerator.Sub(period.Mul(reductionFactor))
	} else { // Exponential
		// bps = (reductionFactor << SCALE_OFFSET) / BASIS_POINT_MAX
		// bps := decimal.NewFromBigInt(new(big.Int).Lsh(reductionFactor.BigInt(), SCALE_OFFSET), 0).Div(decimal.NewFromBigInt(BASIS_POINT_MAX, 0)).Floor()
		bps := dmath.Lsh(reductionFactor, 64).Div(BASIS_POINT_MAX).Floor()

		// base = ONE - bps
		base := N1.Sub(bps)

		result := pow(base, period)

		// feeNumerator = (cliffFeeNumerator * result) >> SCALE_OFFSET
		// feeNumerator = decimal.NewFromBigInt(new(big.Int).Rsh(cliffFeeNumerator.Mul(result).BigInt(), SCALE_OFFSET), 0)
		feeNumerator = dmath.Rsh(cliffFeeNumerator.Mul(result), 64)
	}

	return feeNumerator
}

// getDynamicFeeNumerator calculates the dynamic fee numerator based on market volatility
func getDynamicFeeNumerator(
	volatilityAccumulator decimal.Decimal,
	binStep decimal.Decimal,
	variableFeeControl decimal.Decimal,
) decimal.Decimal {
	if variableFeeControl.IsZero() {
		return N0
	}

	// squareVfaBin = (volatilityAccumulator * binStep)^2
	squareVfaBin := volatilityAccumulator.Mul(binStep)
	squareVfaBin = squareVfaBin.Mul(squareVfaBin)

	// vFee = variableFeeControl * squareVfaBin
	vFee := variableFeeControl.Mul(squareVfaBin)

	// vFee + 99_999_999_999
	vFee = vFee.Add(N99_999_999_999)

	// divide by 100_000_000_000
	result := vFee.Div(N100_000_000_000)

	return result
}

// DynamicFeeParams represents parameters for dynamic fee calculation based on market volatility
type DynamicFeeParams struct {
	// VolatilityAccumulator tracks the accumulated volatility over time
	VolatilityAccumulator *big.Int
	// BinStep represents the step size for price bins in the dynamic fee calculation
	BinStep               *big.Int
	// VariableFeeControl controls the variable fee adjustment mechanism
	VariableFeeControl    *big.Int
}

// GetFeeNumerator
func GetFeeNumerator(
	currentPoint decimal.Decimal,
	activationPoint decimal.Decimal,
	numberOfPeriod decimal.Decimal,
	periodFrequency decimal.Decimal,
	feeSchedulerMode FeeSchedulerMode,
	cliffFeeNumerator decimal.Decimal,
	reductionFactor decimal.Decimal,
	dynamicFeeParams *DynamicFeeParams,
) decimal.Decimal {

	if periodFrequency.IsZero() || currentPoint.Cmp(activationPoint) < 0 {
		return cliffFeeNumerator
	}

	// period = min(numberOfPeriod, (currentPoint - activationPoint)/periodFrequency)

	diff := currentPoint.Sub(activationPoint)
	period := diff.Div(periodFrequency).Floor() // new(big.Int).Div(diff, periodFrequency)

	maxPeriod := numberOfPeriod

	if period.Cmp(maxPeriod) > 0 {
		period = maxPeriod
	}

	// feeNumerator = getBaseFeeNumerator(...)
	feeNumerator := getBaseFeeNumerator(feeSchedulerMode, cliffFeeNumerator, period, reductionFactor)

	if dynamicFeeParams != nil {
		dynamicFeeNumerator := getDynamicFeeNumerator(
			decimal.NewFromBigInt(dynamicFeeParams.VolatilityAccumulator, 0),
			decimal.NewFromBigInt(dynamicFeeParams.BinStep, 0),
			decimal.NewFromBigInt(dynamicFeeParams.VariableFeeControl, 0),
		)
		feeNumerator = feeNumerator.Add(dynamicFeeNumerator.Floor())
	}

	if feeNumerator.Cmp(MAX_FEE_NUMERATOR) > 0 {
		feeNumerator = MAX_FEE_NUMERATOR
	}

	return feeNumerator
}

// FeeMode represents the fee collection mode for swap operations
type FeeMode struct {
	// FeeOnInput indicates whether fees are collected on the input token
	FeeOnInput   bool
	// FeesOnTokenA indicates whether fees are collected on token A
	FeesOnTokenA bool
}

// GetFeeMode
func GetFeeMode(collectFeeMode CollectFeeMode, btoA bool) FeeMode {
	feeOnInput := btoA && collectFeeMode == CollectFeeModeOnlyB
	feesOnTokenA := btoA && collectFeeMode == CollectFeeModeBothToken

	return FeeMode{
		FeeOnInput:   feeOnInput,
		FeesOnTokenA: feesOnTokenA,
	}
}

// GetSwapAmount
func GetSwapAmount(
	actualInAmount, sqrtPrice, liquidity, tradeFeeNumerator decimal.Decimal,
	swapBaseForQuote bool,
	collectFeeMode CollectFeeMode,
) (decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {

	feeMode := GetFeeMode(collectFeeMode, !swapBaseForQuote)
	// actualInAmount := decimal.NewFromBigInt(inAmount, 0) //new(big.Int).Set(inAmount)
	totalFee := decimal.Zero

	if feeMode.FeeOnInput {
		fee, err := mulDiv(actualInAmount, tradeFeeNumerator, FEE_DENOMINATOR, true)
		if err != nil {
			return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
		}
		totalFee = fee
		actualInAmount = actualInAmount.Sub(totalFee)

	}

	nextSqrtPrice := getNextSqrtPrice(actualInAmount, sqrtPrice, liquidity, swapBaseForQuote)

	var outAmount decimal.Decimal
	if swapBaseForQuote {
		outAmount = GetAmountBFromLiquidityDelta(liquidity, sqrtPrice, nextSqrtPrice, false)
	} else {
		outAmount = GetAmountAFromLiquidityDelta(liquidity, sqrtPrice, nextSqrtPrice, false)
	}

	var amountOut decimal.Decimal
	if feeMode.FeeOnInput {
		amountOut = outAmount
	} else {
		fee, err := mulDiv(outAmount, tradeFeeNumerator, FEE_DENOMINATOR, true)
		if err != nil {
			return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{}, err
		}
		totalFee = fee
		amountOut = outAmount.Sub(totalFee) //new(big.Int).Sub(outAmount, totalFee.BigInt())
	}
	return amountOut, totalFee, nextSqrtPrice, nil

}

// GetPriceImpact
// abs(execution_price - spot_price) / spot_price * 100%
func GetPriceImpact(
	amountIn, amountOut, currentSqrtPrice decimal.Decimal,
	aToB bool,
	tokenADecimal, tokenBDecimal uint8,
) (decimal.Decimal, error) {

	if amountIn.IsZero() {
		return N0, nil
	}
	if amountOut.IsZero() {
		return decimal.Decimal{}, errors.New("amountOut must be greater than 0")
	}

	spotPrice := getPriceFromSqrtPrice(currentSqrtPrice, tokenADecimal, tokenBDecimal)

	// 1.0526315900277009477
	executionPrice := amountIn.DivRound(amountOut, 19) // new(big.Float).Quo(amountInF, amountOutF)

	expDiff := int32(tokenBDecimal) - int32(tokenADecimal)
	if !aToB {
		expDiff = int32(tokenADecimal) - int32(tokenBDecimal)
	}
	adjustFactor := decimal.New(1, int32(expDiff)) // 10^expDiff

	executionPrice = executionPrice.Mul(adjustFactor).Round(13)

	var actualExecutionPrice decimal.Decimal
	if aToB {
		// 0.94999998999999999996
		actualExecutionPrice = N1.DivRound(executionPrice, 20)
	} else {
		actualExecutionPrice = executionPrice
	}
	// priceImpact = |execution_price - spot_price| / spot_price * 100
	diff := actualExecutionPrice.Sub(spotPrice).Abs()
	priceImpact := diff.Div(spotPrice).Mul(N100)

	return priceImpact, nil
}

// RewardResult represents the result of reward and fee calculations
type RewardResult struct {
	// FeeTokenA is the fee amount for token A
	FeeTokenA *big.Int
	// FeeTokenB is the fee amount for token B
	FeeTokenB *big.Int
	// Rewards is an array of reward amounts for different reward tokens
	Rewards   []*big.Int
}

func reverseBytes(b []byte) []byte {
	n := len(b)
	out := make([]byte, n)
	for i := range b {
		out[n-1-i] = b[i]
	}
	return out
}

func bytesToBigIntLE(b []byte) *big.Int {
	return new(big.Int).SetBytes(reverseBytes(b))
}

func CalculateUnClaimFee(poolState *Pool, positionState *Position) (decimal.Decimal, decimal.Decimal) {

	// totalPositionLiquidity := new(big.Int).Add(positionState.UnlockedLiquidity.BigInt(),
	// new(big.Int).Add(positionState.VestedLiquidity.BigInt(), positionState.PermanentLockedLiquidity.BigInt()))

	totalPositionLiquidity := decimal.NewFromBigInt(positionState.UnlockedLiquidity.BigInt(), 0).Add(
		decimal.NewFromBigInt(positionState.VestedLiquidity.BigInt(), 0).Add(
			decimal.NewFromBigInt(positionState.PermanentLockedLiquidity.BigInt(), 0),
		),
	)

	// feeAPerTokenStored := new(big.Int).Sub(
	// 	bytesToBigIntLE(poolState.FeeAPerLiquidity[:]),
	// 	bytesToBigIntLE(positionState.FeeAPerTokenCheckpoint[:]),
	// )
	feeAPerTokenStored := decimal.NewFromBigInt(bytesToBigIntLE(poolState.FeeAPerLiquidity[:]), 0).Sub(
		decimal.NewFromBigInt(bytesToBigIntLE(positionState.FeeAPerTokenCheckpoint[:]), 0),
	)

	// feeBPerTokenStored := new(big.Int).Sub(
	// 	bytesToBigIntLE(poolState.FeeBPerLiquidity[:]),
	// 	bytesToBigIntLE(positionState.FeeBPerTokenCheckpoint[:]),
	// )

	feeBPerTokenStored := decimal.NewFromBigInt(bytesToBigIntLE(poolState.FeeBPerLiquidity[:]), 0).Sub(
		decimal.NewFromBigInt(bytesToBigIntLE(positionState.FeeBPerTokenCheckpoint[:]), 0),
	)

	// feeA := new(big.Int).Rsh(new(big.Int).Mul(totalPositionLiquidity, feeAPerTokenStored), LIQUIDITY_SCALE)
	// feeB := new(big.Int).Rsh(new(big.Int).Mul(totalPositionLiquidity, feeBPerTokenStored), LIQUIDITY_SCALE)

	feeA := decimal.NewFromBigInt(new(big.Int).Rsh(totalPositionLiquidity.Mul(feeAPerTokenStored).BigInt(), 128), 0)
	feeB := decimal.NewFromBigInt(new(big.Int).Rsh(totalPositionLiquidity.Mul(feeBPerTokenStored).BigInt(), 128), 0)

	// return new(big.Int).Add(new(big.Int).SetUint64(positionState.FeeAPending), feeA),
	// 	new(big.Int).Add(new(big.Int).SetUint64(positionState.FeeBPending), feeB)
	return decimal.NewFromUint64(positionState.FeeAPending).Add(feeA),
		decimal.NewFromUint64(positionState.FeeBPending).Add(feeB)
}

func CalculateUnClaimReward(poolState *Pool, positionState *Position) []decimal.Decimal {
	var rewards []decimal.Decimal
	for _, item := range positionState.RewardInfos {
		rewards = append(rewards, decimal.NewFromUint64(item.RewardPendings))
	}
	return rewards
}

// bpsToFeeNumerator converts basis points (bps) to a fee numerator.
// 1 bps = 0.01% = 0.0001 in decimal
// @param bps - The value in basis points [1-10_000]
// @returns The equivalent fee numerator
func bpsToFeeNumerator(bps int64) decimal.Decimal {
	bpsBig := decimal.NewFromInt(bps) //big.NewInt(bps)
	// numerator = bps * FEE_DENOMINATOR / BASIS_POINT_MAX
	numerator := bpsBig.Mul(FEE_DENOMINATOR)   //new(big.Int).Mul(bpsBig, FEE_DENOMINATOR)
	numerator = numerator.Div(BASIS_POINT_MAX) // numerator.Div(numerator, BASIS_POINT_MAX)
	return numerator
}

// feeNumeratorToBps converts a fee numerator back to basis points (bps).
// @param feeNumerator - The fee numerator to convert
// @returns The equivalent value in basis points [1-10_000]
func feeNumeratorToBps(feeNumerator decimal.Decimal) int64 {
	// bps = feeNumerator * BASIS_POINT_MAX / FEE_DENOMINATOR
	result := feeNumerator.Mul(BASIS_POINT_MAX)
	result.Div(FEE_DENOMINATOR)

	return result.BigInt().Int64()
}

// getBaseFeeParams
func GetBaseFeeParams(
	maxBaseFeeBps int64,
	minBaseFeeBps int64,
	feeSchedulerMode FeeSchedulerMode,
	numberOfPeriod int,
	totalDuration int64,
) (*BaseFeeParameters, error) {

	if maxBaseFeeBps == minBaseFeeBps {
		if numberOfPeriod != 0 || totalDuration != 0 {
			return nil, errors.New("numberOfPeriod and totalDuration must both be zero")
		}

		return &BaseFeeParameters{
			CliffFeeNumerator: bpsToFeeNumerator(maxBaseFeeBps).BigInt().Uint64(),
			NumberOfPeriod:    0,
			PeriodFrequency:   0,
			ReductionFactor:   0,
			FeeSchedulerMode:  0,
		}, nil
	}

	if numberOfPeriod <= 0 {
		return nil, errors.New("total periods must be greater than zero")
	}

	if maxBaseFeeBps > feeNumeratorToBps(MAX_FEE_NUMERATOR) {
		return nil, fmt.Errorf(
			"maxBaseFeeBps (%d bps) exceeds maximum allowed value of %d bps",
			maxBaseFeeBps,
			feeNumeratorToBps(MAX_FEE_NUMERATOR),
		)
	}

	if minBaseFeeBps > maxBaseFeeBps {
		return nil, errors.New("minBaseFee bps must be <= maxBaseFee bps")
	}

	if numberOfPeriod == 0 || totalDuration == 0 {
		return nil, errors.New("numberOfPeriod and totalDuration must both be greater than zero")
	}

	maxBaseFeeNumerator := bpsToFeeNumerator(maxBaseFeeBps)
	minBaseFeeNumerator := bpsToFeeNumerator(minBaseFeeBps)

	periodFrequency := decimal.NewFromInt(totalDuration / int64(numberOfPeriod))

	var reductionFactor decimal.Decimal
	if feeSchedulerMode == FeeSchedulerModeLinear {
		// totalReduction := maxBaseFeeNumerator.Sub(minBaseFeeNumerator)                  //new(big.Int).Sub(maxBaseFeeNumerator, minBaseFeeNumerator)
		// reductionFactor = totalReduction.Div(decimal.NewFromInt(int64(numberOfPeriod))) //new(big.Int).Div(totalReduction, big.NewInt(int64(numberOfPeriod)))

		reductionFactor = maxBaseFeeNumerator.Sub(minBaseFeeNumerator).Div(decimal.NewFromUint64(uint64(numberOfPeriod)))

	} else {
		// ratio := float64(minBaseFeeNumerator.BigInt().Int64()) / float64(maxBaseFeeNumerator.BigInt().Int64())
		// decayBase := math.Pow(ratio, 1.0/float64(numberOfPeriod))
		// reductionFactor = decimal.NewFromInt(int64(float64(BASIS_POINT_MAX.Int64()) * (1 - decayBase)))

		decayBase := dmath.Pow(minBaseFeeNumerator.Div(maxBaseFeeNumerator), N1.DivRound(decimal.NewFromUint64(uint64(numberOfPeriod)), 18), 18)
		reductionFactor = BASIS_POINT_MAX.Mul(N1.Sub(decayBase))

	}

	return &BaseFeeParameters{
		CliffFeeNumerator: maxBaseFeeNumerator.BigInt().Uint64(),
		NumberOfPeriod:    uint16(numberOfPeriod),
		PeriodFrequency:   periodFrequency.BigInt().Uint64(),
		ReductionFactor:   reductionFactor.BigInt().Uint64(),
		FeeSchedulerMode:  feeSchedulerMode,
	}, nil
}

func GetDynamicFeeParams(baseFeeBps int64, maxPriceChangeBps int) (*DynamicFeeParameters, error) {
	if maxPriceChangeBps == 0 {
		maxPriceChangeBps = MAX_PRICE_CHANGE_BPS_DEFAULT
	}

	if maxPriceChangeBps > MAX_PRICE_CHANGE_BPS_DEFAULT {
		return nil, errors.New(
			"maxPriceChangeBps must be <= MAX_PRICE_CHANGE_BPS_DEFAULT",
		)
	}

	// priceRatio = maxPriceChangeBps / BASIS_POINT_MAX + 1
	priceRatio := decimal.NewFromInt(int64(maxPriceChangeBps)).
		Div(BASIS_POINT_MAX).
		Add(N1)
	// Q64: sqrt(priceRatio) * 2^64
	sqrtPriceRatioQ64 := decimalSqrt(priceRatio).Round(19).Mul(Q64).Floor()
	// 2️⃣ deltaBinId = (sqrtPriceRatioQ64 - ONE_Q64) / BIN_STEP_BPS_U128_DEFAULT * 2
	deltaBinId := sqrtPriceRatioQ64.Sub(Q64).Div(BIN_STEP_BPS_U128_DEFAULT).Floor().Mul(N2)
	// 3️⃣ maxVolatilityAccumulator = deltaBinId * BASIS_POINT_MAX
	maxVolatilityAccumulator := deltaBinId.Mul(BASIS_POINT_MAX)
	// 4️⃣ squareVfaBin = (maxVolatilityAccumulator * BIN_STEP_BPS_DEFAULT)^2
	squareVfaBin := maxVolatilityAccumulator.Mul(BIN_STEP_BPS_DEFAULT).Pow(N2)
	// baseFeeNumerator
	baseFeeNumerator := bpsToFeeNumerator(baseFeeBps)

	// maxDynamicFeeNumerator = baseFeeNumerator * 20% (divide by 100)
	maxDynamicFeeNumerator := baseFeeNumerator.Mul(N20)       //new(big.Int).Mul(baseFeeNumerator, big.NewInt(20))
	maxDynamicFeeNumerator = maxDynamicFeeNumerator.Div(N100) //maxDynamicFeeNumerator.Div(maxDynamicFeeNumerator, big.NewInt(100))

	// vFee = maxDynamicFeeNumerator * 1e11 - 99999999999
	vFee := maxDynamicFeeNumerator.Mul(N100_000_000_000) //new(big.Int).Mul(maxDynamicFeeNumerator, big.NewInt(100_000_000_000))
	vFee = vFee.Sub(N99_999_999_999)                     // vFee.Sub(vFee, big.NewInt(99_999_999_999))

	// variableFeeControl = vFee / squareVfaBin
	variableFeeControl := vFee.Div(squareVfaBin) // new(big.Int).Div(vFee, squareVfaBin)

	return &DynamicFeeParameters{
		BinStep:                  uint16(BIN_STEP_BPS_DEFAULT.BigInt().Int64()),
		BinStepU128:              u128.GenUint128FromString(BIN_STEP_BPS_U128_DEFAULT.String()),
		FilterPeriod:             DYNAMIC_FEE_FILTER_PERIOD_DEFAULT,
		DecayPeriod:              DYNAMIC_FEE_DECAY_PERIOD_DEFAULT,
		ReductionFactor:          DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT,
		MaxVolatilityAccumulator: uint32(maxVolatilityAccumulator.BigInt().Int64()),
		VariableFeeControl:       uint32(variableFeeControl.BigInt().Int64()),
	}, nil
}
