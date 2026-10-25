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

const maxExponential = 0x80000

func mulQ64(left, right *big.Int) (*big.Int, error) {
	product := new(big.Int).Mul(left, right)
	if product.Cmp(dbc.U128Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return new(big.Int).Rsh(product, dbc.Resolution), nil
}

// pow is the program's Q64 power. A base at or above 1 is inverted through u128::MAX.
func pow(base, exponent *big.Int, scaling bool) (*big.Int, error) {
	one := new(big.Int).Lsh(big.NewInt(1), dbc.Resolution)
	i32Min := big.NewInt(-2147483648)
	if exponent.Cmp(i32Min) == 0 || base.Sign() < 0 || base.Cmp(dbc.U128Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	if exponent.Sign() == 0 {
		return new(big.Int).Set(one), nil
	}

	invert := exponent.Sign() < 0
	exp := new(big.Int).Abs(exponent)
	if exp.Cmp(big.NewInt(maxExponential)) >= 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}

	squaredBase := new(big.Int).Set(base)
	result := new(big.Int).Set(one)
	if squaredBase.Cmp(result) >= 0 {
		squaredBase = new(big.Int).Div(dbc.U128Max, squaredBase)
		invert = !invert
	}

	remaining := new(big.Int).Set(exp)
	for remaining.Sign() != 0 {
		if remaining.Bit(0) == 1 {
			next, err := mulQ64(result, squaredBase)
			if err != nil {
				return nil, err
			}
			result = next
		}
		nextBase, err := mulQ64(squaredBase, squaredBase)
		if err != nil {
			return nil, err
		}
		squaredBase = nextBase
		remaining.Rsh(remaining, 1)
	}
	if result.Sign() == 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	if invert {
		result = new(big.Int).Div(dbc.U128Max, result)
	}
	if scaling {
		return result, nil
	}
	return new(big.Int).Rsh(result, dbc.Resolution), nil
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
