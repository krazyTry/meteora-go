package pool_fees

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func GetFeeMarketCapBaseFeeNumeratorByPeriod(cliffFeeNumerator *big.Int, numberOfPeriod uint16, period *big.Int, reductionFactor *big.Int, feeMarketCapSchedulerMode shared.BaseFeeMode) (*big.Int, error) {
	periodValue := new(big.Int).Set(period)
	maxPeriod := big.NewInt(int64(numberOfPeriod))
	if periodValue.Cmp(maxPeriod) > 0 {
		periodValue = maxPeriod
	}
	periodNumber := uint16(periodValue.Uint64())
	switch feeMarketCapSchedulerMode {
	case shared.BaseFeeModeFeeMarketCapSchedulerLinear:
		return GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor, periodNumber), nil
	case shared.BaseFeeModeFeeMarketCapSchedulerExp:
		return GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor, periodNumber), nil
	default:
		return nil, errors.New("invalid fee market cap scheduler mode")
	}
}

func GetFeeMarketCapBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, sqrtPriceStepBps uint16, schedulerExpirationDuration uint32, reductionFactor *big.Int, feeMarketCapSchedulerMode shared.BaseFeeMode, currentPoint, activationPoint, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error) {
	schedulerExpirationPoint := new(big.Int).Add(activationPoint, big.NewInt(int64(schedulerExpirationDuration)))
	var period *big.Int
	if currentPoint.Cmp(schedulerExpirationPoint) > 0 || currentPoint.Cmp(activationPoint) < 0 {
		period = big.NewInt(int64(numberOfPeriod))
	} else {
		if currentSqrtPrice.Cmp(initSqrtPrice) <= 0 {
			period = big.NewInt(0)
		} else {
			passed := new(big.Int).Sub(currentSqrtPrice, initSqrtPrice)
			passed.Mul(passed, big.NewInt(shared.BasisPointMax))
			passed.Div(passed, initSqrtPrice)
			passed.Div(passed, big.NewInt(int64(sqrtPriceStepBps)))
			if passed.Cmp(big.NewInt(int64(numberOfPeriod))) > 0 {
				period = big.NewInt(int64(numberOfPeriod))
			} else {
				period = passed
			}
		}
		if period.Cmp(big.NewInt(int64(numberOfPeriod))) > 0 {
			period = big.NewInt(int64(numberOfPeriod))
		}
	}
	return GetFeeMarketCapBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, period, reductionFactor, feeMarketCapSchedulerMode)
}

func GetFeeMarketCapMinBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, reductionFactor *big.Int, feeMarketCapSchedulerMode shared.BaseFeeMode) (*big.Int, error) {
	return GetFeeMarketCapBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, big.NewInt(int64(numberOfPeriod)), reductionFactor, feeMarketCapSchedulerMode)
}
