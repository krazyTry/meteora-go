package pool_fees

import (
	"errors"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func IsDynamicFeeEnabled(dynamicFee dbc.DynamicFeeConfig) bool {
	return dynamicFee.Initialized != 0
}

func GetVariableFeeNumerator(dynamicFee dbc.DynamicFeeConfig, volatilityTracker dbc.VolatilityTracker) *big.Int {
	if !IsDynamicFeeEnabled(dynamicFee) {
		return big.NewInt(0)
	}
	volatilityTimesBinStep := new(big.Int).Mul(volatilityTracker.VolatilityAccumulator.BigInt(), big.NewInt(int64(dynamicFee.BinStep)))
	squareVfaBin := new(big.Int).Mul(volatilityTimesBinStep, volatilityTimesBinStep)
	vFee := new(big.Int).Mul(squareVfaBin, big.NewInt(int64(dynamicFee.VariableFeeControl)))
	scaledVFee, _ := div(new(big.Int).Add(vFee, dbc.DynamicFeeRoundingOffset), dbc.DynamicFeeScalingFactor)
	return scaledVFee
}

func div(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, errors.New("division by zero")
	}
	return new(big.Int).Div(a, b), nil
}
