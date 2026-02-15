package pool_fees

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func ValidateFeeTimeScheduler(numberOfPeriod uint16, periodFrequency, reductionFactor, cliffFeeNumerator *big.Int, baseFeeMode shared.BaseFeeMode, poolVersion shared.PoolVersion) bool {
	if periodFrequency.Sign() != 0 || numberOfPeriod != 0 || reductionFactor.Sign() != 0 {
		if numberOfPeriod == 0 || periodFrequency.Sign() == 0 || reductionFactor.Sign() == 0 {
			return false
		}
	}
	minFeeNumerator, err := GetFeeTimeMinBaseFeeNumerator(cliffFeeNumerator, numberOfPeriod, reductionFactor, baseFeeMode)
	if err != nil {
		return false
	}
	maxFeeNumerator := GetMaxBaseFeeNumerator(cliffFeeNumerator)
	if err := ValidateFeeFraction(minFeeNumerator, big.NewInt(shared.FeeDenominator)); err != nil {
		return false
	}
	if err := ValidateFeeFraction(maxFeeNumerator, big.NewInt(shared.FeeDenominator)); err != nil {
		return false
	}
	if minFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) < 0 || maxFeeNumerator.Cmp(getMaxFeeNumerator(poolVersion)) > 0 {
		return false
	}
	return true
}

func ValidateFeeTimeSchedulerBaseFeeIsStatic(currentPoint, activationPoint, numberOfPeriod, periodFrequency *big.Int) bool {
	schedulerExpirationPoint := new(big.Int).Add(activationPoint, new(big.Int).Mul(numberOfPeriod, periodFrequency))
	return currentPoint.Cmp(schedulerExpirationPoint) > 0
}

func ValidateFeeMarketCapScheduler(cliffFeeNumerator *big.Int, numberOfPeriod uint16, sqrtPriceStepBps *big.Int, reductionFactor *big.Int, schedulerExpirationDuration *big.Int, feeMarketCapSchedulerMode shared.BaseFeeMode, poolVersion shared.PoolVersion) bool {
	if reductionFactor.Sign() <= 0 || sqrtPriceStepBps.Sign() <= 0 || schedulerExpirationDuration.Sign() <= 0 || numberOfPeriod == 0 {
		return false
	}
	minFeeNumerator, err := GetFeeMarketCapMinBaseFeeNumerator(cliffFeeNumerator, numberOfPeriod, reductionFactor, feeMarketCapSchedulerMode)
	if err != nil {
		return false
	}
	maxFeeNumerator := new(big.Int).Set(cliffFeeNumerator)
	if err := ValidateFeeFraction(minFeeNumerator, big.NewInt(shared.FeeDenominator)); err != nil {
		return false
	}
	if err := ValidateFeeFraction(maxFeeNumerator, big.NewInt(shared.FeeDenominator)); err != nil {
		return false
	}
	if minFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) < 0 || maxFeeNumerator.Cmp(getMaxFeeNumerator(poolVersion)) > 0 {
		return false
	}
	return true
}

func ValidateFeeMarketCapBaseFeeIsStatic(currentPoint, activationPoint, schedulerExpirationDuration *big.Int) bool {
	schedulerExpirationPoint := new(big.Int).Add(activationPoint, schedulerExpirationDuration)
	return currentPoint.Cmp(schedulerExpirationPoint) > 0
}

func ValidateFeeRateLimiter(cliffFeeNumerator *big.Int, feeIncrementBps uint16, maxFeeBps uint16, maxLimiterDuration uint32, referenceAmount *big.Int, collectFeeMode shared.CollectFeeMode, activationType shared.ActivationType, poolVersion shared.PoolVersion) bool {
	if collectFeeMode != shared.CollectFeeModeOnlyB {
		return false
	}
	maxFeeNumeratorFromBps := toNumerator(big.NewInt(int64(maxFeeBps)))
	if cliffFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) < 0 || cliffFeeNumerator.Cmp(maxFeeNumeratorFromBps) > 0 {
		return false
	}
	if IsZeroRateLimiter(referenceAmount, maxLimiterDuration, maxFeeBps, feeIncrementBps) {
		return true
	}
	if IsNonZeroRateLimiter(referenceAmount, maxLimiterDuration, maxFeeBps, feeIncrementBps) {
		return false
	}
	maxLimiterDurationLimit := uint32(shared.MaxRateLimiterDurationInSlots)
	if activationType == shared.ActivationTypeTimestamp {
		maxLimiterDurationLimit = uint32(shared.MaxRateLimiterDurationInSeconds)
	}
	if maxLimiterDuration > maxLimiterDurationLimit {
		return false
	}
	feeIncrementNumerator := toNumerator(big.NewInt(int64(feeIncrementBps)))
	if feeIncrementNumerator.Cmp(big.NewInt(shared.FeeDenominator)) >= 0 {
		return false
	}
	if maxFeeBps > getMaxFeeBps(poolVersion) {
		return false
	}
	minFeeNumerator, err := GetFeeNumeratorFromIncludedFeeAmount(big.NewInt(0), referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return false
	}
	maxFeeNumeratorFromAmount, err := GetFeeNumeratorFromIncludedFeeAmount(big.NewInt(int64(^uint64(0)>>1)), referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return false
	}
	if minFeeNumerator.Cmp(big.NewInt(shared.MinFeeNumerator)) < 0 || maxFeeNumeratorFromAmount.Cmp(getMaxFeeNumerator(poolVersion)) > 0 {
		return false
	}
	return true
}

func ValidateFeeRateLimiterBaseFeeIsStatic(currentPoint, activationPoint *big.Int, maxLimiterDuration uint32, referenceAmount *big.Int, maxFeeBps uint16, feeIncrementBps uint16) bool {
	if IsZeroRateLimiter(referenceAmount, maxLimiterDuration, maxFeeBps, feeIncrementBps) {
		return true
	}
	lastEffective := new(big.Int).Add(activationPoint, big.NewInt(int64(maxLimiterDuration)))
	return currentPoint.Cmp(lastEffective) > 0
}

func ValidateFeeFraction(numerator, denominator *big.Int) error {
	if denominator.Sign() == 0 || numerator.Cmp(denominator) >= 0 {
		return errors.New("invalid fee: numerator must be less than denominator and denominator must be non-zero")
	}
	return nil
}
