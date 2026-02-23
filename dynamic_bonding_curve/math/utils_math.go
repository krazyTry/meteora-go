package math

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func MulDiv(x, y, denominator *big.Int, rounding dbc.Rounding) (*big.Int, error) {
	if denominator.Sign() == 0 {
		return nil, errors.New("MulDiv: division by zero")
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

func MulShr(x, y *big.Int, offset uint) *big.Int {
	if offset == 0 || x.Sign() == 0 || y.Sign() == 0 {
		return new(big.Int).Mul(x, y)
	}
	prod := new(big.Int).Mul(x, y)
	return new(big.Int).Rsh(prod, offset)
}

func Sqrt(value *big.Int) *big.Int {
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
