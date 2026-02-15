package pool_fees

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func GetFeeTimeBaseFeeNumeratorByPeriod(cliffFeeNumerator *big.Int, numberOfPeriod uint16, period *big.Int, reductionFactor *big.Int, feeTimeSchedulerMode shared.BaseFeeMode) (*big.Int, error) {
	periodValue := new(big.Int).Set(period)
	maxPeriod := big.NewInt(int64(numberOfPeriod))
	if periodValue.Cmp(maxPeriod) > 0 {
		periodValue = maxPeriod
	}
	periodNumber := periodValue.Uint64()
	if periodNumber > shared.U16Max {
		return nil, errors.New("math overflow")
	}
	switch feeTimeSchedulerMode {
	case shared.BaseFeeModeFeeTimeSchedulerLinear:
		return GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor, uint16(periodNumber)), nil
	case shared.BaseFeeModeFeeTimeSchedulerExponential:
		return GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor, uint16(periodNumber)), nil
	default:
		return nil, errors.New("invalid fee time scheduler mode")
	}
}

func GetFeeTimeBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, periodFrequency *big.Int, reductionFactor *big.Int, feeTimeSchedulerMode shared.BaseFeeMode, currentPoint, activationPoint *big.Int) (*big.Int, error) {
	if periodFrequency.Sign() == 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}
	var period *big.Int
	if currentPoint.Cmp(activationPoint) < 0 {
		period = big.NewInt(int64(numberOfPeriod))
	} else {
		period = new(big.Int).Sub(currentPoint, activationPoint)
		period.Div(period, periodFrequency)
		if period.Cmp(big.NewInt(int64(numberOfPeriod))) > 0 {
			period = big.NewInt(int64(numberOfPeriod))
		}
	}
	return GetFeeTimeBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, period, reductionFactor, feeTimeSchedulerMode)
}

func GetFeeTimeMinBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, reductionFactor *big.Int, feeTimeSchedulerMode shared.BaseFeeMode) (*big.Int, error) {
	return GetFeeTimeBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, big.NewInt(int64(numberOfPeriod)), reductionFactor, feeTimeSchedulerMode)
}
