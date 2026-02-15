package math

import (
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
)

const maxFeeBasisPoints = 10_000

type TransferFeeIncludedAmount struct {
	Amount      *big.Int
	TransferFee *big.Int
}

type TransferFeeExcludedAmount struct {
	Amount      *big.Int
	TransferFee *big.Int
}

func calculatePreFeeAmount(transferFeeBasisPoints uint16, maximumFee *big.Int, postFeeAmount *big.Int) *big.Int {
	if postFeeAmount.Sign() == 0 {
		return big.NewInt(0)
	}
	if transferFeeBasisPoints == 0 {
		return new(big.Int).Set(postFeeAmount)
	}
	if transferFeeBasisPoints == maxFeeBasisPoints {
		return new(big.Int).Add(postFeeAmount, maximumFee)
	}
	oneInBps := big.NewInt(maxFeeBasisPoints)
	numerator := new(big.Int).Mul(postFeeAmount, oneInBps)
	denominator := new(big.Int).Sub(oneInBps, big.NewInt(int64(transferFeeBasisPoints)))
	rawPreFee := new(big.Int).Add(numerator, denominator)
	rawPreFee.Sub(rawPreFee, big.NewInt(1))
	rawPreFee.Div(rawPreFee, denominator)

	if new(big.Int).Sub(rawPreFee, postFeeAmount).Cmp(maximumFee) >= 0 {
		return new(big.Int).Add(postFeeAmount, maximumFee)
	}
	return rawPreFee
}

func calculateInverseFee(transferFeeBasisPoints uint16, maximumFee *big.Int, postFeeAmount *big.Int) *big.Int {
	preFeeAmount := calculatePreFeeAmount(transferFeeBasisPoints, maximumFee, postFeeAmount)
	return calculateFee(transferFeeBasisPoints, maximumFee, preFeeAmount)
}

func calculateFee(transferFeeBasisPoints uint16, maximumFee *big.Int, amount *big.Int) *big.Int {
	if transferFeeBasisPoints == 0 || amount.Sign() == 0 {
		return big.NewInt(0)
	}
	if transferFeeBasisPoints == maxFeeBasisPoints {
		return new(big.Int).Set(maximumFee)
	}
	fee := new(big.Int).Mul(amount, big.NewInt(int64(transferFeeBasisPoints)))
	fee.Div(fee, big.NewInt(maxFeeBasisPoints))
	if fee.Cmp(maximumFee) > 0 {
		return new(big.Int).Set(maximumFee)
	}
	return fee
}

func CalculateTransferFeeIncludedAmount(transferFeeExcludedAmount *big.Int, tokenInfo *helpers.TokenInfo) TransferFeeIncludedAmount {
	if transferFeeExcludedAmount.Sign() == 0 {
		return TransferFeeIncludedAmount{Amount: big.NewInt(0), TransferFee: big.NewInt(0)}
	}
	if tokenInfo == nil || !tokenInfo.HasTransferFee {
		return TransferFeeIncludedAmount{Amount: new(big.Int).Set(transferFeeExcludedAmount), TransferFee: big.NewInt(0)}
	}
	maxFee := tokenInfo.MaximumFee
	if maxFee == nil {
		maxFee = big.NewInt(0)
	}
	transferFee := calculateInverseFee(tokenInfo.BasisPoints, maxFee, transferFeeExcludedAmount)
	return TransferFeeIncludedAmount{Amount: new(big.Int).Add(transferFeeExcludedAmount, transferFee), TransferFee: transferFee}
}

func CalculateTransferFeeExcludedAmount(transferFeeIncludedAmount *big.Int, tokenInfo *helpers.TokenInfo) TransferFeeExcludedAmount {
	if tokenInfo == nil || !tokenInfo.HasTransferFee {
		return TransferFeeExcludedAmount{Amount: new(big.Int).Set(transferFeeIncludedAmount), TransferFee: big.NewInt(0)}
	}
	maxFee := tokenInfo.MaximumFee
	if maxFee == nil {
		maxFee = big.NewInt(0)
	}
	fee := calculateFee(tokenInfo.BasisPoints, maxFee, transferFeeIncludedAmount)
	return TransferFeeExcludedAmount{Amount: new(big.Int).Sub(new(big.Int).Set(transferFeeIncludedAmount), fee), TransferFee: fee}
}
