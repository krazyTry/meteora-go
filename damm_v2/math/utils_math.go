package math

import (
	"math/big"

	"github.com/shopspring/decimal"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func MulDiv(x, y, denominator *big.Int, rounding shared.Rounding) *big.Int {
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

func Q64ToDecimal(num *big.Int, decimalPlaces int32) decimal.Decimal {
	if num == nil {
		return decimal.Zero
	}
	out := decimal.NewFromBigInt(num, 0).Div(decimal.NewFromBigInt(
		new(big.Int).Lsh(
			decimal.NewFromInt(1).BigInt(),
			64,
		),
		0,
	))
	if decimalPlaces >= 0 {
		return out.Round(decimalPlaces)
	}
	return out
}

func DecimalToQ64(num decimal.Decimal) *big.Int {
	v := num.Mul(decimal.NewFromBigInt(
		new(big.Int).Lsh(
			decimal.NewFromInt(1).BigInt(),
			64,
		),
		0,
	)).Floor()
	return v.BigInt()
}

func Sqrt(value *big.Int) *big.Int {
	if value == nil || value.Sign() == 0 {
		return big.NewInt(0)
	}
	if value.Cmp(big.NewInt(1)) == 0 {
		return big.NewInt(1)
	}

	x := new(big.Int).Set(value)
	y := new(big.Int).Add(value, big.NewInt(1))
	y.Div(y, big.NewInt(2))

	for y.Cmp(x) < 0 {
		x.Set(y)
		y = new(big.Int).Add(x, new(big.Int).Div(value, x))
		y.Div(y, big.NewInt(2))
	}

	return x
}

func Pow(base, exp *big.Int) *big.Int {
	if exp == nil {
		return new(big.Int).Set(shared.OneQ64)
	}
	invert := exp.Sign() < 0
	if exp.Sign() == 0 {
		return new(big.Int).Set(shared.OneQ64)
	}
	absExp := new(big.Int).Abs(exp)
	if absExp.Cmp(shared.MaxExponential) > 0 {
		return big.NewInt(0)
	}

	squaredBase := new(big.Int).Set(base)
	result := new(big.Int).Set(shared.OneQ64)
	if squaredBase.Cmp(result) >= 0 {
		squaredBase = new(big.Int).Div(shared.MaxU128, squaredBase)
		invert = !invert
	}

	checkBit := func(bit uint) {
		if absExp.Bit(int(bit)) == 1 {
			result.Mul(result, squaredBase)
			result.Rsh(result, shared.ScaleOffset)
		}
		squaredBase.Mul(squaredBase, squaredBase)
		squaredBase.Rsh(squaredBase, shared.ScaleOffset)
	}

	for bit := uint(0); bit <= 18; bit++ {
		checkBit(bit)
	}

	if result.Sign() == 0 {
		return big.NewInt(0)
	}
	if invert {
		result = new(big.Int).Div(shared.MaxU128, result)
	}
	return result
}
