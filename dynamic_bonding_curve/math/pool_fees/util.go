package pool_fees

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func sub(a, b *big.Int) (*big.Int, error) {
	if b.Cmp(a) > 0 {
		return nil, errors.New("sub: underflow")
	}
	return new(big.Int).Sub(a, b), nil
}

func mulDiv(x, y, denominator *big.Int, rounding dbc.Rounding) (*big.Int, error) {
	if denominator.Sign() == 0 {
		return nil, errors.New("mulDiv: division by zero")
	}
	if denominator.Cmp(big.NewInt(1)) == 0 || x.Sign() == 0 || y.Sign() == 0 {
		return new(big.Int).Mul(x, y), nil
	}
	prod := new(big.Int).Mul(x, y)
	if rounding == dbc.RoundingUp {
		numerator := new(big.Int).Add(prod, new(big.Int).Sub(denominator, big.NewInt(1)))
		return new(big.Int).Div(numerator, denominator), nil
	}
	return new(big.Int).Div(prod, denominator), nil
}

func sqrt(value *big.Int) *big.Int {
	if value.Sign() == 0 {
		return big.NewInt(0)
	}
	if value.Cmp(big.NewInt(1)) == 0 {
		return big.NewInt(1)
	}
	x := new(big.Int).Set(value)
	y := new(big.Int).Add(value, big.NewInt(1))
	y = y.Div(y, big.NewInt(2))

	for y.Cmp(x) < 0 {
		x = new(big.Int).Set(y)
		y = new(big.Int).Add(x, new(big.Int).Div(value, x))
		y = y.Div(y, big.NewInt(2))
	}
	return x
}

// pow computes base^exponent with Q64 scaling when scaling=true.
func pow(base, exponent *big.Int, scaling bool) (*big.Int, error) {
	one := new(big.Int).Lsh(big.NewInt(1), dbc.Resolution)

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
				res, err := mulDiv(result, currentBase, one, dbc.RoundingDown)
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
				res, err := mulDiv(currentBase, currentBase, one, dbc.RoundingDown)
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

func toNumerator(bps *big.Int, feeDenominator *big.Int) (*big.Int, error) {
	return mulDiv(bps, feeDenominator, big.NewInt(dbc.MaxBasisPoint), dbc.RoundingDown)
}

func validateFeeRateLimiter(cliffFeeNumerator, feeIncrementBps, maxLimiterDuration, referenceAmount *big.Int, collectFeeMode dbc.CollectFeeMode, activationType dbc.ActivationType) bool {
	if collectFeeMode != dbc.CollectFeeModeQuoteToken {
		return false
	}
	isZero := referenceAmount.Sign() == 0 && maxLimiterDuration.Sign() == 0 && feeIncrementBps.Sign() == 0
	if isZero {
		return true
	}
	isNonZero := referenceAmount.Sign() > 0 && maxLimiterDuration.Sign() > 0 && feeIncrementBps.Sign() > 0
	if !isNonZero {
		return false
	}
	maxLimiterDurationLimit := big.NewInt(dbc.MaxRateLimiterDurationInSeconds)
	if activationType == dbc.ActivationTypeSlot {
		maxLimiterDurationLimit = big.NewInt(dbc.MaxRateLimiterDurationInSlots)
	}
	if maxLimiterDuration.Cmp(maxLimiterDurationLimit) > 0 {
		return false
	}
	feeIncrementNumerator, err := toNumerator(feeIncrementBps, big.NewInt(dbc.FeeDenominator))
	if err != nil {
		return false
	}
	if feeIncrementNumerator.Cmp(big.NewInt(dbc.FeeDenominator)) >= 0 {
		return false
	}
	return true
}

func validateFeeScheduler(numberOfPeriod uint16, periodFrequency, reductionFactor, cliffFeeNumerator *big.Int, baseFeeMode dbc.BaseFeeMode) bool {
	if periodFrequency.Sign() != 0 || numberOfPeriod != 0 || reductionFactor.Sign() != 0 {
		if numberOfPeriod == 0 || periodFrequency.Sign() == 0 || reductionFactor.Sign() == 0 {
			return false
		}
	}
	minFeeNumerator, err := GetFeeSchedulerMinBaseFeeNumerator(cliffFeeNumerator, numberOfPeriod, reductionFactor, baseFeeMode)
	if err != nil {
		return false
	}
	maxFeeNumerator := GetFeeSchedulerMaxBaseFeeNumerator(cliffFeeNumerator)
	if minFeeNumerator.Cmp(big.NewInt(dbc.MinFeeNumerator)) < 0 || maxFeeNumerator.Cmp(big.NewInt(dbc.MaxFeeNumerator)) > 0 {
		return false
	}
	return true
}
