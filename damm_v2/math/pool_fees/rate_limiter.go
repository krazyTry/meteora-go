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
	return (referenceAmount != nil && referenceAmount.Sign() == 0) && maxLimiterDuration != 0 && maxFeeBps != 0 && feeIncrementBps != 0
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

	feeNumerator := mulDiv(tradingFeeNumerator, big.NewInt(shared.FeeDenominator), inputAmount, shared.RoundingUp)
	if feeNumerator.Cmp(maxFeeNumerator) > 0 {
		return maxFeeNumerator, nil
	}
	return feeNumerator, nil
}

func GetFeeNumeratorFromExcludedFeeAmount(excludedFeeAmount, referenceAmount, cliffFeeNumerator *big.Int, maxFeeBps uint16, feeIncrementBps uint16) (*big.Int, error) {
	includedFeeAmount, _, err := getIncludedFeeAmount(cliffFeeNumerator, excludedFeeAmount)
	if err != nil {
		return nil, err
	}
	return GetFeeNumeratorFromIncludedFeeAmount(includedFeeAmount, referenceAmount, cliffFeeNumerator, maxFeeBps, feeIncrementBps)
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
