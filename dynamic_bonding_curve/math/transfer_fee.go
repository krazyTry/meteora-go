package math

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

// BaseTransferFee is a DBC base mint fee. The config stores only the basis points, so the cap is u64::MAX.
// Zero basis points means there is no fee.
func BaseTransferFee(transferFeeBasisPoints uint16) *dbc.EpochTransferFee {
	if transferFeeBasisPoints == 0 {
		return nil
	}
	return &dbc.EpochTransferFee{
		TransferFeeBasisPoints: transferFeeBasisPoints,
		MaximumFee:             new(big.Int).Set(dbc.U64Max),
	}
}

func configBaseTransferFee(config *dbc.PoolConfig) *dbc.EpochTransferFee {
	if config.TransferFeeFlag != 1 {
		return nil
	}
	return &dbc.EpochTransferFee{
		TransferFeeBasisPoints: config.TransferFeeBasisPoints,
		MaximumFee:             new(big.Int).Set(dbc.U64Max),
	}
}

// IsZeroTransferFee reports whether the fee charges nothing.
func IsZeroTransferFee(transferFee *dbc.EpochTransferFee) bool {
	if transferFee == nil || transferFee.TransferFeeBasisPoints == 0 {
		return true
	}
	return transferFee.MaximumFee == nil || transferFee.MaximumFee.Sign() == 0
}

// ResolveSwapTransferFees returns the transfer fee on the input and output mints.
func ResolveSwapTransferFees(config *dbc.PoolConfig, swapBaseForQuote bool, fees *dbc.QuoteTransferFees) (input, output *dbc.EpochTransferFee, err error) {
	var base *dbc.EpochTransferFee
	switch {
	case fees != nil && fees.Base != nil:
		base = fees.Base
	case fees != nil && fees.BaseTransferFeeBasisPoints != nil:
		base = BaseTransferFee(*fees.BaseTransferFeeBasisPoints)
	default:
		base = configBaseTransferFee(config)
	}

	var quote *dbc.EpochTransferFee
	if fees != nil && fees.Quote != nil {
		quote = fees.Quote
	} else if config.QuoteTokenFlag == uint8(dbc.TokenTypeToken2022) {
		return nil, nil, errors.New("quoteMint and currentEpoch are required for a Token-2022 quote mint, use getSwapQuoteTransferFees")
	}

	if swapBaseForQuote {
		return base, quote, nil
	}
	return quote, base, nil
}

func calculateRawTransferFee(transferFee *dbc.EpochTransferFee, includedAmount *big.Int) *big.Int {
	if IsZeroTransferFee(transferFee) || includedAmount.Sign() == 0 {
		return big.NewInt(0)
	}
	numerator := new(big.Int).Mul(includedAmount, big.NewInt(int64(transferFee.TransferFeeBasisPoints)))
	raw := new(big.Int).Add(numerator, big.NewInt(dbc.MaxBasisPoint-1))
	raw.Div(raw, big.NewInt(dbc.MaxBasisPoint))
	if raw.Cmp(transferFee.MaximumFee) > 0 {
		return new(big.Int).Set(transferFee.MaximumFee)
	}
	return raw
}

// CalculateTransferFeeExcludedAmount is the amount received after the transfer fee, and the fee.
func CalculateTransferFeeExcludedAmount(transferFee *dbc.EpochTransferFee, includedAmount *big.Int) (amount, fee *big.Int) {
	if IsZeroTransferFee(transferFee) || includedAmount.Sign() == 0 {
		return new(big.Int).Set(includedAmount), big.NewInt(0)
	}
	fee = calculateRawTransferFee(transferFee, includedAmount)
	return new(big.Int).Sub(includedAmount, fee), fee
}

func calculatePreFeeAmount(transferFee *dbc.EpochTransferFee, excludedAmount *big.Int) *big.Int {
	if excludedAmount.Sign() == 0 || IsZeroTransferFee(transferFee) {
		return new(big.Int).Set(excludedAmount)
	}
	if transferFee.TransferFeeBasisPoints == dbc.MaxBasisPoint {
		return new(big.Int).Add(excludedAmount, transferFee.MaximumFee)
	}
	denominator := big.NewInt(int64(dbc.MaxBasisPoint - transferFee.TransferFeeBasisPoints))
	raw := new(big.Int).Mul(excludedAmount, big.NewInt(dbc.MaxBasisPoint))
	raw.Add(raw, denominator)
	raw.Sub(raw, big.NewInt(1))
	raw.Div(raw, denominator)
	if new(big.Int).Sub(raw, excludedAmount).Cmp(transferFee.MaximumFee) >= 0 {
		return new(big.Int).Add(excludedAmount, transferFee.MaximumFee)
	}
	return raw
}

// CalculateTransferFeeIncludedAmount is the amount to send so excludedAmount arrives after the transfer fee.
func CalculateTransferFeeIncludedAmount(transferFee *dbc.EpochTransferFee, excludedAmount *big.Int) (amount, fee *big.Int, err error) {
	if excludedAmount.Sign() == 0 {
		return big.NewInt(0), big.NewInt(0), nil
	}
	if IsZeroTransferFee(transferFee) {
		return new(big.Int).Set(excludedAmount), big.NewInt(0), nil
	}
	preFeeAmount := calculatePreFeeAmount(transferFee, excludedAmount)
	if preFeeAmount.Cmp(dbc.U64Max) > 0 {
		return nil, nil, errors.New(dbc.ErrMathOverflow)
	}
	_, fee = CalculateTransferFeeExcludedAmount(transferFee, preFeeAmount)
	included := new(big.Int).Add(excludedAmount, fee)
	_, verification := CalculateTransferFeeExcludedAmount(transferFee, included)
	if fee.Cmp(verification) != 0 {
		return nil, nil, errors.New(dbc.ErrFeeInverseIsIncorrect)
	}
	return included, fee, nil
}
