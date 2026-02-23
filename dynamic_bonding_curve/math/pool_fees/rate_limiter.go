package pool_fees

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func IsRateLimiterApplied(currentPoint, activationPoint *big.Int, tradeDirection dbc.TradeDirection, maxLimiterDuration, referenceAmount, feeIncrementBps *big.Int) bool {
	if IsZeroRateLimiter(referenceAmount, maxLimiterDuration, feeIncrementBps) {
		return false
	}
	if tradeDirection == dbc.TradeDirectionBaseToQuote {
		return false
	}
	lastEffective := new(big.Int).Add(activationPoint, maxLimiterDuration)
	return currentPoint.Cmp(lastEffective) <= 0
}

func IsZeroRateLimiter(referenceAmount, maxLimiterDuration, feeIncrementBps *big.Int) bool {
	return referenceAmount.Sign() == 0 && maxLimiterDuration.Sign() == 0 && feeIncrementBps.Sign() == 0
}

func IsNonZeroRateLimiter(referenceAmount, maxLimiterDuration, feeIncrementBps *big.Int) bool {
	return referenceAmount.Sign() != 0 && maxLimiterDuration.Sign() != 0 && feeIncrementBps.Sign() != 0
}

func GetMaxIndex(cliffFeeNumerator, feeIncrementBps *big.Int) (*big.Int, error) {
	if cliffFeeNumerator.Cmp(big.NewInt(dbc.MaxFeeNumerator)) > 0 {
		return nil, errors.New("Cliff fee numerator exceeds maximum fee numerator")
	}
	deltaNumerator := new(big.Int).Sub(big.NewInt(dbc.MaxFeeNumerator), cliffFeeNumerator)
	feeIncrementNumerator, err := toNumerator(feeIncrementBps, big.NewInt(dbc.FeeDenominator))
	if err != nil {
		return nil, err
	}
	if feeIncrementNumerator.Sign() == 0 {
		return nil, errors.New("Fee increment numerator cannot be zero")
	}
	return new(big.Int).Div(deltaNumerator, feeIncrementNumerator), nil
}

func GetMaxOutAmountWithMinBaseFee(cliffFeeNumerator, referenceAmount, feeIncrementBps *big.Int) (*big.Int, error) {
	return GetRateLimiterExcludedFeeAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, referenceAmount)
}

func GetCheckedAmounts(cliffFeeNumerator, referenceAmount, feeIncrementBps *big.Int) (*big.Int, *big.Int, bool, error) {
	maxIndex, err := GetMaxIndex(cliffFeeNumerator, feeIncrementBps)
	if err != nil {
		return nil, nil, false, err
	}
	x0 := referenceAmount
	one := big.NewInt(1)
	maxIndexInput := new(big.Int).Mul(new(big.Int).Add(maxIndex, one), x0)
	if maxIndexInput.Cmp(dbc.U64Max) <= 0 {
		checkedIncluded := maxIndexInput
		checkedExcluded, err := GetRateLimiterExcludedFeeAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, checkedIncluded)
		return checkedExcluded, checkedIncluded, false, err
	}
	checkedExcluded, err := GetRateLimiterExcludedFeeAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, dbc.U64Max)
	return checkedExcluded, dbc.U64Max, true, err
}

func GetFeeNumeratorFromExcludedAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, excludedFeeAmount *big.Int) (*big.Int, error) {
	excludedFeeReferenceAmount, err := GetRateLimiterExcludedFeeAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, referenceAmount)
	if err != nil {
		return nil, err
	}
	if excludedFeeAmount.Cmp(excludedFeeReferenceAmount) <= 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}

	checkedExcludedFeeAmount, checkedIncludedFeeAmount, isOverflow, err := GetCheckedAmounts(cliffFeeNumerator, referenceAmount, feeIncrementBps)
	if err != nil {
		return nil, err
	}

	if excludedFeeAmount.Cmp(checkedExcludedFeeAmount) == 0 {
		return GetFeeNumeratorFromIncludedAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, checkedIncludedFeeAmount)
	}

	var includedFeeAmount *big.Int
	if excludedFeeAmount.Cmp(checkedExcludedFeeAmount) < 0 {
		two := big.NewInt(2)
		four := big.NewInt(4)

		i, err := toNumerator(feeIncrementBps, big.NewInt(dbc.FeeDenominator))
		if err != nil {
			return nil, err
		}
		x0 := referenceAmount
		d := big.NewInt(dbc.FeeDenominator)
		c := cliffFeeNumerator
		ex := excludedFeeAmount

		x := i
		y := new(big.Int).Mul(two, d)
		y.Mul(y, x0)
		y.Add(y, new(big.Int).Mul(i, x0))
		y.Sub(y, new(big.Int).Mul(two, new(big.Int).Mul(c, x0)))
		z := new(big.Int).Mul(two, ex)
		z.Mul(z, d)
		z.Mul(z, x0)

		discriminant := new(big.Int).Sub(new(big.Int).Mul(y, y), new(big.Int).Mul(four, new(big.Int).Mul(x, z)))
		sqrtDiscriminant := sqrt(discriminant)
		includedFeeAmount = new(big.Int).Div(new(big.Int).Sub(y, sqrtDiscriminant), new(big.Int).Mul(two, x))

		aPlusOne := new(big.Int).Div(includedFeeAmount, x0)
		firstExcluded, err := GetRateLimiterExcludedFeeAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, includedFeeAmount)
		if err != nil {
			return nil, err
		}
		excludedRemaining := new(big.Int).Sub(excludedFeeAmount, firstExcluded)
		remainingAmountFeeNumerator := new(big.Int).Add(c, new(big.Int).Mul(i, aPlusOne))
		includedRemaining, err := mulDiv(excludedRemaining, big.NewInt(dbc.FeeDenominator), new(big.Int).Sub(big.NewInt(dbc.FeeDenominator), remainingAmountFeeNumerator), dbc.RoundingUp)
		if err != nil {
			return nil, err
		}
		includedFeeAmount = new(big.Int).Add(includedFeeAmount, includedRemaining)
	} else {
		if isOverflow {
			return nil, errors.New("Math overflow")
		}
		excludedRemaining := new(big.Int).Sub(excludedFeeAmount, checkedExcludedFeeAmount)
		includedRemaining, err := mulDiv(excludedRemaining, big.NewInt(dbc.FeeDenominator), new(big.Int).Sub(big.NewInt(dbc.FeeDenominator), big.NewInt(dbc.MaxFeeNumerator)), dbc.RoundingUp)
		if err != nil {
			return nil, err
		}
		includedFeeAmount = new(big.Int).Add(includedRemaining, checkedIncludedFeeAmount)
	}

	tradingFee := new(big.Int).Sub(includedFeeAmount, excludedFeeAmount)
	feeNumerator, err := mulDiv(tradingFee, big.NewInt(dbc.FeeDenominator), includedFeeAmount, dbc.RoundingUp)
	if err != nil {
		return nil, err
	}
	if feeNumerator.Cmp(cliffFeeNumerator) < 0 {
		return nil, errors.New("Undetermined error: fee numerator less than cliff fee numerator")
	}
	return feeNumerator, nil
}

func GetRateLimiterExcludedFeeAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, includedFeeAmount *big.Int) (*big.Int, error) {
	feeNumerator, err := GetFeeNumeratorFromIncludedAmount(cliffFeeNumerator, referenceAmount, feeIncrementBps, includedFeeAmount)
	if err != nil {
		return nil, err
	}
	tradingFee, err := mulDiv(includedFeeAmount, feeNumerator, big.NewInt(dbc.FeeDenominator), dbc.RoundingUp)
	if err != nil {
		return nil, err
	}
	return new(big.Int).Sub(includedFeeAmount, tradingFee), nil
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
	i, err := ToNumerator(feeIncrementBps, big.NewInt(dbc.FeeDenominator))
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
		numerator2 := big.NewInt(dbc.MaxFeeNumerator)
		firstFee := new(big.Int).Mul(x0, numerator1)
		d := new(big.Int).Sub(a, maxIndex)
		leftAmount := new(big.Int).Add(new(big.Int).Mul(d, x0), b)
		secondFee := new(big.Int).Mul(leftAmount, numerator2)
		tradingFeeNumerator = new(big.Int).Add(firstFee, secondFee)
	}

	denominator := big.NewInt(dbc.FeeDenominator)
	tradingFee := new(big.Int).Add(tradingFeeNumerator, new(big.Int).Sub(denominator, one))
	tradingFee = tradingFee.Div(tradingFee, denominator)

	feeNumerator, err := mulDiv(tradingFee, big.NewInt(dbc.FeeDenominator), includedFeeAmount, dbc.RoundingUp)
	if err != nil {
		return nil, err
	}
	return feeNumerator, nil
}

func GetRateLimiterMinBaseFeeNumerator(cliffFeeNumerator *big.Int) *big.Int {
	return new(big.Int).Set(cliffFeeNumerator)
}

// ToNumerator mirrors toNumerator in feeMath.
func ToNumerator(bps *big.Int, feeDenominator *big.Int) (*big.Int, error) {
	return mulDiv(bps, feeDenominator, big.NewInt(dbc.MaxBasisPoint), dbc.RoundingDown)
}
