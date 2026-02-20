package shared

import (
	"math/big"

	"github.com/shopspring/decimal"
)

// Enums and common types shared by math/pool_fees and dammv2.
type Rounding uint8

const (
	RoundingUp   Rounding = 0
	RoundingDown Rounding = 1
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
	CollectFeeModeOnlyA     CollectFeeMode = 1
	CollectFeeModeOnlyB     CollectFeeMode = 2
)

type TradeDirection uint8

const (
	TradeDirectionAtoB TradeDirection = 0
	TradeDirectionBtoA TradeDirection = 1
)

type ActivationType = uint8

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

type SwapResult2 struct {
	IncludedFeeInputAmount *big.Int
	ExcludedFeeInputAmount *big.Int
	AmountLeft             *big.Int
	OutputAmount           *big.Int
	NextSqrtPrice          *big.Int
	TradingFee             *big.Int
	ProtocolFee            *big.Int
	PartnerFee             *big.Int
	ReferralFee            *big.Int
}

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

	MinFeeNumerator = 100_000

	MaxFeeBpsV0       = 5000
	MaxFeeNumeratorV0 = 500_000_000

	MaxFeeBpsV1       = 9900
	MaxFeeNumeratorV1 = 990_000_000

	ScaleOffset = 64
	U16Max      = 65535

	MaxRateLimiterDurationInSeconds = 43_200
	MaxRateLimiterDurationInSlots   = 108_000
)

var (
	OneQ64         = new(big.Int).Lsh(big.NewInt(1), ScaleOffset)
	MaxExponential = big.NewInt(0x80000)
	MaxU128        = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	U64Max         = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))

	DynamicFeeScalingFactor  = big.NewInt(100000000000)
	DynamicFeeRoundingOffset = big.NewInt(99999999999)
)
