package dynamic_bonding_curve

import (
	"github.com/shopspring/decimal"
)

// getMaxIndex
func getMaxIndex(cliffFeeNumerator, feeIncrementBps decimal.Decimal) decimal.Decimal {
	// deltaNumerator = MAX_FEE_NUMERATOR - cliffFeeNumerator
	deltaNumerator := MAX_FEE_NUMERATOR.Sub(cliffFeeNumerator)

	// feeIncrementNumerator = feeIncrementBps * FEE_DENOMINATOR / BASIS_POINT_MAX
	feeIncrementNumerator := feeIncrementBps.Mul(FEE_DENOMINATOR).Div(BASIS_POINT_MAX)

	return deltaNumerator.Div(feeIncrementNumerator)
}

// getFeeNumeratorOnRateLimiter
func getFeeNumeratorOnRateLimiter(
	cliffFeeNumerator, referenceAmount, feeIncrementBps, inputAmount decimal.Decimal,
) decimal.Decimal {
	if inputAmount.Cmp(referenceAmount) <= 0 {
		return cliffFeeNumerator
	}

	c := cliffFeeNumerator
	diff := inputAmount.Sub(referenceAmount)
	a := diff.Div(referenceAmount).Floor()
	b := diff.Mod(referenceAmount)

	maxIndex := getMaxIndex(cliffFeeNumerator, feeIncrementBps)
	i := feeIncrementBps.Mul(FEE_DENOMINATOR).Div(BASIS_POINT_MAX)

	x0 := referenceAmount

	var tradingFeeNumerator decimal.Decimal

	if a.Cmp(maxIndex) < 0 {
		numerator1 := c.Add(c.Mul(a))
		tmp := i.Mul(a).Mul(a.Add(N1)).Div(N2)
		numerator1 = numerator1.Add(tmp)

		numerator2 := c.Add(i.Mul(a.Add(N1)))

		firstFee := x0.Mul(numerator1)
		secondFee := b.Mul(numerator2)
		tradingFeeNumerator = firstFee.Add(secondFee)
	} else {
		numerator1 := c.Add(c.Mul(maxIndex))
		tmp := i.Mul(maxIndex).Mul(maxIndex.Add(N1)).Div(N2)
		numerator1 = numerator1.Add(tmp)

		numerator2 := MAX_FEE_NUMERATOR
		firstFee := x0.Mul(numerator1)

		d := a.Sub(maxIndex)
		leftAmount := d.Mul(x0).Add(b)
		secondFee := leftAmount.Mul(numerator2)

		tradingFeeNumerator = firstFee.Add(secondFee)
	}

	tradingFee := tradingFeeNumerator.Add(FEE_DENOMINATOR.Sub(N1)).Div(FEE_DENOMINATOR)

	feeNumerator := tradingFee.Mul(FEE_DENOMINATOR).Div(inputAmount).Ceil()

	// if feeNumerator.Cmp(MAX_FEE_NUMERATOR) > 0 {
	// 	feeNumerator = MAX_FEE_NUMERATOR
	// }
	// return feeNumerator

	return decimal.Max(feeNumerator, MAX_FEE_NUMERATOR)
}
