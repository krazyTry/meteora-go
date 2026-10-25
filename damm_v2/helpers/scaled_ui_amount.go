package helpers

import (
	"errors"

	"github.com/shopspring/decimal"
)

// TokenScale converts raw amounts and prices into the values a wallet displays
// for Token-2022 ScaledUiAmount mints. Swap, deposit, and withdraw math stay on raw amounts.
type TokenScale struct {
	BaseMultiplier  decimal.Decimal
	QuoteMultiplier decimal.Decimal
	PriceFactor     decimal.Decimal
}

func DefaultTokenScale() TokenScale {
	return TokenScale{
		BaseMultiplier:  decimal.NewFromInt(1),
		QuoteMultiplier: decimal.NewFromInt(1),
		PriceFactor:     decimal.NewFromInt(1),
	}
}

func TokenScaleFromMultipliers(baseMultiplier, quoteMultiplier decimal.Decimal) (TokenScale, error) {
	if err := assertValidMultiplier(baseMultiplier, "base"); err != nil {
		return TokenScale{}, err
	}
	if err := assertValidMultiplier(quoteMultiplier, "quote"); err != nil {
		return TokenScale{}, err
	}
	return TokenScale{
		BaseMultiplier:  baseMultiplier,
		QuoteMultiplier: quoteMultiplier,
		PriceFactor:     quoteMultiplier.Div(baseMultiplier),
	}, nil
}

func (s TokenScale) ScalePrice(price decimal.Decimal) decimal.Decimal {
	return price.Mul(s.PriceFactor)
}

func (s TokenScale) UnscalePrice(price decimal.Decimal) decimal.Decimal {
	return price.Div(s.PriceFactor)
}

func (s TokenScale) ScaleAmount(amount decimal.Decimal, isBaseToken bool) decimal.Decimal {
	if isBaseToken {
		return amount.Mul(s.BaseMultiplier)
	}
	return amount.Mul(s.QuoteMultiplier)
}

func (s TokenScale) UnscaleAmount(amount decimal.Decimal, isBaseToken bool) decimal.Decimal {
	if isBaseToken {
		return amount.Div(s.BaseMultiplier)
	}
	return amount.Div(s.QuoteMultiplier)
}

func assertValidMultiplier(multiplier decimal.Decimal, label string) error {
	if !multiplier.IsPositive() {
		return errors.New("invalid ScaledUiAmount multiplier (" + label + ")")
	}
	return nil
}
