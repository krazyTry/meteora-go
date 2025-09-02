package cp_amm

import (
	"errors"
	"fmt"
	"math"
	"math/big"

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
		tmp := period.Mul(reductionFactor)
		feeNumerator = cliffFeeNumerator.Sub(tmp)
	} else { // Exponential
		scale := decimal.NewFromInt(1).Shift(int32(SCALE_OFFSET)) // 2^SCALE_OFFSET
		bps := reductionFactor.Mul(scale).Div(decimal.NewFromBigInt(BASIS_POINT_MAX, 0))

		base := decimal.NewFromBigInt(ONE, 0).Sub(bps)
		result := pow(base, period)

		feeNumerator = cliffFeeNumerator.Mul(result).Div(scale)
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
		return decimal.Zero
	}

	// squareVfaBin = (volatilityAccumulator * binStep)^2
	squareVfaBin := volatilityAccumulator.Mul(binStep)
	squareVfaBin = squareVfaBin.Mul(squareVfaBin)

	// vFee = variableFeeControl * squareVfaBin
	vFee := variableFeeControl.Mul(squareVfaBin)

	// vFee + 99_999_999_999
	vFee = vFee.Add(decimal.NewFromInt(99_999_999_999))

	// divide by 100_000_000_000
	result := vFee.Div(decimal.NewFromInt(100_000_000_000))

	return result
}

// DynamicFeeParams
type DynamicFeeParams struct {
	VolatilityAccumulator *big.Int
	BinStep               *big.Int
	VariableFeeControl    *big.Int
}

// GetFeeNumerator
func GetFeeNumerator(
	currentPoint *big.Int,
	activationPoint *big.Int,
	numberOfPeriod int64,
	periodFrequency *big.Int,
	feeSchedulerMode FeeSchedulerMode,
	cliffFeeNumerator *big.Int,
	reductionFactor *big.Int,
	dynamicFeeParams *DynamicFeeParams,
) *big.Int {
	periodF := decimal.NewFromBigInt(periodFrequency, 0)
	currentP := decimal.NewFromBigInt(currentPoint, 0)
	activationP := decimal.NewFromBigInt(activationPoint, 0)

	if periodF.IsZero() || currentP.Cmp(activationP) < 0 {
		return new(big.Int).Set(cliffFeeNumerator)
	}

	// period = min(numberOfPeriod, (currentPoint - activationPoint)/periodFrequency)

	diff := currentP.Sub(activationP)
	period := diff.Div(periodF) // new(big.Int).Div(diff, periodFrequency)

	maxPeriod := decimal.NewFromInt(numberOfPeriod)

	if period.Cmp(maxPeriod) > 0 {
		period = maxPeriod
	}

	// feeNumerator = getBaseFeeNumerator(...)
	feeNumerator := getBaseFeeNumerator(feeSchedulerMode, decimal.NewFromBigInt(cliffFeeNumerator, 0), period, decimal.NewFromBigInt(reductionFactor, 0))

	if dynamicFeeParams != nil {
		dynamicFeeNumerator := getDynamicFeeNumerator(
			decimal.NewFromBigInt(dynamicFeeParams.VolatilityAccumulator, 0),
			decimal.NewFromBigInt(dynamicFeeParams.BinStep, 0),
			decimal.NewFromBigInt(dynamicFeeParams.VariableFeeControl, 0),
		)
		feeNumerator = feeNumerator.Add(dynamicFeeNumerator)
	}

	if feeNumerator.Cmp(decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0)) > 0 {
		feeNumerator = decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0)
	}

	return feeNumerator.BigInt()
}

// FeeMode
type FeeMode struct {
	FeeOnInput   bool
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
	inAmount, sqrtPrice, liquidity, tradeFeeNumerator *big.Int,
	swapBaseForQuote bool,
	collectFeeMode CollectFeeMode,
) (*big.Int, *big.Int, *big.Int, error) {

	feeMode := GetFeeMode(collectFeeMode, !swapBaseForQuote)

	actualInAmount := decimal.NewFromBigInt(inAmount, 0) //new(big.Int).Set(inAmount)
	totalFee := decimal.Zero

	if feeMode.FeeOnInput {

		fee, err := mulDiv(actualInAmount, decimal.NewFromBigInt(tradeFeeNumerator, 0), decimal.NewFromBigInt(FEE_DENOMINATOR, 0), true)
		if err != nil {
			return nil, nil, nil, err
		}
		totalFee = fee
		actualInAmount = actualInAmount.Sub(totalFee)

	}

	nextSqrtPrice := getNextSqrtPrice(actualInAmount, decimal.NewFromBigInt(sqrtPrice, 0), decimal.NewFromBigInt(liquidity, 0), swapBaseForQuote)

	var outAmount *big.Int
	if swapBaseForQuote {
		outAmount = GetAmountBFromLiquidityDelta(liquidity, sqrtPrice, nextSqrtPrice.BigInt(), false)
	} else {
		outAmount = GetAmountAFromLiquidityDelta(liquidity, sqrtPrice, nextSqrtPrice.BigInt(), false)
	}

	var amountOut *big.Int
	if feeMode.FeeOnInput {
		amountOut = outAmount
	} else {
		fee, err := mulDiv(decimal.NewFromBigInt(outAmount, 0), decimal.NewFromBigInt(tradeFeeNumerator, 0), decimal.NewFromBigInt(FEE_DENOMINATOR, 0), true)
		if err != nil {
			return nil, nil, nil, err
		}
		totalFee = fee
		amountOut = new(big.Int).Sub(outAmount, totalFee.BigInt())
	}
	return amountOut, totalFee.BigInt(), nextSqrtPrice.BigInt(), nil

}

// GetPriceImpact
// abs(execution_price - spot_price) / spot_price * 100%
func GetPriceImpact(
	amountIn, amountOut, currentSqrtPrice *big.Int,
	aToB bool,
	tokenADecimal, tokenBDecimal uint8,
) (*big.Float, error) {
	amountInF := decimal.NewFromBigInt(amountIn, 0)
	amountOutF := decimal.NewFromBigInt(amountOut, 0)

	if amountInF.IsZero() {
		return big.NewFloat(0), nil
	}
	if amountOutF.IsZero() {
		return nil, errors.New("amountOut must be greater than 0")
	}

	spotPrice := getPriceFromSqrtPrice(decimal.NewFromBigInt(currentSqrtPrice, 0), tokenADecimal, tokenBDecimal)

	// 1.0526315900277009477
	executionPrice := amountInF.DivRound(amountOutF, 19) // new(big.Float).Quo(amountInF, amountOutF)

	expDiff := int32(tokenBDecimal) - int32(tokenADecimal)
	if !aToB {
		expDiff = int32(tokenADecimal) - int32(tokenBDecimal)
	}
	adjustFactor := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(expDiff)))

	executionPrice = executionPrice.Mul(adjustFactor)

	var actualExecutionPrice decimal.Decimal
	if aToB {
		// 0.94999998999999999996
		actualExecutionPrice = decimal.NewFromInt(1).DivRound(executionPrice, 20)
	} else {
		actualExecutionPrice = executionPrice
	}
	// priceImpact = |execution_price - spot_price| / spot_price * 100
	diff := actualExecutionPrice.Sub(spotPrice).Abs()
	priceImpact := diff.DivRound(spotPrice, 20).Mul(decimal.NewFromInt(100))

	return priceImpact.BigFloat(), nil
}

type RewardResult struct {
	FeeTokenA *big.Int
	FeeTokenB *big.Int
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

func CalculateUnClaimFee(poolState *Pool, positionState *Position) (*big.Int, *big.Int) {

	// totalPositionLiquidity := decimal.NewFromBigInt(positionState.UnlockedLiquidity.BigInt(), 0).Add(
	// 	decimal.NewFromBigInt(positionState.VestedLiquidity.BigInt(), 0).Add(
	// 		decimal.NewFromBigInt(positionState.PermanentLockedLiquidity.BigInt(), 0),
	// 	),
	// )

	totalPositionLiquidity := new(big.Int).Add(positionState.UnlockedLiquidity.BigInt(),
		new(big.Int).Add(positionState.VestedLiquidity.BigInt(), positionState.PermanentLockedLiquidity.BigInt()))

	feeAPerTokenStored := new(big.Int).Sub(
		bytesToBigIntLE(poolState.FeeAPerLiquidity[:]),
		bytesToBigIntLE(positionState.FeeAPerTokenCheckpoint[:]),
	)

	feeBPerTokenStored := new(big.Int).Sub(
		bytesToBigIntLE(poolState.FeeBPerLiquidity[:]),
		bytesToBigIntLE(positionState.FeeBPerTokenCheckpoint[:]),
	)

	feeA := new(big.Int).Rsh(new(big.Int).Mul(totalPositionLiquidity, feeAPerTokenStored), LIQUIDITY_SCALE)
	feeB := new(big.Int).Rsh(new(big.Int).Mul(totalPositionLiquidity, feeBPerTokenStored), LIQUIDITY_SCALE)

	return new(big.Int).Add(new(big.Int).SetUint64(positionState.FeeAPending), feeA),
		new(big.Int).Add(new(big.Int).SetUint64(positionState.FeeBPending), feeB)
}

func CalculateUnClaimReward(poolState *Pool, positionState *Position) []*big.Int {
	var rewards []*big.Int
	for _, item := range positionState.RewardInfos {
		rewards = append(rewards, new(big.Int).SetUint64(item.RewardPendings))
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
	numerator := bpsBig.Mul(decimal.NewFromBigInt(FEE_DENOMINATOR, 0))   //new(big.Int).Mul(bpsBig, FEE_DENOMINATOR)
	numerator = numerator.Div(decimal.NewFromBigInt(BASIS_POINT_MAX, 0)) // numerator.Div(numerator, BASIS_POINT_MAX)
	return numerator
}

// feeNumeratorToBps converts a fee numerator back to basis points (bps).
// @param feeNumerator - The fee numerator to convert
// @returns The equivalent value in basis points [1-10_000]
func feeNumeratorToBps(feeNumerator *big.Int) int64 {
	// bps = feeNumerator * BASIS_POINT_MAX / FEE_DENOMINATOR
	result := new(big.Int).Mul(feeNumerator, BASIS_POINT_MAX)
	result.Div(result, FEE_DENOMINATOR)

	return result.Int64()
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
		totalReduction := maxBaseFeeNumerator.Sub(minBaseFeeNumerator)                  //new(big.Int).Sub(maxBaseFeeNumerator, minBaseFeeNumerator)
		reductionFactor = totalReduction.Div(decimal.NewFromInt(int64(numberOfPeriod))) //new(big.Int).Div(totalReduction, big.NewInt(int64(numberOfPeriod)))
	} else {
		ratio := float64(minBaseFeeNumerator.BigInt().Int64()) / float64(maxBaseFeeNumerator.BigInt().Int64())
		decayBase := math.Pow(ratio, 1.0/float64(numberOfPeriod))
		reductionFactor = decimal.NewFromInt(int64(float64(BASIS_POINT_MAX.Int64()) * (1 - decayBase)))
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
	priceRatio := float64(maxPriceChangeBps)/float64(BASIS_POINT_MAX.Int64()) + 1

	// sqrtPriceRatioQ64 = sqrt(priceRatio) * 2^64
	sqrtPriceRatio := math.Sqrt(priceRatio)

	sqrtPriceRatioQ64 := new(big.Int).SetUint64(uint64(sqrtPriceRatio * math.Pow(2, 64)))

	// deltaBinId = (sqrtPriceRatioQ64 - 1) / BIN_STEP_BPS_U128_DEFAULT * 2
	deltaBinId := decimal.NewFromBigInt(sqrtPriceRatioQ64, 0).Sub(decimal.NewFromInt(1)) // new(big.Int).Sub(sqrtPriceRatioQ64, big.NewInt(1))
	deltaBinId = deltaBinId.Div(decimal.NewFromBigInt(BIN_STEP_BPS_U128_DEFAULT, 0))
	deltaBinId = deltaBinId.Mul(decimal.NewFromInt(2))

	// maxVolatilityAccumulator = deltaBinId * BASIS_POINT_MAX
	maxVolatilityAccumulator := deltaBinId.Mul(decimal.NewFromBigInt(BASIS_POINT_MAX, 0)) //new(big.Int).Mul(deltaBinId, BASIS_POINT_MAX)

	// squareVfaBin = (maxVolatilityAccumulator * BIN_STEP_BPS_DEFAULT) ^ 2
	squareVfaBin := maxVolatilityAccumulator.Mul(decimal.NewFromBigInt(BIN_STEP_BPS_DEFAULT, 0)) //new(big.Int).Mul(maxVolatilityAccumulator, BIN_STEP_BPS_DEFAULT)
	squareVfaBin = squareVfaBin.Mul(squareVfaBin)

	// baseFeeNumerator
	baseFeeNumerator := bpsToFeeNumerator(baseFeeBps)

	// maxDynamicFeeNumerator = baseFeeNumerator * 20% (除以100)
	maxDynamicFeeNumerator := baseFeeNumerator.Mul(decimal.NewFromInt(20))       //new(big.Int).Mul(baseFeeNumerator, big.NewInt(20))
	maxDynamicFeeNumerator = maxDynamicFeeNumerator.Div(decimal.NewFromInt(100)) //maxDynamicFeeNumerator.Div(maxDynamicFeeNumerator, big.NewInt(100))

	// vFee = maxDynamicFeeNumerator * 1e11 - 99999999999
	vFee := maxDynamicFeeNumerator.Mul(decimal.NewFromInt(100_000_000_000)) //new(big.Int).Mul(maxDynamicFeeNumerator, big.NewInt(100_000_000_000))
	vFee = vFee.Sub(decimal.NewFromInt(99_999_999_999))                     // vFee.Sub(vFee, big.NewInt(99_999_999_999))

	// variableFeeControl = vFee / squareVfaBin
	variableFeeControl := vFee.Div(squareVfaBin) // new(big.Int).Div(vFee, squareVfaBin)

	return &DynamicFeeParameters{
		BinStep:                  uint16(BIN_STEP_BPS_DEFAULT.Int64()),
		BinStepU128:              u128.GenUint128FromString(BIN_STEP_BPS_U128_DEFAULT.String()),
		FilterPeriod:             DYNAMIC_FEE_FILTER_PERIOD_DEFAULT,
		DecayPeriod:              DYNAMIC_FEE_DECAY_PERIOD_DEFAULT,
		ReductionFactor:          DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT,
		MaxVolatilityAccumulator: uint32(maxVolatilityAccumulator.BigInt().Int64()),
		VariableFeeControl:       uint32(variableFeeControl.BigInt().Int64()),
	}, nil
}
