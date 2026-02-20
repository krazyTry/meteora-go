package shared

import (
	"math/big"

	"github.com/gagliardetto/solana-go"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
)

const (
	MaxCurvePoint = 16

	Offset     = 64
	Resolution = 64

	FeeDenominator = 1_000_000_000
	MaxBasisPoint  = 10_000

	U16Max = 65_535
	U24Max = 16_777_215

	MinFeeBps = 25
	MaxFeeBps = 9900

	MinFeeNumerator = 2_500_000
	MaxFeeNumerator = 990_000_000

	MaxRateLimiterDurationInSeconds = 43_200
	MaxRateLimiterDurationInSlots   = 108_000

	DynamicFeeFilterPeriodDefault    = 10
	DynamicFeeDecayPeriodDefault     = 120
	DynamicFeeReductionFactorDefault = 5000
	BinStepBpsDefault                = 1
	MaxPriceChangePercentageDefault  = 20

	ProtocolFeePercent = 20
	HostFeePercent     = 20

	SwapBufferPercentage = 25

	MaxMigrationFeePercentage        = 99
	MaxCreatorMigrationFeePercentage = 100

	MinLockedLiquidityBps    = 1000
	SecondsPerDay            = 86400
	MaxLockDurationInSeconds = 63_072_000

	ProtocolPoolCreationFeePercent = 10
	MinPoolCreationFee             = 1_000_000
	MaxPoolCreationFee             = 100_000_000_000

	MinMigratedPoolFeeBps = 10
	MaxMigratedPoolFeeBps = 1000
)

var (
	OneQ64 = new(big.Int).Lsh(big.NewInt(1), Resolution)

	U64Max  = new(big.Int).SetUint64(^uint64(0))
	U128Max = bigIntFromString("340282366920938463463374607431768211455")

	MinSqrtPrice = bigIntFromString("4295048016")
	MaxSqrtPrice = bigIntFromString("79226673521066979257578248091")

	DynamicFeeScalingFactor  = bigIntFromString("100000000000")
	DynamicFeeRoundingOffset = bigIntFromString("99999999999")

	BinStepBpsU128Default = bigIntFromString("1844674407370955")
)

func bigIntFromString(v string) *big.Int {
	out, ok := new(big.Int).SetString(v, 10)
	if !ok {
		panic("invalid big integer literal")
	}
	return out
}

// IDL type aliases.
type ConfigParameters = dbcidl.ConfigParameters
type LockedVestingParameters = dbcidl.LockedVestingParams
type InitializePoolParameters = dbcidl.InitializePoolParameters
type PoolFeeParameters = dbcidl.PoolFeeParameters
type DynamicFeeParameters = dbcidl.DynamicFeeParameters
type LiquidityDistributionParameters = dbcidl.LiquidityDistributionParameters
type MigratedPoolMarketCapFeeSchedulerParameters = dbcidl.MigratedPoolMarketCapFeeSchedulerParams
type LiquidityVestingInfoParameters = dbcidl.LiquidityVestingInfoParams
type CreatePartnerMetadataParameters = dbcidl.CreatePartnerMetadataParameters
type CreateVirtualPoolMetadataParameters = dbcidl.CreateVirtualPoolMetadataParameters
type PoolFeesConfig = dbcidl.PoolFeesConfig
type BaseFeeConfig = dbcidl.BaseFeeConfig
type BaseFeeParameters = dbcidl.BaseFeeParameters
type DynamicFeeConfig = dbcidl.DynamicFeeConfig
type MigratedPoolFee = dbcidl.MigratedPoolFee
type MigrationFee = dbcidl.MigrationFee
type TokenSupplyParams = dbcidl.TokenSupplyParams
type VolatilityTracker = dbcidl.VolatilityTracker

// IDL accounts.
type PoolConfig = dbcidl.PoolConfig
type VirtualPool = dbcidl.VirtualPool
type MeteoraDammMigrationMetadata = dbcidl.MeteoraDammMigrationMetadata
type LockEscrow = dbcidl.LockEscrow
type PartnerMetadata = dbcidl.PartnerMetadata
type VirtualPoolMetadata = dbcidl.VirtualPoolMetadata

// Enums.
type ActivationType uint8

const (
	ActivationTypeSlot      ActivationType = 0
	ActivationTypeTimestamp ActivationType = 1
)

type TokenType uint8

const (
	TokenTypeSPL       TokenType = 0
	TokenTypeToken2022 TokenType = 1
)

type CollectFeeMode uint8

const (
	CollectFeeModeQuoteToken  CollectFeeMode = 0
	CollectFeeModeOutputToken CollectFeeMode = 1
)

type DammV2DynamicFeeMode uint8

const (
	DammV2DynamicFeeModeDisabled DammV2DynamicFeeMode = 0
	DammV2DynamicFeeModeEnabled  DammV2DynamicFeeMode = 1
)

type DammV2BaseFeeMode uint8

const (
	DammV2BaseFeeModeFeeTimeSchedulerLinear      DammV2BaseFeeMode = 0
	DammV2BaseFeeModeFeeTimeSchedulerExponential DammV2BaseFeeMode = 1
	DammV2BaseFeeModeRateLimiter                 DammV2BaseFeeMode = 2
	DammV2BaseFeeModeFeeMarketCapSchedulerLinear DammV2BaseFeeMode = 3
	DammV2BaseFeeModeFeeMarketCapSchedulerExp    DammV2BaseFeeMode = 4
)

type MigrationOption uint8

const (
	MigrationOptionMetDamm   MigrationOption = 0
	MigrationOptionMetDammV2 MigrationOption = 1
)

type BaseFeeMode uint8

const (
	BaseFeeModeFeeSchedulerLinear      BaseFeeMode = 0
	BaseFeeModeFeeSchedulerExponential BaseFeeMode = 1
	BaseFeeModeRateLimiter             BaseFeeMode = 2
)

type MigrationFeeOption uint8

const (
	MigrationFeeOptionFixedBps25   MigrationFeeOption = 0
	MigrationFeeOptionFixedBps30   MigrationFeeOption = 1
	MigrationFeeOptionFixedBps100  MigrationFeeOption = 2
	MigrationFeeOptionFixedBps200  MigrationFeeOption = 3
	MigrationFeeOptionFixedBps400  MigrationFeeOption = 4
	MigrationFeeOptionFixedBps600  MigrationFeeOption = 5
	MigrationFeeOptionCustomizable MigrationFeeOption = 6
)

type TokenDecimal uint8

const (
	TokenDecimalSix   TokenDecimal = 6
	TokenDecimalSeven TokenDecimal = 7
	TokenDecimalEight TokenDecimal = 8
	TokenDecimalNine  TokenDecimal = 9
)

type TradeDirection uint8

const (
	TradeDirectionBaseToQuote TradeDirection = 0
	TradeDirectionQuoteToBase TradeDirection = 1
)

type Rounding uint8

const (
	RoundingUp   Rounding = 0
	RoundingDown Rounding = 1
)

type TokenUpdateAuthorityOption uint8

const (
	TokenUpdateAuthorityCreatorUpdateAuthority        TokenUpdateAuthorityOption = 0
	TokenUpdateAuthorityImmutable                     TokenUpdateAuthorityOption = 1
	TokenUpdateAuthorityPartnerUpdateAuthority        TokenUpdateAuthorityOption = 2
	TokenUpdateAuthorityCreatorUpdateAndMintAuthority TokenUpdateAuthorityOption = 3
	TokenUpdateAuthorityPartnerUpdateAndMintAuthority TokenUpdateAuthorityOption = 4
)

type SwapMode uint8

const (
	SwapModeExactIn     SwapMode = 0
	SwapModePartialFill SwapMode = 1
	SwapModeExactOut    SwapMode = 2
)

// MigrationProgress defines the migration progress states
type MigrationProgress uint8

const (
	MigrationProgressPreBondingCurve MigrationProgress = iota
	MigrationProgressPostBondingCurve
	MigrationProgressLockedVesting
	MigrationProgressCreatedPool
)

// IsMigrated defines the migration status
type IsMigrated uint8

const (
	IsMigratedProcess IsMigrated = iota
	IsMigratedCompleted
)

// WithdrawMigrationFeeFlag defines the migration fee withdrawal flags
type WithdrawMigrationFeeFlag uint8

const (
	PartnerWithdrawMigrationFeeFlag WithdrawMigrationFeeFlag = iota
	CreatorWithdrawMigrationFeeFlag
)

type MigrationFeeWithdrawStatus uint8

func (m MigrationFeeWithdrawStatus) IsPartnerWithdraw() uint8 {
	partnerMask := uint8(1) << 0 // 0x01
	return uint8(m) & partnerMask
}

func (m MigrationFeeWithdrawStatus) IsCreatorWithdraw() uint8 {
	creatorMask := uint8(1) << 1 // 0x01
	return uint8(m) & creatorMask
}

// Param/DTO structs mirroring TS types.
type CreateConfigParams struct {
	ConfigParameters
	Config           solana.PublicKey
	FeeClaimer       solana.PublicKey
	LeftoverReceiver solana.PublicKey
	QuoteMint        solana.PublicKey
	Payer            solana.PublicKey
}

type FeeSchedulerParams struct {
	StartingFeeBps uint16
	EndingFeeBps   uint16
	NumberOfPeriod uint64
	TotalDuration  uint64
}

type RateLimiterParams struct {
	BaseFeeBps         uint16
	FeeIncrementBps    uint16
	ReferenceAmount    uint64
	MaxLimiterDuration uint64
}

type BaseFeeParams struct {
	BaseFeeMode       BaseFeeMode
	FeeSchedulerParam *FeeSchedulerParams
	RateLimiterParam  *RateLimiterParams
}

type LockedVestingParams struct {
	TotalLockedVestingAmount       uint64
	NumberOfVestingPeriod          uint64
	CliffUnlockAmount              uint64
	TotalVestingDuration           uint64
	CliffDurationFromMigrationTime uint64
}

type BuildCurveBaseParams struct {
	TotalTokenSupply            uint64
	TokenType                   TokenType
	TokenBaseDecimal            TokenDecimal
	TokenQuoteDecimal           TokenDecimal
	TokenUpdateAuthority        uint8
	LockedVestingParams         LockedVestingParams
	Leftover                    uint64
	BaseFeeParams               BaseFeeParams
	DynamicFeeEnabled           bool
	ActivationType              ActivationType
	CollectFeeMode              CollectFeeMode
	CreatorTradingFeePercentage uint8
	PoolCreationFee             uint64
	MigrationOption             MigrationOption
	MigrationFeeOption          MigrationFeeOption
	MigrationFee                struct {
		FeePercentage        uint8
		CreatorFeePercentage uint8
	}
	PartnerPermanentLockedLiquidityPercentage uint8
	PartnerLiquidityPercentage                uint8
	CreatorPermanentLockedLiquidityPercentage uint8
	CreatorLiquidityPercentage                uint8
	EnableFirstSwapWithMinFee                 bool
	PartnerLiquidityVestingInfoParams         *LiquidityVestingInfoParams
	CreatorLiquidityVestingInfoParams         *LiquidityVestingInfoParams
	MigratedPoolFee                           *struct {
		CollectFeeMode CollectFeeMode
		DynamicFee     DammV2DynamicFeeMode
		PoolFeeBps     uint16
	}
	MigratedPoolBaseFeeMode                 *DammV2BaseFeeMode
	MigratedPoolMarketCapFeeSchedulerParams *MigratedPoolMarketCapFeeSchedulerParams
}

type BuildCurveParams struct {
	BuildCurveBaseParams
	PercentageSupplyOnMigration float64
	MigrationQuoteThreshold     float64
}

type BuildCurveWithMarketCapParams struct {
	BuildCurveBaseParams
	InitialMarketCap   float64
	MigrationMarketCap float64
}

type BuildCurveWithTwoSegmentsParams struct {
	BuildCurveBaseParams
	InitialMarketCap            float64
	MigrationMarketCap          float64
	PercentageSupplyOnMigration float64
}

type BuildCurveWithMidPriceParams struct {
	BuildCurveBaseParams
	InitialMarketCap            float64
	MigrationMarketCap          float64
	MidPrice                    uint64
	PercentageSupplyOnMigration uint64
}

type BuildCurveWithLiquidityWeightsParams struct {
	BuildCurveBaseParams
	InitialMarketCap   float64
	MigrationMarketCap float64
	LiquidityWeights   []float64
}

type BuildCurveWithCustomSqrtPricesParams struct {
	BuildCurveBaseParams
	SqrtPrices       []*big.Int
	LiquidityWeights []uint64
}

type LiquidityVestingInfoParams struct {
	LiquidityVestingInfoParameters
	TotalDuration uint64
}

type MigratedPoolMarketCapFeeSchedulerParams struct {
	MigratedPoolMarketCapFeeSchedulerParameters
	EndingBaseFeeBps uint16
}

type SwapResult struct {
	ActualInputAmount *big.Int
	OutputAmount      *big.Int
	NextSqrtPrice     *big.Int
	TradingFee        *big.Int
	ProtocolFee       *big.Int
	ReferralFee       *big.Int
}

type SwapResult2 struct {
	AmountLeft             *big.Int
	IncludedFeeInputAmount *big.Int
	ExcludedFeeInputAmount *big.Int
	OutputAmount           *big.Int
	NextSqrtPrice          *big.Int
	TradingFee             *big.Int
	ProtocolFee            *big.Int
	ReferralFee            *big.Int
}

type FeeMode struct {
	FeesOnInput     bool
	FeesOnBaseToken bool
	HasReferral     bool
}

type SwapQuoteResult struct {
	SwapResult
	MinimumAmountOut *big.Int
}

type SwapQuote2Result struct {
	SwapResult2
	MinimumAmountOut *big.Int
	MaximumAmountIn  *big.Int
}

type FeeOnAmountResult struct {
	Amount      *big.Int
	ProtocolFee *big.Int
	TradingFee  *big.Int
	ReferralFee *big.Int
}

type SwapAmount struct {
	OutputAmount  *big.Int
	NextSqrtPrice *big.Int
	AmountLeft    *big.Int
}

type BaseFeeHandler interface {
	Validate(collectFeeMode CollectFeeMode, activationType ActivationType) bool
	GetMinBaseFeeNumerator() *big.Int
	GetBaseFeeNumeratorFromIncludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection TradeDirection, includedFeeAmount *big.Int) *big.Int
	GetBaseFeeNumeratorFromExcludedFeeAmount(currentPoint, activationPoint *big.Int, tradeDirection TradeDirection, excludedFeeAmount *big.Int) *big.Int
}
