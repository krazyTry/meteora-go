package pool_fees

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func IsZeroRateLimiter(referenceAmount *big.Int, maxLimiterDuration uint32, maxFeeBps uint16, feeIncrementBps uint16) bool {
	return (referenceAmount == nil || referenceAmount.Sign() == 0) && maxLimiterDuration == 0 && maxFeeBps == 0 && feeIncrementBps == 0
}

func IsNonZeroRateLimiter(referenceAmount *big.Int, maxLimiterDuration uint32, maxFeeBps uint16, feeIncrementBps uint16) bool {
	return referenceAmount != nil && referenceAmount.Sign() != 0 && maxLimiterDuration != 0 && maxFeeBps != 0 && feeIncrementBps != 0
}

func IsRateLimiterApplied(referenceAmount *big.Int, maxLimiterDuration uint32, maxFeeBps uint16, feeIncrementBps uint16, currentPoint, activationPoint *big.Int, tradeDirection shared.TradeDirection) bool {
	if IsZeroRateLimiter(referenceAmount, maxLimiterDuration, maxFeeBps, feeIncrementBps) {
		return false
	}
	if tradeDirection == shared.TradeDirectionAtoB {
		return false
	}
	if currentPoint.Cmp(activationPoint) < 0 {
		return false
	}
	lastEffective := new(big.Int).Add(activationPoint, big.NewInt(int64(maxLimiterDuration)))
	if currentPoint.Cmp(lastEffective) > 0 {
		return false
	}
	return true
}

func GetMaxIndex(maxFeeBps uint16, cliffFeeNumerator *big.Int, feeIncrementBps uint16) (*big.Int, error) {
	maxFeeNumerator := toNumerator(big.NewInt(int64(maxFeeBps)))
	if cliffFeeNumerator.Cmp(maxFeeNumerator) > 0 {
		return nil, errors.New("cliffFeeNumerator cannot be greater than maxFeeNumerator")
	}
	deltaNumerator := new(big.Int).Sub(maxFeeNumerator, cliffFeeNumerator)
	feeIncrementNumerator := toNumerator(big.NewInt(int64(feeIncrementBps)))
	if feeIncrementNumerator.Sign() == 0 {
		return nil, errors.New("feeIncrementNumerator cannot be zero")
	}
	return new(big.Int).Div(deltaNumerator, feeIncrementNumerator), nil
}

func GetFeeNumeratorFromIncludedFeeAmount(inputAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps uint16, feeIncrementBps uint16) (*big.Int, error) {
	if inputAmount.Cmp(referenceAmount) <= 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}
	maxFeeNumerator := toNumerator(big.NewInt(int64(maxFeeBps)))
	c := new(big.Int).Set(cliffFeeNumerator)
	inputMinusRef := new(big.Int).Sub(inputAmount, referenceAmount)
	a := new(big.Int).Div(inputMinusRef, referenceAmount)
	b := new(big.Int).Mod(inputMinusRef, referenceAmount)

	maxIndex, err := GetMaxIndex(maxFeeBps, cliffFeeNumerator, feeIncrementBps)
	if err != nil {
		return nil, err
	}
	i := toNumerator(big.NewInt(int64(feeIncrementBps)))
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
		maxIndexPlus := new(big.Int).Set(maxIndex)
		numerator1 := new(big.Int).Add(c, new(big.Int).Mul(c, maxIndexPlus))
		numerator1.Add(numerator1, new(big.Int).Div(new(big.Int).Mul(i, new(big.Int).Mul(maxIndexPlus, new(big.Int).Add(maxIndexPlus, one))), two))
		firstFee := new(big.Int).Mul(x0, numerator1)
		d := new(big.Int).Sub(a, maxIndexPlus)
		secondFee := new(big.Int).Add(new(big.Int).Mul(d, x0), b)
		secondFee.Mul(secondFee, maxFeeNumerator)
		tradingFeeNumerator = new(big.Int).Add(firstFee, secondFee)
	}

	denominator := big.NewInt(shared.FeeDenominator)
	tradingFee := new(big.Int).Add(tradingFeeNumerator, denominator)
	tradingFee.Sub(tradingFee, one)
	tradingFee.Div(tradingFee, denominator)
	numerator := mulDiv(tradingFee, denominator, inputAmount, shared.RoundingUp)
	if numerator.Cmp(shared.U64Max) > 0 {
		return nil, errors.New("math overflow")
	}
	return numerator, nil
}

func excludedFeeAmountFromIncluded(includedFeeAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps, feeIncrementBps uint16) (*big.Int, error) {
	feeNumerator, err := GetFeeNumeratorFromIncludedFeeAmount(includedFeeAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return nil, err
	}
	tradingFee := mulDiv(includedFeeAmount, feeNumerator, big.NewInt(shared.FeeDenominator), shared.RoundingUp)
	return new(big.Int).Sub(includedFeeAmount, tradingFee), nil
}

func checkedRateLimiterAmounts(referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps, feeIncrementBps uint16) (checkedExcluded, checkedIncluded *big.Int, overflow bool, err error) {
	maxIndex, err := GetMaxIndex(maxFeeBps, cliffFeeNumerator, feeIncrementBps)
	if err != nil {
		return nil, nil, false, err
	}
	checkedIncluded = new(big.Int).Add(maxIndex, big.NewInt(1))
	checkedIncluded.Mul(checkedIncluded, referenceAmount)
	if checkedIncluded.Cmp(shared.U64Max) <= 0 {
		checkedExcluded, err = excludedFeeAmountFromIncluded(checkedIncluded, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
		return checkedExcluded, checkedIncluded, false, err
	}
	checkedIncluded = new(big.Int).Set(shared.U64Max)
	checkedExcluded, err = excludedFeeAmountFromIncluded(checkedIncluded, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	return checkedExcluded, checkedIncluded, true, err
}

func GetFeeNumeratorFromExcludedFeeAmount(excludedFeeAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps uint16, feeIncrementBps uint16) (*big.Int, error) {
	excludedReference, err := excludedFeeAmountFromIncluded(referenceAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return nil, err
	}
	if excludedFeeAmount.Cmp(excludedReference) <= 0 {
		return new(big.Int).Set(cliffFeeNumerator), nil
	}

	checkedExcluded, checkedIncluded, overflow, err := checkedRateLimiterAmounts(referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return nil, err
	}
	if excludedFeeAmount.Cmp(checkedExcluded) == 0 {
		return GetFeeNumeratorFromIncludedFeeAmount(checkedIncluded, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	}

	var includedFeeAmount *big.Int
	if excludedFeeAmount.Cmp(checkedExcluded) < 0 {
		i := toNumerator(big.NewInt(int64(feeIncrementBps)))
		x0 := referenceAmount
		d := big.NewInt(shared.FeeDenominator)
		c := cliffFeeNumerator
		two := big.NewInt(2)
		x := i
		y := new(big.Int).Mul(two, d)
		y.Mul(y, x0)
		y.Add(y, new(big.Int).Mul(i, x0))
		y.Sub(y, new(big.Int).Mul(new(big.Int).Mul(two, c), x0))
		z := new(big.Int).Mul(two, excludedFeeAmount)
		z.Mul(z, d)
		z.Mul(z, x0)
		discriminant := new(big.Int).Mul(y, y)
		discriminant.Sub(discriminant, new(big.Int).Mul(big.NewInt(4), new(big.Int).Mul(x, z)))
		if discriminant.Sign() < 0 {
			return nil, errors.New("math overflow")
		}
		sqrtDiscriminant := floorSqrt(discriminant)
		numerator := new(big.Int).Sub(y, sqrtDiscriminant)
		includedFeeAmount = numerator.Div(numerator, new(big.Int).Mul(two, x))
		aPlusOne := new(big.Int).Div(new(big.Int).Set(includedFeeAmount), x0)
		firstExcluded, err := excludedFeeAmountFromIncluded(includedFeeAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
		if err != nil {
			return nil, err
		}
		remainingExcluded := new(big.Int).Sub(excludedFeeAmount, firstExcluded)
		remainingFeeNumerator := new(big.Int).Add(new(big.Int).Set(c), new(big.Int).Mul(i, aPlusOne))
		includedRemaining, _, err := getIncludedFeeAmount(remainingFeeNumerator, remainingExcluded)
		if err != nil {
			return nil, err
		}
		includedFeeAmount = new(big.Int).Add(includedFeeAmount, includedRemaining)
	} else {
		if overflow {
			return nil, errors.New("math overflow")
		}
		remainingExcluded := new(big.Int).Sub(excludedFeeAmount, checkedExcluded)
		maxFeeNumerator := toNumerator(big.NewInt(int64(maxFeeBps)))
		includedRemaining, _, err := getIncludedFeeAmount(maxFeeNumerator, remainingExcluded)
		if err != nil {
			return nil, err
		}
		includedFeeAmount = new(big.Int).Add(includedRemaining, checkedIncluded)
	}

	tradingFee := new(big.Int).Sub(includedFeeAmount, excludedFeeAmount)
	feeNumerator := mulDiv(tradingFee, big.NewInt(shared.FeeDenominator), includedFeeAmount, shared.RoundingUp)
	if feeNumerator.Cmp(cliffFeeNumerator) < 0 {
		return nil, errors.New("fee numerator is less than cliff fee numerator")
	}
	return feeNumerator, nil
}

func GetRateLimiterMinBaseFeeNumerator(cliffFeeNumerator *big.Int) *big.Int {
	return new(big.Int).Set(cliffFeeNumerator)
}

func GetRateLimiterMaxBaseFeeNumerator(inputAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps uint16, feeIncrementBps uint16) (*big.Int, error) {
	return GetFeeNumeratorFromIncludedFeeAmount(inputAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
}

func GetFeeNumeratorFromIncludedFeeAmountUnchecked(inputAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps uint16, feeIncrementBps uint16) *big.Int {
	feeNumerator, err := GetFeeNumeratorFromIncludedFeeAmount(inputAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return big.NewInt(0)
	}
	return feeNumerator
}

func GetFeeNumeratorFromExcludedFeeAmountUnchecked(excludedFeeAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps uint16, feeIncrementBps uint16) *big.Int {
	feeNumerator, err := GetFeeNumeratorFromExcludedFeeAmount(excludedFeeAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
	if err != nil {
		return big.NewInt(0)
	}
	return feeNumerator
}
