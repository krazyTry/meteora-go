package pool_fees

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

// FeeRateLimiter implements BaseFeeHandler.
type FeeRateLimiter struct {
	CliffFeeNumerator  *big.Int
	FeeIncrementBps    uint16
	MaxFeeBps          uint16
	MaxLimiterDuration uint32
	ReferenceAmount    *big.Int
}

func (f FeeRateLimiter) Validate(collectFeeMode shared.CollectFeeMode, activationType shared.ActivationType, poolVersion shared.PoolVersion) bool {
	return ValidateFeeRateLimiter(f.CliffFeeNumerator, f.FeeIncrementBps, f.MaxFeeBps, f.MaxLimiterDuration, f.ReferenceAmount, collectFeeMode, activationType, poolVersion)
}

func (f FeeRateLimiter) GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection shared.TradeDirection, includedFeeAmount *big.Int, _initSqrtPrice, _currentSqrtPrice *big.Int) (*big.Int, error) {
	if IsRateLimiterApplied(f.ReferenceAmount, f.MaxLimiterDuration, f.MaxFeeBps, f.FeeIncrementBps, currentPoint, activationPoint, tradeDirection) {
		return GetFeeNumeratorFromIncludedFeeAmount(includedFeeAmount, f.ReferenceAmount, f.CliffFeeNumerator, f.MaxFeeBps, f.FeeIncrementBps)
	}
	return new(big.Int).Set(f.CliffFeeNumerator), nil
}

func (f FeeRateLimiter) GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection shared.TradeDirection, excludedFeeAmount *big.Int, _initSqrtPrice, _currentSqrtPrice *big.Int) (*big.Int, error) {
	if IsRateLimiterApplied(f.ReferenceAmount, f.MaxLimiterDuration, f.MaxFeeBps, f.FeeIncrementBps, currentPoint, activationPoint, tradeDirection) {
		return GetFeeNumeratorFromExcludedFeeAmount(excludedFeeAmount, f.ReferenceAmount, f.CliffFeeNumerator, f.MaxFeeBps, f.FeeIncrementBps)
	}
	return new(big.Int).Set(f.CliffFeeNumerator), nil
}

func (f FeeRateLimiter) ValidateBaseFeeIsStatic(currentPoint, activationPoint *big.Int) bool {
	return ValidateFeeRateLimiterBaseFeeIsStatic(currentPoint, activationPoint, f.MaxLimiterDuration, f.ReferenceAmount, f.MaxFeeBps, f.FeeIncrementBps)
}

func (f FeeRateLimiter) GetMinFeeNumerator() *big.Int {
	return GetRateLimiterMinBaseFeeNumerator(f.CliffFeeNumerator)
}

func (f FeeRateLimiter) GetMaxFeeNumerator() (*big.Int, error) {
	return GetRateLimiterMaxBaseFeeNumerator(shared.U64Max, f.ReferenceAmount, f.CliffFeeNumerator, f.MaxFeeBps, f.FeeIncrementBps)
}

// FeeTimeScheduler implements BaseFeeHandler.
type FeeTimeScheduler struct {
	CliffFeeNumerator    *big.Int
	NumberOfPeriod       uint16
	PeriodFrequency      *big.Int
	ReductionFactor      *big.Int
	FeeTimeSchedulerMode shared.BaseFeeMode
}

func (f FeeTimeScheduler) Validate(collectFeeMode shared.CollectFeeMode, activationType shared.ActivationType, poolVersion shared.PoolVersion) bool {
	return ValidateFeeTimeScheduler(f.NumberOfPeriod, f.PeriodFrequency, f.ReductionFactor, f.CliffFeeNumerator, f.FeeTimeSchedulerMode, poolVersion)
}

func (f FeeTimeScheduler) GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, _tradeDirection shared.TradeDirection, _includedFeeAmount *big.Int, _initSqrtPrice, _currentSqrtPrice *big.Int) (*big.Int, error) {
	return GetFeeTimeBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.PeriodFrequency, f.ReductionFactor, f.FeeTimeSchedulerMode, currentPoint, activationPoint)
}

func (f FeeTimeScheduler) GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, _tradeDirection shared.TradeDirection, _excludedFeeAmount *big.Int, _initSqrtPrice, _currentSqrtPrice *big.Int) (*big.Int, error) {
	return GetFeeTimeBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.PeriodFrequency, f.ReductionFactor, f.FeeTimeSchedulerMode, currentPoint, activationPoint)
}

func (f FeeTimeScheduler) ValidateBaseFeeIsStatic(currentPoint, activationPoint *big.Int) bool {
	return ValidateFeeTimeSchedulerBaseFeeIsStatic(currentPoint, activationPoint, big.NewInt(int64(f.NumberOfPeriod)), f.PeriodFrequency)
}

func (f FeeTimeScheduler) GetMinFeeNumerator() *big.Int {
	minFee, _ := GetFeeTimeMinBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.ReductionFactor, f.FeeTimeSchedulerMode)
	return minFee
}

func (f FeeTimeScheduler) GetMaxFeeNumerator() (*big.Int, error) {
	return new(big.Int).Set(f.CliffFeeNumerator), nil
}

// FeeMarketCapScheduler implements BaseFeeHandler.
type FeeMarketCapScheduler struct {
	CliffFeeNumerator           *big.Int
	NumberOfPeriod              uint16
	SqrtPriceStepBps            uint16
	SchedulerExpirationDuration uint32
	ReductionFactor             *big.Int
	FeeMarketCapSchedulerMode   shared.BaseFeeMode
}

func (f FeeMarketCapScheduler) Validate(_collectFeeMode shared.CollectFeeMode, _activationType shared.ActivationType, poolVersion shared.PoolVersion) bool {
	return ValidateFeeMarketCapScheduler(f.CliffFeeNumerator, f.NumberOfPeriod, big.NewInt(int64(f.SqrtPriceStepBps)), f.ReductionFactor, big.NewInt(int64(f.SchedulerExpirationDuration)), f.FeeMarketCapSchedulerMode, poolVersion)
}

func (f FeeMarketCapScheduler) GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, _tradeDirection shared.TradeDirection, _includedFeeAmount *big.Int, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error) {
	return GetFeeMarketCapBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.SqrtPriceStepBps, f.SchedulerExpirationDuration, f.ReductionFactor, f.FeeMarketCapSchedulerMode, currentPoint, activationPoint, initSqrtPrice, currentSqrtPrice)
}

func (f FeeMarketCapScheduler) GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, _tradeDirection shared.TradeDirection, _excludedFeeAmount *big.Int, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error) {
	return GetFeeMarketCapBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.SqrtPriceStepBps, f.SchedulerExpirationDuration, f.ReductionFactor, f.FeeMarketCapSchedulerMode, currentPoint, activationPoint, initSqrtPrice, currentSqrtPrice)
}

func (f FeeMarketCapScheduler) ValidateBaseFeeIsStatic(currentPoint, activationPoint *big.Int) bool {
	return ValidateFeeMarketCapBaseFeeIsStatic(currentPoint, activationPoint, big.NewInt(int64(f.SchedulerExpirationDuration)))
}

func (f FeeMarketCapScheduler) GetMinFeeNumerator() *big.Int {
	minFee, _ := GetFeeMarketCapMinBaseFeeNumerator(f.CliffFeeNumerator, f.NumberOfPeriod, f.ReductionFactor, f.FeeMarketCapSchedulerMode)
	return minFee
}

func (f FeeMarketCapScheduler) GetMaxFeeNumerator() (*big.Int, error) {
	return new(big.Int).Set(f.CliffFeeNumerator), nil
}

// GetBaseFeeHandler selects handler based on base fee mode.
func GetBaseFeeHandler(rawData []byte) (shared.BaseFeeHandler, error) {
	if len(rawData) < 9 {
		return nil, errors.New("invalid base fee data")
	}
	modeIndex := rawData[8]
	baseFeeMode := shared.BaseFeeMode(modeIndex)
	switch baseFeeMode {
	case shared.BaseFeeModeFeeTimeSchedulerLinear, shared.BaseFeeModeFeeTimeSchedulerExponential:
		poolFees, err := helpers.DecodePodAlignedFeeTimeScheduler(rawData)
		if err != nil {
			return nil, err
		}
		return FeeTimeScheduler{
			CliffFeeNumerator:    new(big.Int).SetUint64(poolFees.CliffFeeNumerator),
			NumberOfPeriod:       poolFees.NumberOfPeriod,
			PeriodFrequency:      new(big.Int).SetUint64(poolFees.PeriodFrequency),
			ReductionFactor:      new(big.Int).SetUint64(poolFees.ReductionFactor),
			FeeTimeSchedulerMode: shared.BaseFeeMode(poolFees.BaseFeeMode),
		}, nil
	case shared.BaseFeeModeRateLimiter:
		poolFees, err := helpers.DecodePodAlignedFeeRateLimiter(rawData)
		if err != nil {
			return nil, err
		}
		return FeeRateLimiter{
			CliffFeeNumerator:  new(big.Int).SetUint64(poolFees.CliffFeeNumerator),
			FeeIncrementBps:    poolFees.FeeIncrementBps,
			MaxFeeBps:          uint16(poolFees.MaxFeeBps),
			MaxLimiterDuration: poolFees.MaxLimiterDuration,
			ReferenceAmount:    new(big.Int).SetUint64(poolFees.ReferenceAmount),
		}, nil
	case shared.BaseFeeModeFeeMarketCapSchedulerLinear, shared.BaseFeeModeFeeMarketCapSchedulerExp:
		poolFees, err := helpers.DecodePodAlignedFeeMarketCapScheduler(rawData)
		if err != nil {
			return nil, err
		}
		return FeeMarketCapScheduler{
			CliffFeeNumerator:           new(big.Int).SetUint64(poolFees.CliffFeeNumerator),
			NumberOfPeriod:              poolFees.NumberOfPeriod,
			SqrtPriceStepBps:            uint16(poolFees.SqrtPriceStepBps),
			SchedulerExpirationDuration: poolFees.SchedulerExpirationDuration,
			ReductionFactor:             new(big.Int).SetUint64(poolFees.ReductionFactor),
			FeeMarketCapSchedulerMode:   shared.BaseFeeMode(poolFees.BaseFeeMode),
		}, nil
	default:
		return nil, errors.New("invalid base fee mode")
	}
}
