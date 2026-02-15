package helpers

import (
	"math/big"
)

const (

	// AccountKeyClaimFeeOperator is the account key for claim fee operator accounts
	AccountKeyClaimFeeOperator = "ClaimFeeOperator"
	// AccountKeyConfig is the account key for configuration accounts
	AccountKeyConfig = "Config"
	// AccountKeyPool is the account key for liquidity pool accounts
	AccountKeyPool = "Pool"
	// AccountKeyPosition is the account key for position accounts
	AccountKeyPosition = "Position"
	// AccountKeyTokenBadge is the account key for token badge accounts
	AccountKeyTokenBadge = "TokenBadge"
	// AccountKeyVesting is the account key for vesting accounts
	AccountKeyVesting = "Vesting"

	LiquidityScale = 128
	ScaleOffset    = 64

	BasisPointMax  = 10_000
	FeeDenominator = 1_000_000_000

	MinFeeBps       = 1
	MinFeeNumerator = 100_000

	MaxFeeBpsV0       = 5000
	MaxFeeNumeratorV0 = 500_000_000

	MaxFeeBpsV1       = 9900
	MaxFeeNumeratorV1 = 990_000_000

	DynamicFeeFilterPeriodDefault    = 10
	DynamicFeeDecayPeriodDefault     = 120
	DynamicFeeReductionFactorDefault = 5000
	BinStepBpsDefault                = 1
	MaxPriceChangeBpsDefault         = 1500

	PoolStatusEnable        = 0
	ActivationTypeSlot      = 0
	ActivationTypeTimestamp = 1
	PoolVersionV0           = 0
	PoolVersionV1           = 1

	BaseFeeModeFeeTimeSchedulerLinear      = 0
	BaseFeeModeFeeTimeSchedulerExponential = 1
	BaseFeeModeRateLimiter                 = 2
	BaseFeeModeFeeMarketCapSchedulerLinear = 3
	BaseFeeModeFeeMarketCapSchedulerExp    = 4

	SwapModeExactIn     = 0
	SwapModePartialFill = 1
	SwapModeExactOut    = 2
)

var (
	OneQ64 = new(big.Int).Lsh(big.NewInt(1), ScaleOffset)

	DynamicFeeScalingFactor  = big.NewInt(100000000000)
	DynamicFeeRoundingOffset = big.NewInt(99999999999)

	BinStepBpsU128Default = bigIntFromString("1844674407370955")

	FeePadding = [3]uint8{0, 0, 0}
)

func bigIntFromString(v string) *big.Int {
	out, ok := new(big.Int).SetString(v, 10)
	if !ok {
		panic("invalid big integer literal")
	}
	return out
}
