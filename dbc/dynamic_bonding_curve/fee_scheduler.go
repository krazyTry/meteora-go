package dynamic_bonding_curve

import (
	"math/big"

	"github.com/shopspring/decimal"
)

// getFeeNumeratorOnLinearFeeScheduler
func getFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor decimal.Decimal, periodDecimal decimal.Decimal) decimal.Decimal {

	// reduction = period * reductionFactor
	reduction := periodDecimal.Mul(reductionFactor)

	if reduction.Cmp(cliffFeeNumerator) > 0 {
		return decimal.Zero
	}

	return cliffFeeNumerator.Sub(reduction)
}

// getFeeNumeratorOnExponentialFeeScheduler
func getFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor decimal.Decimal, periodDecimal decimal.Decimal) decimal.Decimal {
	if periodDecimal.IsZero() {
		return cliffFeeNumerator
	}

	basisPointMax := decimal.NewFromBigInt(BASIS_POINT_MAX, 0)
	oneQ64 := decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 64), 0) // 1 << 64

	if periodDecimal.Equal(decimal.NewFromInt(1)) {
		tmp := basisPointMax.Sub(reductionFactor) // basisPointMax - reductionFactor
		tmp = tmp.Mul(cliffFeeNumerator)          // cliffFeeNumerator * (basisPointMax - reductionFactor)
		tmp = tmp.Div(basisPointMax)              // / basisPointMax
		return tmp
	}

	// base = ONE_Q64 - (reductionFactor << 64) / BASIS_POINT_MAX
	reductionFactorScaled := reductionFactor.Mul(oneQ64).Div(basisPointMax)
	base := oneQ64.Sub(reductionFactorScaled)

	// result = base ^ period
	result := base.Pow(periodDecimal)

	// final fee: cliffFeeNumerator * result >> 64 → * result / ONE_Q64
	finalFee := cliffFeeNumerator.Mul(result).Div(oneQ64)
	return finalFee
}
