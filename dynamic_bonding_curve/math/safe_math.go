package math

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func Add(a, b *big.Int) *big.Int {
	return new(big.Int).Add(a, b)
}

func Sub(a, b *big.Int) (*big.Int, error) {
	if b.Cmp(a) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return new(big.Int).Sub(a, b), nil
}

func Mul(a, b *big.Int) *big.Int {
	return new(big.Int).Mul(a, b)
}

func Div(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return new(big.Int).Div(a, b), nil
}

func Mod(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, errors.New("SafeMath: modulo by zero")
	}
	return new(big.Int).Mod(a, b), nil
}

func Shl(a *big.Int, b uint) *big.Int {
	return new(big.Int).Lsh(a, b)
}

func Shr(a *big.Int, b uint) *big.Int {
	return new(big.Int).Rsh(a, b)
}

const maxExponential = 0x80000

func mulQ64(left, right *big.Int) (*big.Int, error) {
	product := new(big.Int).Mul(left, right)
	if product.Cmp(dbc.U128Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	return new(big.Int).Rsh(product, dbc.Resolution), nil
}

// Pow is the program's Q64 power. A base at or above 1 is inverted through
// u128::MAX so each multiplication stays inside u128. scaling=false returns
// the integer part.
func Pow(base, exponent *big.Int, scaling bool) (*big.Int, error) {
	one := new(big.Int).Lsh(big.NewInt(1), dbc.Resolution)
	i32Min := big.NewInt(-2147483648)

	if exponent.Cmp(i32Min) == 0 || base.Sign() < 0 || base.Cmp(dbc.U128Max) > 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	if exponent.Sign() == 0 {
		return new(big.Int).Set(one), nil
	}

	invert := exponent.Sign() < 0
	exp := new(big.Int).Abs(exponent)
	if exp.Cmp(big.NewInt(maxExponential)) >= 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}

	squaredBase := new(big.Int).Set(base)
	result := new(big.Int).Set(one)
	if squaredBase.Cmp(result) >= 0 {
		squaredBase = new(big.Int).Div(dbc.U128Max, squaredBase)
		invert = !invert
	}

	remaining := new(big.Int).Set(exp)
	for remaining.Sign() != 0 {
		if remaining.Bit(0) == 1 {
			next, err := mulQ64(result, squaredBase)
			if err != nil {
				return nil, err
			}
			result = next
		}
		nextBase, err := mulQ64(squaredBase, squaredBase)
		if err != nil {
			return nil, err
		}
		squaredBase = nextBase
		remaining.Rsh(remaining, 1)
	}

	if result.Sign() == 0 {
		return nil, errors.New(dbc.ErrMathOverflow)
	}
	if invert {
		result = new(big.Int).Div(dbc.U128Max, result)
	}
	if scaling {
		return result, nil
	}
	return new(big.Int).Rsh(result, dbc.Resolution), nil
}
