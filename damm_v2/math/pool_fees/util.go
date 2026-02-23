package pool_fees

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func mulDiv(x, y, denominator *big.Int, rounding shared.Rounding) *big.Int {
	if denominator.Sign() == 0 {
		return big.NewInt(0)
	}
	mul := new(big.Int).Mul(x, y)
	div, mod := new(big.Int).QuoRem(mul, denominator, new(big.Int))
	if rounding == shared.RoundingUp && mod.Sign() != 0 {
		return div.Add(div, big.NewInt(1))
	}
	return div
}

func toNumerator(bps *big.Int) *big.Int {
	return mulDiv(bps, big.NewInt(shared.FeeDenominator), big.NewInt(shared.BasisPointMax), shared.RoundingDown)
}

func pow(base, exp *big.Int) *big.Int {
	if exp.Sign() == 0 {
		return new(big.Int).Set(shared.OneQ64)
	}
	invert := exp.Sign() < 0
	absExp := new(big.Int).Abs(exp)
	if absExp.Cmp(shared.MaxExponential) > 0 {
		return big.NewInt(0)
	}
	squaredBase := new(big.Int).Set(base)
	result := new(big.Int).Set(shared.OneQ64)
	if squaredBase.Cmp(result) >= 0 {
		squaredBase = new(big.Int).Div(shared.U128Max, squaredBase)
		invert = !invert
	}
	for bit := uint(0); bit <= 18; bit++ {
		if absExp.Bit(int(bit)) == 1 {
			result.Mul(result, squaredBase)
			result.Rsh(result, shared.ScaleOffset)
		}
		squaredBase.Mul(squaredBase, squaredBase)
		squaredBase.Rsh(squaredBase, shared.ScaleOffset)
	}
	if result.Sign() == 0 {
		return big.NewInt(0)
	}
	if invert {
		result = new(big.Int).Div(shared.U128Max, result)
	}
	return result
}

func getIncludedFeeAmount(tradeFeeNumerator, excludedFeeAmount *big.Int) (*big.Int, *big.Int, error) {
	denominator := new(big.Int).Sub(big.NewInt(shared.FeeDenominator), tradeFeeNumerator)
	if denominator.Sign() <= 0 {
		return nil, nil, errors.New("invalid fee numerator")
	}
	included := mulDiv(excludedFeeAmount, big.NewInt(shared.FeeDenominator), denominator, shared.RoundingUp)
	feeAmount := new(big.Int).Sub(included, excludedFeeAmount)
	return included, feeAmount, nil
}

func getMaxFeeNumerator(poolVersion shared.PoolVersion) *big.Int {
	switch poolVersion {
	case shared.PoolVersionV0:
		return big.NewInt(shared.MaxFeeNumeratorV0)
	case shared.PoolVersionV1:
		return big.NewInt(shared.MaxFeeNumeratorV1)
	default:
		return big.NewInt(0)
	}
}

func getMaxFeeBps(poolVersion shared.PoolVersion) uint16 {
	switch poolVersion {
	case shared.PoolVersionV0:
		return shared.MaxFeeBpsV0
	case shared.PoolVersionV1:
		return shared.MaxFeeBpsV1
	default:
		return 0
	}
}
