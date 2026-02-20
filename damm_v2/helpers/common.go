package helpers

import (
	"context"
	"errors"
	"math"
	"math/big"

	binary "github.com/gagliardetto/binary"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func HasPartner(poolState dammv2gen.Pool) bool {
	return !poolState.Partner.Equals(solanago.PublicKey{})
}

func GetCurrentPoint(ctx context.Context, client *rpc.Client, activationType shared.ActivationType) (*big.Int, error) {
	slot, err := client.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	if activationType == shared.ActivationTypeSlot {
		return big.NewInt(int64(slot)), nil
	}
	time, err := client.GetBlockTime(ctx, slot)
	if err != nil {
		return nil, err
	}
	if time == nil {
		return nil, errors.New("block time unavailable")
	}
	return big.NewInt(int64(*time)), nil
}

func IsSwapEnabled(pool dammv2gen.Pool, currentPoint *big.Int) bool {
	return pool.PoolStatus == uint8(shared.PoolStatusEnable) && currentPoint.Cmp(big.NewInt(int64(pool.ActivationPoint))) >= 0
}

func ConvertToFeeSchedulerSecondFactor(value *big.Int) []byte {
	return toBytesLE(value, 8)
}

func ParseFeeSchedulerSecondFactor(secondFactor []byte) *big.Int {
	return fromBytesLE(secondFactor)
}

func ConvertToRateLimiterSecondFactor(maxLimiterDuration *big.Int, maxFeeBps *big.Int) []byte {
	buf1 := toBytesLE(maxLimiterDuration, 4)
	buf2 := toBytesLE(maxFeeBps, 4)
	out := make([]byte, 0, 8)
	out = append(out, buf1...)
	out = append(out, buf2...)
	return out
}

func ParseRateLimiterSecondFactor(secondFactor []byte) (maxLimiterDuration uint32, maxFeeBps uint32) {
	if len(secondFactor) < 8 {
		return 0, 0
	}
	maxLimiterDuration = uint32(secondFactor[0]) | uint32(secondFactor[1])<<8 | uint32(secondFactor[2])<<16 | uint32(secondFactor[3])<<24
	maxFeeBps = uint32(secondFactor[4]) | uint32(secondFactor[5])<<8 | uint32(secondFactor[6])<<16 | uint32(secondFactor[7])<<24
	return maxLimiterDuration, maxFeeBps
}

func BpsToFeeNumerator(bps uint16) *big.Int {
	fee := new(big.Int).Mul(big.NewInt(int64(bps)), big.NewInt(shared.FeeDenominator))
	return fee.Div(fee, big.NewInt(shared.BasisPointMax))
}

func FeeNumeratorToBps(feeNumerator *big.Int) uint16 {
	if feeNumerator == nil {
		return 0
	}
	val := new(big.Int).Mul(feeNumerator, big.NewInt(shared.BasisPointMax))
	val.Div(val, big.NewInt(shared.FeeDenominator))
	return uint16(val.Uint64())
}

func ConvertToLamports(amount decimal.Decimal, tokenDecimal uint8) *big.Int {
	valueInLamports := amount.Mul(decimal.New(1, int32(tokenDecimal)))
	return FromDecimalToBigInt(valueInLamports)
}

func FromDecimalToBigInt(value decimal.Decimal) *big.Int {
	return value.Floor().BigInt()
}

func GetFeeTimeSchedulerParams(startingBaseFeeBps, endingBaseFeeBps uint16, baseFeeMode shared.BaseFeeMode, numberOfPeriod uint16, totalDuration uint32) (dammv2gen.BaseFeeParameters, error) {
	if startingBaseFeeBps == endingBaseFeeBps {
		if numberOfPeriod != 0 || totalDuration != 0 {
			return dammv2gen.BaseFeeParameters{}, errors.New("numberOfPeriod and totalDuration must both be zero")
		}
		data, err := EncodeFeeTimeSchedulerParams(BpsToFeeNumerator(startingBaseFeeBps), 0, big.NewInt(0), big.NewInt(0), baseFeeMode)
		if err != nil {
			return dammv2gen.BaseFeeParameters{}, err
		}
		return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
	}

	if totalDuration == 0 || numberOfPeriod == 0 {
		return dammv2gen.BaseFeeParameters{}, errors.New("totalDuration and numberOfPeriod must be > 0")
	}

	maxBaseFeeNumerator := BpsToFeeNumerator(startingBaseFeeBps)
	minBaseFeeNumerator := BpsToFeeNumerator(endingBaseFeeBps)
	periodFrequency := new(big.Int).Div(big.NewInt(int64(totalDuration)), big.NewInt(int64(numberOfPeriod)))

	var reductionFactor *big.Int
	if baseFeeMode == shared.BaseFeeModeFeeTimeSchedulerLinear {
		totalReduction := new(big.Int).Sub(maxBaseFeeNumerator, minBaseFeeNumerator)
		reductionFactor = new(big.Int).Div(totalReduction, big.NewInt(int64(numberOfPeriod)))
	} else {
		decayBase := decayBase(minBaseFeeNumerator, maxBaseFeeNumerator, uint64(numberOfPeriod))
		reductionFactor = calculateReductionFactor(decayBase, shared.BasisPointMax)
	}

	data, err := EncodeFeeTimeSchedulerParams(maxBaseFeeNumerator, numberOfPeriod, periodFrequency, reductionFactor, baseFeeMode)
	if err != nil {
		return dammv2gen.BaseFeeParameters{}, err
	}
	return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
}

// DecayBase computes:
//
//	ratio    = min / max
//	decayBase = ratio^(1/numberOfPeriod)
//
// min/max are big.Int (exact), ratio is big.Float (high precision).
// Pow uses float64 math.Pow due to stdlib limitations.
func decayBase(minBaseFeeNumerator, maxBaseFeeNumerator *big.Int, numberOfPeriod uint64) *big.Float {
	if numberOfPeriod == 0 {
		panic("numberOfPeriod cannot be 0")
	}
	if maxBaseFeeNumerator.Sign() == 0 {
		panic("maxBaseFeeNumerator cannot be 0")
	}
	// 如果你希望强制 0<min<=max，可以加校验：
	// if minBaseFeeNumerator.Sign() <= 0 || minBaseFeeNumerator.Cmp(maxBaseFeeNumerator) > 0 { ... }

	prec := uint(256)

	// ratio = min / max (big.Float)
	minF := new(big.Float).SetPrec(prec).SetInt(minBaseFeeNumerator)
	maxF := new(big.Float).SetPrec(prec).SetInt(maxBaseFeeNumerator)

	ratio := new(big.Float).SetPrec(prec).Quo(minF, maxF)

	// exponent = 1 / numberOfPeriod
	exp := new(big.Float).SetPrec(prec).Quo(
		new(big.Float).SetPrec(prec).SetInt64(1),
		new(big.Float).SetPrec(prec).SetUint64(numberOfPeriod),
	)

	// decayBase = pow(ratio, exp)  (via float64)
	r64, _ := ratio.Float64()
	e64, _ := exp.Float64()

	decay := math.Pow(r64, e64)
	decayBase := new(big.Float).SetPrec(prec).SetFloat64(decay)

	return decayBase
}

func calculateReductionFactor(decayBase *big.Float, basisPointMax int64) *big.Int {
	prec := uint(256)

	// 1 - decayBase
	one := new(big.Float).SetPrec(prec).SetInt64(1)
	oneMinusDecay := new(big.Float).SetPrec(prec).Sub(one, decayBase)

	// BASIS_POINT_MAX * (1 - decayBase)
	basisF := new(big.Float).SetPrec(prec).SetInt64(basisPointMax)
	resultFloat := new(big.Float).SetPrec(prec).Mul(basisF, oneMinusDecay)

	// 转换为 big.Int（向下取整）
	resultInt := new(big.Int)
	resultFloat.Int(resultInt) // 截断，相当于 floor

	return resultInt
}

func GetFeeRateLimiterParams(startingBaseFeeBps uint16, maxFeeBps uint16, maxLimiterDuration uint32, referenceAmount *big.Int) (dammv2gen.BaseFeeParameters, error) {
	cliffFee := BpsToFeeNumerator(startingBaseFeeBps)
	data, err := EncodeFeeRateLimiterParams(cliffFee, startingBaseFeeBps, maxLimiterDuration, maxFeeBps, referenceAmount)
	if err != nil {
		return dammv2gen.BaseFeeParameters{}, err
	}
	return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
}

func GetFeeMarketCapSchedulerParams(startingBaseFeeBps, endingBaseFeeBps uint16, baseFeeMode shared.BaseFeeMode, numberOfPeriod uint16, sqrtPriceStepBps uint16, schedulerExpirationDuration uint32) (dammv2gen.BaseFeeParameters, error) {
	if startingBaseFeeBps == endingBaseFeeBps {
		data, err := EncodeFeeMarketCapSchedulerParams(BpsToFeeNumerator(startingBaseFeeBps), 0, 0, schedulerExpirationDuration, big.NewInt(0), baseFeeMode)
		if err != nil {
			return dammv2gen.BaseFeeParameters{}, err
		}
		return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
	}
	cliffFee := BpsToFeeNumerator(startingBaseFeeBps)
	reductionFactor := BpsToFeeNumerator(startingBaseFeeBps - endingBaseFeeBps)
	data, err := EncodeFeeMarketCapSchedulerParams(cliffFee, numberOfPeriod, sqrtPriceStepBps, schedulerExpirationDuration, reductionFactor, baseFeeMode)
	if err != nil {
		return dammv2gen.BaseFeeParameters{}, err
	}
	return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
}

func GetBaseFeeParams(baseFeeMode shared.BaseFeeMode, startingBaseFeeBps uint16, endingBaseFeeBps uint16, numberOfPeriod uint16, totalDuration uint32, sqrtPriceStepBps uint16, schedulerExpirationDuration uint32, maxLimiterDuration uint32, maxFeeBps uint16, referenceAmount *big.Int) (dammv2gen.BaseFeeParameters, error) {
	switch baseFeeMode {
	case shared.BaseFeeModeFeeTimeSchedulerLinear, shared.BaseFeeModeFeeTimeSchedulerExponential:
		return GetFeeTimeSchedulerParams(startingBaseFeeBps, endingBaseFeeBps, baseFeeMode, numberOfPeriod, totalDuration)
	case shared.BaseFeeModeRateLimiter:
		return GetFeeRateLimiterParams(startingBaseFeeBps, maxFeeBps, maxLimiterDuration, referenceAmount)
	case shared.BaseFeeModeFeeMarketCapSchedulerLinear, shared.BaseFeeModeFeeMarketCapSchedulerExp:
		return GetFeeMarketCapSchedulerParams(startingBaseFeeBps, endingBaseFeeBps, baseFeeMode, numberOfPeriod, sqrtPriceStepBps, schedulerExpirationDuration)
	default:
		return dammv2gen.BaseFeeParameters{}, errors.New("invalid base fee mode")
	}
}

// GetDynamicFeeParams builds dynamic fee parameters from base fee and max price change.
func GetDynamicFeeParams(baseFeeBps uint16, maxPriceChangeBps uint16) (*dammv2gen.DynamicFeeParameters, error) {
	if maxPriceChangeBps == 0 {
		maxPriceChangeBps = shared.MaxPriceChangeBpsDefault
	}
	if maxPriceChangeBps > shared.MaxPriceChangeBpsDefault {
		return nil, errors.New("maxPriceChangeBps must be <= MaxPriceChangeBpsDefault")
	}

	priceRatio := new(big.Float).SetPrec(256).SetFloat64(float64(maxPriceChangeBps)/float64(shared.BasisPointMax) + 1)
	sqrtPriceRatio := new(big.Float).SetPrec(256).Sqrt(priceRatio)
	sqrtPriceRatio.Mul(sqrtPriceRatio, new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 64)))
	sqrtPriceRatioQ64, _ := sqrtPriceRatio.Int(nil)

	deltaBinId := new(big.Int).Sub(sqrtPriceRatioQ64, shared.OneQ64)
	deltaBinId.Div(deltaBinId, shared.BinStepBpsU128Default)
	deltaBinId.Mul(deltaBinId, big.NewInt(2))

	maxVolatilityAccumulator := new(big.Int).Mul(deltaBinId, big.NewInt(shared.BasisPointMax))
	squareVfaBin := new(big.Int).Mul(maxVolatilityAccumulator, big.NewInt(shared.BinStepBpsDefault))
	squareVfaBin.Mul(squareVfaBin, squareVfaBin)

	baseFeeNumerator := new(big.Int).Set(BpsToFeeNumerator(baseFeeBps))
	maxDynamicFeeNumerator := new(big.Int).Mul(baseFeeNumerator, big.NewInt(20))
	maxDynamicFeeNumerator.Div(maxDynamicFeeNumerator, big.NewInt(100))
	vFee := new(big.Int).Mul(maxDynamicFeeNumerator, shared.DynamicFeeScalingFactor)
	vFee.Sub(vFee, shared.DynamicFeeRoundingOffset)
	variableFeeControl := new(big.Int).Div(vFee, squareVfaBin)

	return &dammv2gen.DynamicFeeParameters{
		BinStep:                  shared.BinStepBpsDefault,
		BinStepU128:              uint128FromBig(shared.BinStepBpsU128Default),
		FilterPeriod:             shared.DynamicFeeFilterPeriodDefault,
		DecayPeriod:              shared.DynamicFeeDecayPeriodDefault,
		ReductionFactor:          shared.DynamicFeeReductionFactorDefault,
		MaxVolatilityAccumulator: uint32(maxVolatilityAccumulator.Uint64()),
		VariableFeeControl:       uint32(variableFeeControl.Uint64()),
	}, nil
}

func GetMaxFeeBps(poolVersion shared.PoolVersion) uint16 {
	if poolVersion == shared.PoolVersionV0 {
		return shared.MaxFeeBpsV0
	}
	return shared.MaxFeeBpsV1
}

func GetMaxFeeNumerator(poolVersion shared.PoolVersion) *big.Int {
	if poolVersion == shared.PoolVersionV0 {
		return big.NewInt(shared.MaxFeeNumeratorV0)
	}
	return big.NewInt(shared.MaxFeeNumeratorV1)
}

func ValidatePoolFeeBps(baseFeeBps uint16, maxFeeBps uint16, poolVersion shared.PoolVersion) error {
	if baseFeeBps < shared.MinFeeBps {
		return errors.New("base fee bps too low")
	}
	if maxFeeBps > GetMaxFeeBps(poolVersion) {
		return errors.New("max fee bps too high")
	}
	return nil
}

func GetAmountWithSlippage(amount *big.Int, slippageBps uint16, swapMode shared.SwapMode) *big.Int {
	if slippageBps == 0 {
		return new(big.Int).Set(amount)
	}
	if swapMode == shared.SwapModeExactOut {
		factor := new(big.Int).Add(big.NewInt(shared.BasisPointMax), big.NewInt(int64(slippageBps)))
		return new(big.Int).Div(new(big.Int).Mul(amount, factor), big.NewInt(shared.BasisPointMax))
	}
	factor := new(big.Int).Sub(big.NewInt(shared.BasisPointMax), big.NewInt(int64(slippageBps)))
	return new(big.Int).Div(new(big.Int).Mul(amount, factor), big.NewInt(shared.BasisPointMax))
}

func GetPriceFromSqrtPrice(sqrtPrice *big.Int, tokenADecimal, tokenBDecimal uint8) decimal.Decimal {
	decSqrt := decimal.NewFromBigInt(sqrtPrice, 0)
	price := decSqrt.Mul(decSqrt).
		Mul(decimal.New(1, int32(tokenADecimal-tokenBDecimal))).
		Div(decimal.NewFromBigInt(
			new(big.Int).Lsh(big.NewInt(1), 128),
			0,
		))
	return price
}

func GetSqrtPriceFromPrice(price decimal.Decimal, tokenADecimal, tokenBDecimal uint8) *big.Int {
	adjusted := price.Div(decimal.New(1, int32(tokenADecimal-tokenBDecimal)))
	f, _ := new(big.Float).SetPrec(256).SetString(adjusted.String())
	if f == nil {
		return big.NewInt(0)
	}
	sqrtValue := new(big.Float).SetPrec(256).Sqrt(f)
	sqrtValue.Mul(sqrtValue, new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 64)))
	sqrtValueQ64, _ := sqrtValue.Int(nil)
	return sqrtValueQ64
}

// initialPoolTokenAmount = tokenAmount * 10^decimals
func GetInitialPoolTokenAmount(amount *big.Int, decimals uint8) *big.Int {
	// 10^decimals
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	// tokenAmount * 10^decimals
	return new(big.Int).Mul(amount, scale)
}

func GetPriceImpact(amountIn, amountOut, currentSqrtPrice *big.Int, aToB bool, tokenADecimal, tokenBDecimal uint8) (decimal.Decimal, error) {
	if amountIn.Sign() == 0 {
		return decimal.Zero, nil
	}
	if amountOut.Sign() == 0 {
		return decimal.Zero, errors.New("amount out must be greater than 0")
	}
	spotPrice := GetPriceFromSqrtPrice(currentSqrtPrice, tokenADecimal, tokenBDecimal)
	executionPrice := decimal.NewFromBigInt(amountIn, 0).Div(decimal.NewFromBigInt(amountOut, 0))
	if aToB {
		executionPrice = decimal.NewFromInt(1).Div(executionPrice)
	}
	priceImpact := executionPrice.Sub(spotPrice).Abs().Div(spotPrice).Mul(decimal.NewFromInt(100))
	return priceImpact, nil
}

// Price change uses big integer math and returns percentage.
func GetPriceChange(nextSqrtPrice, currentSqrtPrice *big.Int) decimal.Decimal {
	diff := new(big.Int).Sub(new(big.Int).Mul(nextSqrtPrice, nextSqrtPrice), new(big.Int).Mul(currentSqrtPrice, currentSqrtPrice))
	if diff.Sign() < 0 {
		diff.Neg(diff)
	}
	den := new(big.Int).Mul(currentSqrtPrice, currentSqrtPrice)
	if den.Sign() == 0 {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(diff, 0).Div(decimal.NewFromBigInt(den, 0)).Mul(decimal.NewFromInt(100))
}

func GetMaxAmountWithSlippage(amount *big.Int, rate float64) *big.Int {
	slippage := ((100 + rate) / 100) * float64(shared.BasisPointMax)
	return new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(int64(slippage))), big.NewInt(shared.BasisPointMax))
}

func toBytesLE(v *big.Int, size int) []byte {
	if v == nil {
		return make([]byte, size)
	}
	b := v.Bytes()
	out := make([]byte, size)
	for i := 0; i < len(b) && i < size; i++ {
		out[i] = b[len(b)-1-i]
	}
	return out
}

func fromBytesLE(b []byte) *big.Int {
	out := new(big.Int)
	for i := len(b) - 1; i >= 0; i-- {
		out.Lsh(out, 8)
		out.Add(out, big.NewInt(int64(b[i])))
	}
	return out
}

func toBaseFeeData(data []byte) [30]uint8 {
	var out [30]uint8
	copy(out[:], data)
	return out
}

func uint128FromBig(v *big.Int) binary.Uint128 {
	if v == nil {
		return binary.Uint128{}
	}
	lo := new(big.Int).And(v, new(big.Int).SetUint64(^uint64(0))).Uint64()
	hi := new(big.Int).Rsh(new(big.Int).Set(v), 64).Uint64()
	return binary.Uint128{Lo: lo, Hi: hi}
}
