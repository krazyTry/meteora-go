package math

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func GetInitialLiquidityFromDeltaQuote(quoteAmount, sqrtMinPrice, sqrtPrice *big.Int) (*big.Int, error) {
	priceDelta, err := Sub(sqrtPrice, sqrtMinPrice)
	if err != nil {
		return nil, err
	}
	quoteAmountShifted := new(big.Int).Lsh(quoteAmount, 128)
	return Div(quoteAmountShifted, priceDelta)
}

func GetInitialLiquidityFromDeltaBase(baseAmount, sqrtMaxPrice, sqrtPrice *big.Int) (*big.Int, error) {
	priceDelta, err := Sub(sqrtMaxPrice, sqrtPrice)
	if err != nil {
		return nil, err
	}
	prod := Mul(Mul(baseAmount, sqrtPrice), sqrtMaxPrice)
	return Div(prod, priceDelta)
}

func GetDeltaAmountBaseUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	return GetDeltaAmountBaseUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountBaseUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	return GetDeltaAmountBaseUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountBaseUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	numerator1 := new(big.Int).Set(liquidity)
	// numerator2 = upper - lower
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

func GetDeltaAmountQuoteUnsigned(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	return GetDeltaAmountQuoteUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountQuoteUnsigned256(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	return GetDeltaAmountQuoteUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity, round)
}

func GetDeltaAmountQuoteUnsignedUnchecked(lowerSqrtPrice, upperSqrtPrice, liquidity *big.Int, round dbc.Rounding) (*big.Int, error) {
	deltaSqrtPrice, err := Sub(upperSqrtPrice, lowerSqrtPrice)
	if err != nil {
		return nil, err
	}
	prod := Mul(liquidity, deltaSqrtPrice)
	if round == dbc.RoundingUp {
		denominator := new(big.Int).Lsh(big.NewInt(1), dbc.Resolution*2)
		numerator := new(big.Int).Add(prod, new(big.Int).Sub(denominator, big.NewInt(1)))
		return Div(numerator, denominator)
	}
	return new(big.Int).Rsh(prod, dbc.Resolution*2), nil
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

func GetNextSqrtPriceFromOutput(sqrtPrice, liquidity, amountOut *big.Int, baseForQuote bool) (*big.Int, error) {
	if sqrtPrice.Sign() == 0 {
		return nil, errors.New("sqrt_price must be greater than 0")
	}
	if liquidity.Sign() == 0 {
		return nil, errors.New("liquidity must be greater than 0")
	}
	if baseForQuote {
		return GetNextSqrtPriceFromQuoteAmountOutRoundingDown(sqrtPrice, liquidity, amountOut)
	}
	return GetNextSqrtPriceFromBaseAmountOutRoundingUp(sqrtPrice, liquidity, amountOut)
}

func GetNextSqrtPriceFromQuoteAmountOutRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	qAmount := new(big.Int).Lsh(amount, 128)
	numerator := new(big.Int).Add(qAmount, new(big.Int).Sub(liquidity, big.NewInt(1)))
	quotient, err := Div(numerator, liquidity)
	if err != nil {
		return nil, err
	}
	return Sub(sqrtPrice, quotient)
}

func GetNextSqrtPriceFromBaseAmountOutRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}
	product := Mul(amount, sqrtPrice)
	denominator, err := Sub(liquidity, product)
	if err != nil || denominator.Sign() <= 0 {
		return nil, errors.New("Invalid denominator: liquidity must be greater than amount * sqrt_price")
	}
	return MulDiv(liquidity, sqrtPrice, denominator, dbc.RoundingUp)
}

func GetNextSqrtPriceFromBaseAmountInRoundingUp(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	if amount.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice), nil
	}
	product := Mul(amount, sqrtPrice)
	if product.Cmp(dbc.U128Max) > 0 {
		quotient, err := Div(liquidity, sqrtPrice)
		if err != nil {
			return nil, err
		}
		denominator := new(big.Int).Add(quotient, amount)
		return Div(liquidity, denominator)
	}
	denominator := new(big.Int).Add(liquidity, product)
	return MulDiv(liquidity, sqrtPrice, denominator, dbc.RoundingUp)
}

func GetNextSqrtPriceFromQuoteAmountInRoundingDown(sqrtPrice, liquidity, amount *big.Int) (*big.Int, error) {
	quotient := new(big.Int).Lsh(amount, dbc.Resolution*2)
	q, err := Div(quotient, liquidity)
	if err != nil {
		return nil, err
	}
	return Add(sqrtPrice, q), nil
}
