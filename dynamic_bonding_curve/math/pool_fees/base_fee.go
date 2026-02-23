package pool_fees

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

type FeeRateLimiter struct {
	CliffFeeNumerator  *big.Int
	FeeIncrementBps    uint16
	MaxLimiterDuration *big.Int
	ReferenceAmount    *big.Int
}

func (f FeeRateLimiter) Validate(collectFeeMode dbc.CollectFeeMode, activationType dbc.ActivationType) bool {
	return validateFeeRateLimiter(f.CliffFeeNumerator, new(big.Int).SetUint64(uint64(f.FeeIncrementBps)), f.MaxLimiterDuration, f.ReferenceAmount, collectFeeMode, activationType)
}

func (f FeeRateLimiter) GetMinBaseFeeNumerator() *big.Int {
	return GetRateLimiterMinBaseFeeNumerator(f.CliffFeeNumerator)
}

func (f FeeRateLimiter) GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection dbc.TradeDirection, includedFeeAmount *big.Int) *big.Int {
	if IsRateLimiterApplied(currentPoint, activationPoint, tradeDirection, f.MaxLimiterDuration, f.ReferenceAmount, new(big.Int).SetUint64(uint64(f.FeeIncrementBps))) {
		v, _ := GetFeeNumeratorFromIncludedAmount(f.CliffFeeNumerator, f.ReferenceAmount, new(big.Int).SetUint64(uint64(f.FeeIncrementBps)), includedFeeAmount)
		return v
	}
	return new(big.Int).Set(f.CliffFeeNumerator)
}

func (f FeeRateLimiter) GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection dbc.TradeDirection, excludedFeeAmount *big.Int) *big.Int {
	if IsRateLimiterApplied(currentPoint, activationPoint, tradeDirection, f.MaxLimiterDuration, f.ReferenceAmount, new(big.Int).SetUint64(uint64(f.FeeIncrementBps))) {
		v, _ := GetFeeNumeratorFromExcludedAmount(f.CliffFeeNumerator, f.ReferenceAmount, new(big.Int).SetUint64(uint64(f.FeeIncrementBps)), excludedFeeAmount)
		return v
	}
	return new(big.Int).Set(f.CliffFeeNumerator)
}

type FeeScheduler struct {
	CliffFeeNumerator *big.Int
	NumberOfPeriod    uint16
	PeriodFrequency   *big.Int
	ReductionFactor   *big.Int
	FeeSchedulerMode  dbc.BaseFeeMode
}

func (f FeeScheduler) Validate(_ dbc.CollectFeeMode, _ dbc.ActivationType) bool {
	return validateFeeScheduler(uint16(f.NumberOfPeriod), f.PeriodFrequency, f.ReductionFactor, f.CliffFeeNumerator, f.FeeSchedulerMode)
}

func (f FeeScheduler) GetMinBaseFeeNumerator() *big.Int {
	v, _ := GetFeeSchedulerMinBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.ReductionFactor, f.FeeSchedulerMode)
	return v
}

func (f FeeScheduler) GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, _ dbc.TradeDirection, _ *big.Int) *big.Int {
	v, _ := GetBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.PeriodFrequency, f.ReductionFactor, f.FeeSchedulerMode, currentPoint, activationPoint)
	return v
}

func (f FeeScheduler) GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, _ dbc.TradeDirection, _ *big.Int) *big.Int {
	v, _ := GetBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.PeriodFrequency, f.ReductionFactor, f.FeeSchedulerMode, currentPoint, activationPoint)
	return v
}

func GetBaseFeeHandler(cliffFeeNumerator *big.Int, firstFactor uint16, secondFactor *big.Int, thirdFactor *big.Int, baseFeeMode dbc.BaseFeeMode) (dbc.BaseFeeHandler, error) {
	switch baseFeeMode {
	case dbc.BaseFeeModeFeeSchedulerLinear, dbc.BaseFeeModeFeeSchedulerExponential:
		return FeeScheduler{CliffFeeNumerator: cliffFeeNumerator, NumberOfPeriod: firstFactor, PeriodFrequency: secondFactor, ReductionFactor: thirdFactor, FeeSchedulerMode: baseFeeMode}, nil
	case dbc.BaseFeeModeRateLimiter:
		return FeeRateLimiter{CliffFeeNumerator: cliffFeeNumerator, FeeIncrementBps: firstFactor, MaxLimiterDuration: secondFactor, ReferenceAmount: thirdFactor}, nil
	default:
		return nil, errors.New("Invalid base fee mode")
	}
}
