package shared

import (
	"math/big"

	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
	"github.com/shopspring/decimal"
)

// Enums and common types shared by math/pool_fees and dammv2.
type Rounding uint8

const (
	RoundingUp   Rounding = 0
	RoundingDown Rounding = 1
)

type ActivationPoint uint8

const (
	ActivationPointTimestamp ActivationPoint = 0
	ActivationPointSlot      ActivationPoint = 1
)

type BaseFeeMode uint8

const (
	BaseFeeModeFeeTimeSchedulerLinear      BaseFeeMode = 0
	BaseFeeModeFeeTimeSchedulerExponential BaseFeeMode = 1
	BaseFeeModeRateLimiter                 BaseFeeMode = 2
	BaseFeeModeFeeMarketCapSchedulerLinear BaseFeeMode = 3
	BaseFeeModeFeeMarketCapSchedulerExp    BaseFeeMode = 4
)

type CollectFeeMode = uint8

const (
	CollectFeeModeBothToken CollectFeeMode = 0
	CollectFeeModeOnlyB     CollectFeeMode = 1
)

type TradeDirection uint8

const (
	TradeDirectionAtoB TradeDirection = 0
	TradeDirectionBtoA TradeDirection = 1
)

type ActivationType uint8

const (
	ActivationTypeSlot      ActivationType = 0
	ActivationTypeTimestamp ActivationType = 1
)

type PoolVersion uint8

const (
	PoolVersionV0 PoolVersion = 0
	PoolVersionV1 PoolVersion = 1
)

type PoolStatus uint8

const (
	PoolStatusEnable  PoolStatus = 0
	PoolStatusDisable PoolStatus = 1
)

type SwapMode uint8

const (
	SwapModeExactIn     SwapMode = 0
	SwapModePartialFill SwapMode = 1
	SwapModeExactOut    SwapMode = 2
)

// Fee mode helpers.
type FeeMode struct {
	FeesOnInput  bool
	FeesOnTokenA bool
	HasReferral  bool
}

type FeeOnAmountResult struct {
	FeeNumerator   *big.Int
	FeeAmount      *big.Int
	AmountAfterFee *big.Int
	TradingFee     *big.Int
	ProtocolFee    *big.Int
	PartnerFee     *big.Int
	ReferralFee    *big.Int
}

type SplitFees struct {
	TradingFee  *big.Int
	ProtocolFee *big.Int
	ReferralFee *big.Int
	PartnerFee  *big.Int
}

type SwapResult2 = dammv2gen.SwapResult2

type Quote2Result struct {
	SwapResult2
	MinimumAmountOut *big.Int
	MaximumAmountIn  *big.Int
	PriceImpact      decimal.Decimal
}

type BaseFeeHandler interface {
	Validate(collectFeeMode CollectFeeMode, activationType ActivationType, poolVersion PoolVersion) bool
	GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection TradeDirection, includedFeeAmount *big.Int, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error)
	GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection TradeDirection, excludedFeeAmount *big.Int, initSqrtPrice, currentSqrtPrice *big.Int) (*big.Int, error)
	ValidateBaseFeeIsStatic(currentPoint, activationPoint *big.Int) bool
	GetMinFeeNumerator() *big.Int
	GetMaxFeeNumerator() (*big.Int, error)
}

const (
	BasisPointMax  = 10_000
	FeeDenominator = 1_000_000_000

	MinFeeBps       = 1       // 0.01%
	MinFeeNumerator = 100_000 // 0.01%

	MaxFeeBpsV0       = 5000        // 50%
	MaxFeeNumeratorV0 = 500_000_000 // 50%

	MaxFeeBpsV1       = 9900        // 99%
	MaxFeeNumeratorV1 = 990_000_000 // 99%

	LiquidityScale = 128
	ScaleOffset    = 64
	U16Max         = 65535

	MaxRateLimiterDurationInSeconds = 43_200
	MaxRateLimiterDurationInSlots   = 108_000

	DynamicFeeFilterPeriodDefault    = 10
	DynamicFeeDecayPeriodDefault     = 120
	DynamicFeeReductionFactorDefault = 5000
	BinStepBpsDefault                = 1
	MaxPriceChangeBpsDefault         = 1500

	MinCuBuffer = 50_000
	MaxCuBuffer = 200_000
)

var (
	OneQ64         = new(big.Int).Lsh(big.NewInt(1), ScaleOffset)
	MaxExponential = big.NewInt(0x80000)
	// MaxU128        = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	// U64Max         = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))

	U128Max = bigIntFromString("340282366920938463463374607431768211455")
	U64Max  = bigIntFromString("18446744073709551615")

	DynamicFeeScalingFactor  = big.NewInt(100000000000)
	DynamicFeeRoundingOffset = big.NewInt(99999999999)
	// BinStepBpsU128Default    = big.NewInt(1844674407370955)

	MinSqrtPrice = bigIntFromString("4295048016")
	MaxSqrtPrice = bigIntFromString("79226673521066979257578248091")

	BinStepBpsU128Default = bigIntFromString("1844674407370955")
)

func bigIntFromString(v string) *big.Int {
	out, ok := new(big.Int).SetString(v, 10)
	if !ok {
		panic("invalid big integer literal")
	}
	return out
}
