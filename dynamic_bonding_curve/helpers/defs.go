package helpers

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
)

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
	Config           solanago.PublicKey
	FeeClaimer       solanago.PublicKey
	LeftoverReceiver solanago.PublicKey
	QuoteMint        solanago.PublicKey
	Payer            solanago.PublicKey
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
