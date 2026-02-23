package helpers

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func GetFeeSchedulerMaxBaseFeeNumerator(cliffFeeNumerator *big.Int) *big.Int {
	return new(big.Int).Set(cliffFeeNumerator)
}

func GetFeeSchedulerMinBaseFeeNumerator(cliffFeeNumerator *big.Int, numberOfPeriod uint16, reductionFactor *big.Int, feeSchedulerMode shared.BaseFeeMode) (*big.Int, error) {
	return GetBaseFeeNumeratorByPeriod(cliffFeeNumerator, numberOfPeriod, big.NewInt(int64(numberOfPeriod)), reductionFactor, feeSchedulerMode)
}

func GetBaseFeeNumeratorByPeriod(cliffFeeNumerator *big.Int, numberOfPeriod uint16, period *big.Int, reductionFactor *big.Int, feeSchedulerMode shared.BaseFeeMode) (*big.Int, error) {
	periodValue := new(big.Int).Set(period)
	if periodValue.Cmp(big.NewInt(int64(numberOfPeriod))) > 0 {
		periodValue = big.NewInt(int64(numberOfPeriod))
	}
	if periodValue.Cmp(big.NewInt(int64(shared.U16Max))) > 0 {
		return nil, errors.New("Math overflow")
	}
	periodNumber := int(periodValue.Uint64())

	switch feeSchedulerMode {
	case shared.BaseFeeModeFeeSchedulerLinear:
		return GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor, periodNumber)
	case shared.BaseFeeModeFeeSchedulerExponential:
		return GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor, periodNumber)
	default:
		return nil, errors.New("Invalid fee scheduler mode")
	}
}

func GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor *big.Int, period int) (*big.Int, error) {
	reduction := new(big.Int).Mul(big.NewInt(int64(period)), reductionFactor)
	return Sub(cliffFeeNumerator, reduction)
}

func GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor *big.Int, period int) (*big.Int, error) {
	if period == 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}
	basisPointMax := big.NewInt(shared.MaxBasisPoint)
	oneQ64 := new(big.Int).Lsh(big.NewInt(1), 64)
	bps := new(big.Int).Lsh(reductionFactor, 64)
	bps = bps.Div(bps, basisPointMax)
	base, err := Sub(oneQ64, bps)
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

func GetFeeNumeratorFromIncludedAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, includedFeeAmount *big.Int) (*big.Int, error) {
	if includedFeeAmount.Cmp(referenceAmount) <= 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}
	c := cliffFeeNumerator
	diff := new(big.Int).Sub(includedFeeAmount, referenceAmount)
	a := new(big.Int).Div(diff, referenceAmount)
	b := new(big.Int).Mod(diff, referenceAmount)
	maxIndex, err := GetMaxIndex(cliffFeeNumerator, feeIncrementBps)
	if err != nil {
		return nil, err
	}
	i, err := ToNumerator(feeIncrementBps, big.NewInt(shared.FeeDenominator))
	if err != nil {
		return nil, err
	}
	x0 := referenceAmount
	one := big.NewInt(1)
	two := big.NewInt(2)

	var tradingFeeNumerator *big.Int
	if a.Cmp(maxIndex) < 0 {
		numerator1 := new(big.Int).Add(c, new(big.Int).Mul(c, a))
		numerator1.Add(numerator1, new(big.Int).Div(new(big.Int).Mul(i, new(big.Int).Mul(a, new(big.Int).Add(a, one))), two))
		numerator2 := new(big.Int).Add(c, new(big.Int).Mul(i, new(big.Int).Add(a, one)))
		firstFee := new(big.Int).Mul(x0, numerator1)
		secondFee := new(big.Int).Mul(b, numerator2)
		tradingFeeNumerator = new(big.Int).Add(firstFee, secondFee)
	} else {
		numerator1 := new(big.Int).Add(c, new(big.Int).Mul(c, maxIndex))
		numerator1.Add(numerator1, new(big.Int).Div(new(big.Int).Mul(i, new(big.Int).Mul(maxIndex, new(big.Int).Add(maxIndex, one))), two))
		numerator2 := big.NewInt(shared.MaxFeeNumerator)
		firstFee := new(big.Int).Mul(x0, numerator1)
		d := new(big.Int).Sub(a, maxIndex)
		leftAmount := new(big.Int).Add(new(big.Int).Mul(d, x0), b)
		secondFee := new(big.Int).Mul(leftAmount, numerator2)
		tradingFeeNumerator = new(big.Int).Add(firstFee, secondFee)
	}

	denominator := big.NewInt(shared.FeeDenominator)
	tradingFee := new(big.Int).Add(tradingFeeNumerator, new(big.Int).Sub(denominator, one))
	tradingFee = tradingFee.Div(tradingFee, denominator)

	feeNumerator, err := MulDiv(tradingFee, big.NewInt(shared.FeeDenominator), includedFeeAmount, shared.RoundingUp)
	if err != nil {
		return nil, err
	}
	return feeNumerator, nil
}

func GetMaxIndex(cliffFeeNumerator, feeIncrementBps *big.Int) (*big.Int, error) {
	if cliffFeeNumerator.Cmp(big.NewInt(shared.MaxFeeNumerator)) > 0 {
		return nil, errors.New("Cliff fee numerator exceeds maximum fee numerator")
	}
	deltaNumerator := new(big.Int).Sub(big.NewInt(shared.MaxFeeNumerator), cliffFeeNumerator)
	feeIncrementNumerator, err := ToNumerator(feeIncrementBps, big.NewInt(shared.FeeDenominator))
	if err != nil {
		return nil, err
	}
	if feeIncrementNumerator.Sign() == 0 {
		return nil, errors.New("Fee increment numerator cannot be zero")
	}
	return new(big.Int).Div(deltaNumerator, feeIncrementNumerator), nil
}

func ToNumerator(bps *big.Int, feeDenominator *big.Int) (*big.Int, error) {
	return MulDiv(bps, feeDenominator, big.NewInt(shared.MaxBasisPoint), shared.RoundingDown)
}

// pow computes base^exponent with Q64 scaling when scaling=true.
func pow(base, exponent *big.Int, scaling bool) (*big.Int, error) {
	one := new(big.Int).Lsh(big.NewInt(1), shared.Resolution)

	if exponent.Sign() == 0 {
		return new(big.Int).Set(one), nil
	}
	if base.Sign() == 0 {
		return big.NewInt(0), nil
	}
	if base.Cmp(one) == 0 {
		return new(big.Int).Set(one), nil
	}
	if exponent.Cmp(big.NewInt(1)) == 0 {
		return new(big.Int).Set(base), nil
	}

	if exponent.BitLen() > 64 {
		return nil, errors.New("pow: exponent too large")
	}
	exponentU64 := exponent.Uint64()
	result := new(big.Int).Set(one)
	currentBase := new(big.Int).Set(base)

	for exponentU64 > 0 {
		if exponentU64&1 == 1 {
			if scaling {
				res, err := MulDiv(result, currentBase, one, shared.RoundingDown)
				if err != nil {
					return nil, err
				}
				result = res
			} else {
				result = new(big.Int).Mul(result, currentBase)
			}
		}
		exponentU64 >>= 1
		if exponentU64 > 0 {
			if scaling {
				res, err := MulDiv(currentBase, currentBase, one, shared.RoundingDown)
				if err != nil {
					return nil, err
				}
				currentBase = res
			} else {
				currentBase = new(big.Int).Mul(currentBase, currentBase)
			}
		}
	}
	return result, nil
}
