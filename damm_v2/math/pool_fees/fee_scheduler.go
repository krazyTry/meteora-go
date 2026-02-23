package pool_fees

import (
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

func GetFeeNumeratorOnLinearFeeScheduler(cliffFeeNumerator, reductionFactor *big.Int, period uint16) *big.Int {
	reduction := new(big.Int).Mul(big.NewInt(int64(period)), reductionFactor)
	return new(big.Int).Sub(cliffFeeNumerator, reduction)
}

func GetFeeNumeratorOnExponentialFeeScheduler(cliffFeeNumerator, reductionFactor *big.Int, period uint16) *big.Int {
	if period == 0 {
		return new(big.Int).Set(cliffFeeNumerator)
	}
	bps := new(big.Int).Lsh(reductionFactor, 64)
	bps.Div(bps, big.NewInt(shared.BasisPointMax))
	base := new(big.Int).Sub(shared.OneQ64, bps)
	result := pow(base, big.NewInt(int64(period)))
	return new(big.Int).Div(new(big.Int).Mul(cliffFeeNumerator, result), shared.OneQ64)
}

func GetMaxBaseFeeNumerator(cliffFeeNumerator *big.Int) *big.Int {
	return new(big.Int).Set(cliffFeeNumerator)
}
