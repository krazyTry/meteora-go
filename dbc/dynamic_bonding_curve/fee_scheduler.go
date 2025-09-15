package dynamic_bonding_curve

import (
	"fmt"

	dmath "github.com/krazyTry/meteora-go/decimal_math"
	"github.com/shopspring/decimal"
)

// getFeeNumeratorOnLinearFeeScheduler
func getFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor decimal.Decimal, periodDecimal decimal.Decimal) decimal.Decimal {

	// reduction = period * reductionFactor
	reduction := periodDecimal.Mul(reductionFactor)

	if reduction.Cmp(cliffFeeNumerator) > 0 {
		return N0
	}

	return cliffFeeNumerator.Sub(reduction)
}

// getFeeNumeratorOnExponentialFeeScheduler
func getFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor decimal.Decimal, periodDecimal decimal.Decimal) decimal.Decimal {
	if periodDecimal.IsZero() {
		return cliffFeeNumerator
	}

	if periodDecimal.Equal(N1) {
		return cliffFeeNumerator.Mul(BASIS_POINT_MAX.Sub(reductionFactor)).Div(BASIS_POINT_MAX)
	}

	base := Q64.Sub(dmath.Lsh(reductionFactor, 64).Div(BASIS_POINT_MAX))

	// result = base ^ period
	result := base.Pow(periodDecimal)

	finalFee := cliffFeeNumerator.Mul(dmath.Rsh(result, 64))
	return finalFee
}

// Corresponds to TS's getBaseFeeNumeratorByPeriod
func getBaseFeeNumeratorByPeriod(
	cliffFeeNumerator decimal.Decimal,
	numberOfPeriod decimal.Decimal,
	period decimal.Decimal,
	reductionFactor decimal.Decimal,
	feeSchedulerMode BaseFeeMode,
) (decimal.Decimal, error) {

	// min(period, numberOfPeriod)
	periodValue := period
	if periodValue.Cmp(numberOfPeriod) > 0 {
		periodValue = numberOfPeriod
	}

	if periodValue.Cmp(U16_MAX) > 0 {
		return decimal.Decimal{}, fmt.Errorf("math overflow: periodNumber=%v > %v", periodValue, U16_MAX)
	}

	switch feeSchedulerMode {
	case BaseFeeModeFeeSchedulerLinear:
		return getFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor, periodValue), nil

	case BaseFeeModeFeeSchedulerExponential:
		return getFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor, periodValue), nil
	default:
		return decimal.Decimal{}, fmt.Errorf("invalid fee scheduler mode: %d", feeSchedulerMode)
	}
}

func getBaseFeeNumerator7(
	cliffFeeNumerator decimal.Decimal,
	numberOfPeriod decimal.Decimal,
	periodFrequency decimal.Decimal,
	reductionFactor decimal.Decimal,
	feeSchedulerMode BaseFeeMode,
	currentPoint decimal.Decimal,
	activationPoint decimal.Decimal,
) (decimal.Decimal, error) {

	if periodFrequency.IsZero() {
		return cliffFeeNumerator, nil
	}

	period := currentPoint.Sub(activationPoint).Div(periodFrequency)

	// Call the previous function
	return getBaseFeeNumeratorByPeriod(
		cliffFeeNumerator,
		numberOfPeriod,
		period,
		reductionFactor,
		feeSchedulerMode,
	)
}
