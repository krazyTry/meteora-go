package dammv2

import (
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/math/pool_fees"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func ValidateFeeTimeScheduler(numberOfPeriod uint16, periodFrequency, reductionFactor, cliffFeeNumerator *big.Int, baseFeeMode BaseFeeMode, poolVersion PoolVersion) bool {
	return pool_fees.ValidateFeeTimeScheduler(numberOfPeriod, periodFrequency, reductionFactor, cliffFeeNumerator, baseFeeMode, poolVersion)
}

func ValidateFeeTimeSchedulerBaseFeeIsStatic(currentPoint, activationPoint, numberOfPeriod, periodFrequency *big.Int) bool {
	return pool_fees.ValidateFeeTimeSchedulerBaseFeeIsStatic(currentPoint, activationPoint, numberOfPeriod, periodFrequency)
}

func ValidateFeeMarketCapScheduler(cliffFeeNumerator *big.Int, numberOfPeriod uint16, sqrtPriceStepBps *big.Int, reductionFactor *big.Int, schedulerExpirationDuration *big.Int, feeMarketCapSchedulerMode BaseFeeMode, poolVersion PoolVersion) bool {
	return pool_fees.ValidateFeeMarketCapScheduler(cliffFeeNumerator, numberOfPeriod, sqrtPriceStepBps, reductionFactor, schedulerExpirationDuration, feeMarketCapSchedulerMode, poolVersion)
}

func ValidateFeeMarketCapBaseFeeIsStatic(currentPoint, activationPoint, schedulerExpirationDuration *big.Int) bool {
	return pool_fees.ValidateFeeMarketCapBaseFeeIsStatic(currentPoint, activationPoint, schedulerExpirationDuration)
}

func ValidateFeeRateLimiter(cliffFeeNumerator *big.Int, feeIncrementBps uint16, maxFeeBps uint16, maxLimiterDuration uint32, referenceAmount *big.Int, collectFeeMode shared.CollectFeeMode, activationType ActivationType, poolVersion PoolVersion) bool {
	return pool_fees.ValidateFeeRateLimiter(cliffFeeNumerator, feeIncrementBps, maxFeeBps, maxLimiterDuration, referenceAmount, collectFeeMode, activationType, poolVersion)
}

func ValidateFeeRateLimiterBaseFeeIsStatic(currentPoint, activationPoint *big.Int, maxLimiterDuration uint32, referenceAmount *big.Int, maxFeeBps uint16, feeIncrementBps uint16) bool {
	return pool_fees.ValidateFeeRateLimiterBaseFeeIsStatic(currentPoint, activationPoint, maxLimiterDuration, referenceAmount, maxFeeBps, feeIncrementBps)
}

func ValidateFeeFraction(numerator, denominator *big.Int) error {
	return pool_fees.ValidateFeeFraction(numerator, denominator)
}
