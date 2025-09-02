package dynamic_bonding_curve

import (
	"github.com/shopspring/decimal"
)

// getMaxIndex
func getMaxIndex(cliffFeeNumerator, feeIncrementBps decimal.Decimal) decimal.Decimal {
	// deltaNumerator = MAX_FEE_NUMERATOR - cliffFeeNumerator
	deltaNumerator := decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0).Sub(cliffFeeNumerator)

	// feeIncrementNumerator = feeIncrementBps * FEE_DENOMINATOR / BASIS_POINT_MAX
	feeIncrementNumerator := feeIncrementBps.Mul(decimal.NewFromBigInt(FEE_DENOMINATOR, 0)).Div(decimal.NewFromBigInt(BASIS_POINT_MAX, 0))

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
	i := feeIncrementBps.Mul(decimal.NewFromBigInt(FEE_DENOMINATOR, 0)).Div(decimal.NewFromBigInt(BASIS_POINT_MAX, 0))

	x0 := referenceAmount
	one := decimal.NewFromInt(1)
	two := decimal.NewFromInt(2)

	var tradingFeeNumerator decimal.Decimal

	if a.Cmp(maxIndex) < 0 {
		numerator1 := c.Add(c.Mul(a))
		tmp := i.Mul(a).Mul(a.Add(one)).Div(two)
		numerator1 = numerator1.Add(tmp)

		numerator2 := c.Add(i.Mul(a.Add(one)))

		firstFee := x0.Mul(numerator1)
		secondFee := b.Mul(numerator2)
		tradingFeeNumerator = firstFee.Add(secondFee)
	} else {
		numerator1 := c.Add(c.Mul(maxIndex))
		tmp := i.Mul(maxIndex).Mul(maxIndex.Add(one)).Div(two)
		numerator1 = numerator1.Add(tmp)

		numerator2 := decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0)
		firstFee := x0.Mul(numerator1)

		d := a.Sub(maxIndex)
		leftAmount := d.Mul(x0).Add(b)
		secondFee := leftAmount.Mul(numerator2)

		tradingFeeNumerator = firstFee.Add(secondFee)
	}

	tradingFee := tradingFeeNumerator.Add(decimal.NewFromBigInt(FEE_DENOMINATOR, 0).Sub(one)).Div(decimal.NewFromBigInt(FEE_DENOMINATOR, 0))

	feeNumerator := tradingFee.Mul(decimal.NewFromBigInt(FEE_DENOMINATOR, 0)).Div(inputAmount).Ceil()

	if feeNumerator.Cmp(decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0)) > 0 {
		feeNumerator = decimal.NewFromBigInt(MAX_FEE_NUMERATOR, 0)
	}
	return feeNumerator
}
