package helpers

import (
	"errors"
	"math/big"
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

func MulDiv(x, y, denominator *big.Int, rounding Rounding) (*big.Int, error) {
	if denominator.Sign() == 0 {
		return nil, errors.New("MulDiv: division by zero")
	}
	if denominator.Cmp(big.NewInt(1)) == 0 || x.Sign() == 0 || y.Sign() == 0 {
		return new(big.Int).Mul(x, y), nil
	}
	prod := new(big.Int).Mul(x, y)
	if rounding == RoundingUp {
		numerator := new(big.Int).Add(prod, new(big.Int).Sub(denominator, big.NewInt(1)))
		return new(big.Int).Div(numerator, denominator), nil
	}
	return new(big.Int).Div(prod, denominator), nil
}

func GetInitialLiquidityFromDeltaQuote(quoteAmount, sqrtMinPrice, sqrtPrice *big.Int) (*big.Int, error) {
	priceDelta, err := Sub(sqrtPrice, sqrtMinPrice)
	if err != nil {
		return nil, err
	}
	quoteAmountShifted := new(big.Int).Lsh(quoteAmount, 128)
	return Div(quoteAmountShifted, priceDelta)
}

func GetDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round Rounding) (*big.Int, error) {
	return GetDeltaAmountBaseUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountBaseUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round Rounding) (*big.Int, error) {
	numerator1 := new(big.Int).Set(liquidity)
	numerator2, err := Sub(upperSqrtPrice, lowerSqrtPrice)
	if err != nil {
		return nil, err
	}
	denominator := Mul(lowerSqrtPrice, upperSqrtPrice)
	if denominator.Sign() == 0 {
		return nil, errors.New("Denominator cannot be zero")
	}
	return MulDiv(numerator1, numerator2, denominator, round)
}

func GetDeltaAmountQuoteUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round Rounding) (*big.Int, error) {
	return GetDeltaAmountQuoteUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountQuoteUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round Rounding) (*big.Int, error) {
	deltaSqrtPrice, err := Sub(upperSqrtPrice, lowerSqrtPrice)
	if err != nil {
		return nil, err
	}
	prod := Mul(liquidity, deltaSqrtPrice)
	if round == RoundingUp {
		denominator := new(big.Int).Lsh(big.NewInt(1), Resolution*2)
		numerator := new(big.Int).Add(prod, new(big.Int).Sub(denominator, big.NewInt(1)))
		return Div(numerator, denominator)
	}
	return new(big.Int).Rsh(prod, Resolution*2), nil
}

func GetNextSqrtPriceFromInput(sqrtPrice, liquidity, amountIn *big.Int, baseForQuote bool) (*big.Int, error) {
	if sqrtPrice.Sign() == 0 {
		return nil, errors.New("sqrt_price must be greater than 0")
	}
	if liquidity.Sign() == 0 {
		return nil, errors.New("liquidity must be greater than 0")
	}
	if baseForQuote {
		return GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amountIn)
	}
	return GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amountIn)
}

func GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}
	product := Mul(amount, sqrtPrice)
	if product.Cmp(U128Max) > 0 {
		quotient, err := Div(liquidity, sqrtPrice)
		if err != nil {
			return nil, err
		}
		denominator := new(big.Int).Add(quotient, amount)
		return Div(liquidity, denominator)
	}
	denominator := new(big.Int).Add(liquidity, product)
	return MulDiv(liquidity, sqrtPrice, denominator, RoundingUp)
}

func GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	quotient := new(big.Int).Lsh(amount, Resolution*2)
	q, err := Div(quotient, liquidity)
	if err != nil {
		return nil, err
	}
	return Add(sqrtPrice, q), nil
}
