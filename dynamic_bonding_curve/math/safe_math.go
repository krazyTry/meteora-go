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
		return nil, errors.New("SafeMath: subtraction overflow")
	}
	return new(big.Int).Sub(a, b), nil
}

func Mul(a, b *big.Int) *big.Int {
	return new(big.Int).Mul(a, b)
}

func Div(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, errors.New("SafeMath: division by zero")
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

// Pow computes base^exponent with Q64 scaling when scaling=true.
func Pow(base, exponent *big.Int, scaling bool) (*big.Int, error) {
	one := new(big.Int).Lsh(big.NewInt(1), dbc.Resolution)

	if exponent.Sign() == 0 {
		return new(big.Int).Set(one), nil
	}
	if base.Sign() == 0 {
		return big.NewInt(0), nil
	}
	if base.Cmp(one) == 0 {
		return new(big.Int).Set(one), nil
	}

	isNegative := exponent.Sign() < 0
	absExp := new(big.Int).Abs(exponent)

	result := new(big.Int).Set(one)
	currentBase := new(big.Int).Set(base)
	exp := new(big.Int).Set(absExp)
	oneInt := big.NewInt(1)

	for exp.Sign() != 0 {
		if new(big.Int).And(exp, oneInt).Cmp(oneInt) == 0 {
			prod := new(big.Int).Mul(result, currentBase)
			result = new(big.Int).Div(prod, one)
		}
		prod := new(big.Int).Mul(currentBase, currentBase)
		currentBase = new(big.Int).Div(prod, one)
		exp = new(big.Int).Rsh(exp, 1)
	}

	if isNegative {
		// result = ONE*ONE / result
		prod := new(big.Int).Mul(one, one)
		result = new(big.Int).Div(prod, result)
	}

	if scaling {
		return result, nil
	}
	return new(big.Int).Div(result, one), nil
}
