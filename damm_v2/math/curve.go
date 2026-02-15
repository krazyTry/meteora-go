package math

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func GetNextSqrtPriceFromAmountInBRoundingDown(sqrtPrice, liquidity, amount *big.Int) *big.Int {
	quotient := new(big.Int).Lsh(amount, shared.ScaleOffset*2)
	quotient.Div(quotient, liquidity)
	return new(big.Int).Add(sqrtPrice, quotient)
}

func GetNextSqrtPriceFromAmountOutBRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	numerator := new(big.Int).Lsh(amount, shared.ScaleOffset*2)
	quotient := new(big.Int).Add(numerator, liquidity)
	quotient.Sub(quotient, big.NewInt(1))
	quotient.Div(quotient, liquidity)
	result := new(big.Int).Sub(sqrtPrice, quotient)
	if result.Sign() < 0 {
		return nil, errors.New("sqrt price cannot be negative")
	}
	return result, nil
}

func GetNextSqrtPriceFromAmountInARoundingUp(sqrtPrice, liquidity, amount *big.Int) *big.Int {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice)
	}
	product := new(big.Int).Mul(amount, sqrtPrice)
	denominator := new(big.Int).Add(liquidity, product)
	return MulDiv(liquidity, sqrtPrice, denominator, shared.RoundingUp)
}

func GetNextSqrtPriceFromAmountOutARoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}
	product := new(big.Int).Mul(amount, sqrtPrice)
	denominator := new(big.Int).Sub(liquidity, product)
	if denominator.Sign() <= 0 {
		return nil, errors.New("math overflow: denominator is zero or negative")
	}
	return MulDiv(liquidity, sqrtPrice, denominator, shared.RoundingUp), nil
}

func GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountOut *big.Int, aForB bool) (*big.Int, error) {
	if sqrtPrice.Sign() <= 0 {
		return nil, errors.New("sqrtPrice must be greater than 0")
	}
	if liquidity.Sign() <= 0 {
		return nil, errors.New("liquidity must be greater than 0")
	}
	if aForB {
		return GetNextSqrtPriceFromAmountOutBRoundingDown(sqrtPrice, liquidity, amountOut)
	}
	return GetNextSqrtPriceFromAmountOutARoundingUp(sqrtPrice, liquidity, amountOut)
}

func GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn *big.Int, aForB bool) (*big.Int, error) {
	if sqrtPrice.Sign() <= 0 {
		return nil, errors.New("sqrtPrice must be greater than 0")
	}
	if liquidity.Sign() <= 0 {
		return nil, errors.New("liquidity must be greater than 0")
	}
	if aForB {
		return GetNextSqrtPriceFromAmountInARoundingUp(sqrtPrice, liquidity, amountIn), nil
	}
	return GetNextSqrtPriceFromAmountInBRoundingDown(sqrtPrice, liquidity, amountIn), nil
}

func GetAmountBFromLiquidityDelta(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, rounding shared.Rounding) *big.Int {
	return getDeltaAmountBUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity, rounding)
}

func getDeltaAmountBUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, rounding shared.Rounding) *big.Int {
	deltaSqrtPrice := new(big.Int).Sub(upperSqrtPrice, lowerSqrtPrice)
	prod := new(big.Int).Mul(liquidity, deltaSqrtPrice)
	shift := uint(shared.ScaleOffset * 2)
	if rounding == shared.RoundingUp {
		denominator := new(big.Int).Lsh(big.NewInt(1), shift)
		result := new(big.Int).Add(prod, new(big.Int).Sub(denominator, big.NewInt(1)))
		return result.Div(result, denominator)
	}
	return prod.Rsh(prod, shift)
}

func GetAmountAFromLiquidityDelta(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, rounding shared.Rounding) *big.Int {
	return getDeltaAmountAUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity, rounding)
}

func getDeltaAmountAUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, rounding shared.Rounding) *big.Int {
	numerator1 := liquidity
	numerator2 := new(big.Int).Sub(upperSqrtPrice, lowerSqrtPrice)
	denominator := new(big.Int).Mul(lowerSqrtPrice, upperSqrtPrice)
	if denominator.Sign() <= 0 {
		panic("denominator must be greater than zero")
	}
	return MulDiv(numerator1, numerator2, denominator, rounding)
}

func GetLiquidityDeltaFromAmountA(amountA, lowerSqrtPrice, upperSqrtPrice *big.Int) *big.Int {
	product := new(big.Int).Mul(amountA, lowerSqrtPrice)
	product.Mul(product, upperSqrtPrice)
	denominator := new(big.Int).Sub(upperSqrtPrice, lowerSqrtPrice)
	return product.Div(product, denominator)
}

func GetLiquidityDeltaFromAmountB(amountB, lowerSqrtPrice, upperSqrtPrice *big.Int) *big.Int {
	denominator := new(big.Int).Sub(upperSqrtPrice, lowerSqrtPrice)
	product := new(big.Int).Lsh(amountB, 128)
	return product.Div(product, denominator)
}
