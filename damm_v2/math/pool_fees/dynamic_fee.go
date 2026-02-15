package pool_fees

import (
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func IsDynamicFeeEnabled(dynamicFee dammv2gen.DynamicFeeStruct) bool {
	return dynamicFee.Initialized != 0
}

func GetDynamicFeeNumerator(volatilityAccumulator, binStep, variableFeeControl *big.Int) *big.Int {
	squareVfaBin := new(big.Int).Mul(volatilityAccumulator, binStep)
	squareVfaBin.Mul(squareVfaBin, squareVfaBin)
	vFee := new(big.Int).Mul(variableFeeControl, squareVfaBin)
	vFee.Add(vFee, shared.DynamicFeeRoundingOffset)
	return vFee.Div(vFee, shared.DynamicFeeScalingFactor)
}
