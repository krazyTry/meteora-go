package helpers

import (
	"context"
	"errors"
	"math/big"

	binary "github.com/gagliardetto/binary"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"

	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func HasPartner(poolState dammv2gen.Pool) bool {
	return !poolState.Partner.Equals(solanago.PublicKey{})
}

func GetCurrentPoint(ctx context.Context, client *rpc.Client, activationType uint8) (*big.Int, error) {
	slot, err := client.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	if activationType == ActivationTypeSlot {
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
	return pool.PoolStatus == uint8(PoolStatusEnable) && currentPoint.Cmp(big.NewInt(int64(pool.ActivationPoint))) >= 0
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
	fee := new(big.Int).Mul(big.NewInt(int64(bps)), big.NewInt(FeeDenominator))
	return fee.Div(fee, big.NewInt(BasisPointMax))
}

func FeeNumeratorToBps(feeNumerator *big.Int) uint16 {
	if feeNumerator == nil {
		return 0
	}
	val := new(big.Int).Mul(feeNumerator, big.NewInt(BasisPointMax))
	val.Div(val, big.NewInt(FeeDenominator))
	return uint16(val.Uint64())
}

func ConvertToLamports(amount decimal.Decimal, tokenDecimal uint8) *big.Int {
	valueInLamports := amount.Mul(decimal.New(1, int32(tokenDecimal)))
	return FromDecimalToBigInt(valueInLamports)
}

func FromDecimalToBigInt(value decimal.Decimal) *big.Int {
	return value.Floor().BigInt()
}

func GetFeeTimeSchedulerParams(startingBaseFeeBps, endingBaseFeeBps uint16, baseFeeMode uint8, numberOfPeriod uint16, totalDuration uint32) (dammv2gen.BaseFeeParameters, error) {
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
	cliffFee := BpsToFeeNumerator(startingBaseFeeBps)
	reductionFactor := BpsToFeeNumerator(startingBaseFeeBps - endingBaseFeeBps)
	periodFrequency := new(big.Int).Div(big.NewInt(int64(totalDuration)), big.NewInt(int64(numberOfPeriod)))
	data, err := EncodeFeeTimeSchedulerParams(cliffFee, numberOfPeriod, periodFrequency, reductionFactor, baseFeeMode)
	if err != nil {
		return dammv2gen.BaseFeeParameters{}, err
	}
	return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
}

func GetFeeRateLimiterParams(startingBaseFeeBps uint16, maxFeeBps uint16, maxLimiterDuration uint32, referenceAmount *big.Int) (dammv2gen.BaseFeeParameters, error) {
	cliffFee := BpsToFeeNumerator(startingBaseFeeBps)
	data, err := EncodeFeeRateLimiterParams(cliffFee, startingBaseFeeBps, maxLimiterDuration, maxFeeBps, referenceAmount)
	if err != nil {
		return dammv2gen.BaseFeeParameters{}, err
	}
	return dammv2gen.BaseFeeParameters{Data: toBaseFeeData(data)}, nil
}

func GetFeeMarketCapSchedulerParams(startingBaseFeeBps, endingBaseFeeBps uint16, baseFeeMode uint8, numberOfPeriod uint16, sqrtPriceStepBps uint16, schedulerExpirationDuration uint32) (dammv2gen.BaseFeeParameters, error) {
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

func GetBaseFeeParams(baseFeeMode uint8, startingBaseFeeBps uint16, endingBaseFeeBps uint16, numberOfPeriod uint16, totalDuration uint32, sqrtPriceStepBps uint16, schedulerExpirationDuration uint32, maxLimiterDuration uint32, maxFeeBps uint16, referenceAmount *big.Int) (dammv2gen.BaseFeeParameters, error) {
	switch baseFeeMode {
	case BaseFeeModeFeeTimeSchedulerLinear, BaseFeeModeFeeTimeSchedulerExponential:
		return GetFeeTimeSchedulerParams(startingBaseFeeBps, endingBaseFeeBps, baseFeeMode, numberOfPeriod, totalDuration)
	case BaseFeeModeRateLimiter:
		return GetFeeRateLimiterParams(startingBaseFeeBps, maxFeeBps, maxLimiterDuration, referenceAmount)
	case BaseFeeModeFeeMarketCapSchedulerLinear, BaseFeeModeFeeMarketCapSchedulerExp:
		return GetFeeMarketCapSchedulerParams(startingBaseFeeBps, endingBaseFeeBps, baseFeeMode, numberOfPeriod, sqrtPriceStepBps, schedulerExpirationDuration)
	default:
		return dammv2gen.BaseFeeParameters{}, errors.New("invalid base fee mode")
	}
}

// GetDynamicFeeParams builds dynamic fee parameters from base fee and max price change.
func GetDynamicFeeParams(baseFeeBps uint16, maxPriceChangeBps uint16) (dammv2gen.DynamicFeeParameters, error) {
	if maxPriceChangeBps == 0 {
		maxPriceChangeBps = MaxPriceChangeBpsDefault
	}
	if maxPriceChangeBps > MaxPriceChangeBpsDefault {
		return dammv2gen.DynamicFeeParameters{}, errors.New("maxPriceChangeBps must be <= MaxPriceChangeBpsDefault")
	}

	priceRatio := new(big.Float).SetPrec(256).SetFloat64(float64(maxPriceChangeBps)/float64(BasisPointMax) + 1)
	sqrtPriceRatio := new(big.Float).SetPrec(256).Sqrt(priceRatio)
	sqrtPriceRatio.Mul(sqrtPriceRatio, new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 64)))
	sqrtPriceRatioQ64, _ := sqrtPriceRatio.Int(nil)

	deltaBinId := new(big.Int).Sub(sqrtPriceRatioQ64, OneQ64)
	deltaBinId.Div(deltaBinId, BinStepBpsU128Default)
	deltaBinId.Mul(deltaBinId, big.NewInt(2))

	maxVolatilityAccumulator := new(big.Int).Mul(deltaBinId, big.NewInt(BasisPointMax))
	squareVfaBin := new(big.Int).Mul(maxVolatilityAccumulator, big.NewInt(BinStepBpsDefault))
	squareVfaBin.Mul(squareVfaBin, squareVfaBin)

	baseFeeNumerator := new(big.Int).Set(BpsToFeeNumerator(baseFeeBps))
	maxDynamicFeeNumerator := new(big.Int).Mul(baseFeeNumerator, big.NewInt(20))
	maxDynamicFeeNumerator.Div(maxDynamicFeeNumerator, big.NewInt(100))
	vFee := new(big.Int).Mul(maxDynamicFeeNumerator, DynamicFeeScalingFactor)
	vFee.Sub(vFee, DynamicFeeRoundingOffset)
	variableFeeControl := new(big.Int).Div(vFee, squareVfaBin)

	return dammv2gen.DynamicFeeParameters{
		BinStep:                  BinStepBpsDefault,
		BinStepU128:              uint128FromBig(BinStepBpsU128Default),
		FilterPeriod:             DynamicFeeFilterPeriodDefault,
		DecayPeriod:              DynamicFeeDecayPeriodDefault,
		ReductionFactor:          DynamicFeeReductionFactorDefault,
		MaxVolatilityAccumulator: uint32(maxVolatilityAccumulator.Uint64()),
		VariableFeeControl:       uint32(variableFeeControl.Uint64()),
	}, nil
}

func GetMaxFeeBps(poolVersion uint8) uint16 {
	if poolVersion == PoolVersionV0 {
		return MaxFeeBpsV0
	}
	return MaxFeeBpsV1
}

func GetMaxFeeNumerator(poolVersion uint8) *big.Int {
	if poolVersion == PoolVersionV0 {
		return big.NewInt(MaxFeeNumeratorV0)
	}
	return big.NewInt(MaxFeeNumeratorV1)
}

func ValidatePoolFeeBps(baseFeeBps uint16, maxFeeBps uint16, poolVersion uint8) error {
	if baseFeeBps < MinFeeBps {
		return errors.New("base fee bps too low")
	}
	if maxFeeBps > GetMaxFeeBps(poolVersion) {
		return errors.New("max fee bps too high")
	}
	return nil
}

func GetAmountWithSlippage(amount *big.Int, slippageBps uint16, swapMode uint8) *big.Int {
	if slippageBps == 0 {
		return new(big.Int).Set(amount)
	}
	if swapMode == SwapModeExactOut {
		factor := new(big.Int).Add(big.NewInt(BasisPointMax), big.NewInt(int64(slippageBps)))
		return new(big.Int).Div(new(big.Int).Mul(amount, factor), big.NewInt(BasisPointMax))
	}
	factor := new(big.Int).Sub(big.NewInt(BasisPointMax), big.NewInt(int64(slippageBps)))
	return new(big.Int).Div(new(big.Int).Mul(amount, factor), big.NewInt(BasisPointMax))
}

func GetPriceFromSqrtPrice(sqrtPrice *big.Int, tokenADecimal, tokenBDecimal uint8) decimal.Decimal {
	decSqrt := decimal.NewFromBigInt(sqrtPrice, 0)
	price := decSqrt.Mul(decSqrt).
		Mul(decimal.New(1, int32(tokenADecimal-tokenBDecimal))).
		Div(decimal.NewFromBigInt(
			new(big.Int).Lsh(
				decimal.NewFromInt(1).BigInt(),
				128,
			),
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
	slippage := ((100 + rate) / 100) * float64(BasisPointMax)
	return new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(int64(slippage))), big.NewInt(BasisPointMax))
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
