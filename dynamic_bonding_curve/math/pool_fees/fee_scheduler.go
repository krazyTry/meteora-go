package pool_fees

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func GetFeeSchedulerMaxBaseFeeNumerator(cliffFeeNumerator *big.Int) *big.Int {
	return new(big.Int).Set(cliffFeeNumerator)
}

func GetFeeSchedulerMinBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, reductionFactor *big.Int, feeSchedulerMode dbc.BaseFeeMode) (*big.Int, error) {
	return GetBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, big.NewInt(int64(numberOfPeriod)), reductionFactor, feeSchedulerMode)
}

func GetBaseFeeNumeratorByPeriod(cliffFeeNumerator *big.Int, numberOfPeriod uint16, period *big.Int, reductionFactor *big.Int, feeSchedulerMode dbc.BaseFeeMode) (*big.Int, error) {
	periodValue := new(big.Int).Set(period)
	if periodValue.Cmp(big.NewInt(int64(numberOfPeriod))) > 0 {
		periodValue = big.NewInt(int64(numberOfPeriod))
	}
	if periodValue.Cmp(big.NewInt(int64(dbc.U16Max))) > 0 {
		return nil, errors.New("Math overflow")
	}
	periodNumber := int(periodValue.Uint64())

	switch feeSchedulerMode {
	case dbc.BaseFeeModeFeeSchedulerLinear:
		return GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor, periodNumber)
	case dbc.BaseFeeModeFeeSchedulerExponential:
		return GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor, periodNumber)
	default:
		return nil, errors.New("Invalid fee scheduler mode")
	}
}

func GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor *big.Int, period int) (*big.Int, error) {
	reduction := new(big.Int).Mul(big.NewInt(int64(period)), reductionFactor)
	return sub(cliffFeeNumerator, reduction)
}

func GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor *big.Int, period int) (*big.Int, error) {
	if period == 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}
	basisPointMax := big.NewInt(dbc.MaxBasisPoint)
	oneQ64 := new(big.Int).Lsh(big.NewInt(1), 64)
	bps := new(big.Int).Lsh(reductionFactor, 64)
	bps = bps.Div(bps, basisPointMax)
	base, err := sub(oneQ64, bps)
	if err != nil {
		return nil, err
	}
	result, err := pow(base, big.NewInt(int64(period)), true)
	if err != nil {
		return nil, err
	}
	prod := new(big.Int).Mul(cliffFeeNumerator, result)
	return new(big.Int).Div(prod, oneQ64), nil
}

func GetBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, periodFrequency *big.Int, reductionFactor *big.Int, feeSchedulerMode dbc.BaseFeeMode, currentPoint *big.Int, activationPoint *big.Int) (*big.Int, error) {
	if periodFrequency.Sign() == 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}
	period := new(big.Int).Sub(currentPoint, activationPoint)
	period = period.Div(period, periodFrequency)
	return GetBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, period, reductionFactor, feeSchedulerMode)
}
