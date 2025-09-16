package dynamic_bonding_curve

import (
	dmath "github.com/krazyTry/meteora-go/decimal_math"
	"github.com/shopspring/decimal"
)

// Account keys supported in IDL
var (
	AccountKeyClaimFeeOperator             = "ClaimFeeOperator"
	AccountKeyConfig                       = "Config"
	AccountKeyLockEscrow                   = "LockEscrow"
	AccountKeyMeteoraDammMigrationMetadata = "MeteoraDammMigrationMetadata"
	AccountKeyMeteoraDammV2Metadata        = "MeteoraDammV2Metadata"
	AccountKeyPartnerMetadata              = "PartnerMetadata"
	AccountKeyPoolConfig                   = "PoolConfig"
	AccountKeyVirtualPool                  = "VirtualPool"
	AccountKeyVirtualPoolMetadata          = "VirtualPoolMetadata"
)

// Fee calculation constants
var (
	MIN_FEE_NUMERATOR = decimal.NewFromInt(100_000)     // 0.0001%
	MAX_FEE_NUMERATOR = decimal.NewFromInt(990_000_000) // 99%
	FEE_DENOMINATOR   = decimal.NewFromInt(1_000_000_000)
	BASIS_POINT_MAX   = decimal.NewFromInt(10000)

	// Fee basis points and numeric limits
	MAX_FEE_BPS = int64(9900) // 99%
	MIN_FEE_BPS = int64(1)    // 0.0001%
	U16_MAX     = decimal.NewFromInt(65535)
	U128_MAX, _ = decimal.NewFromString("340282366920938463463374607431768211455")

	// Binary step and price constants
	BIN_STEP_BPS_DEFAULT      = decimal.NewFromInt(1)
	BIN_STEP_BPS_U128_DEFAULT = decimal.NewFromInt(1844674407370955)

	// Price range limits
	MIN_SQRT_PRICE    = decimal.NewFromInt(4295048016)
	MAX_SQRT_PRICE, _ = decimal.NewFromString("79226673521066979257578248091")

	// Dynamic fee configuration defaults
	DYNAMIC_FEE_FILTER_PERIOD_DEFAULT    = uint16(10)   // 10 seconds
	DYNAMIC_FEE_DECAY_PERIOD_DEFAULT     = uint16(120)  // 120 seconds
	DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT = uint16(5000) // 50%

	// Rate limiter duration limits
	MAX_RATE_LIMITER_DURATION_IN_SECONDS = 43200
	MAX_RATE_LIMITER_DURATION_IN_SLOTS   = 108000

	// Price change limits
	MAX_PRICE_CHANGE_BPS_DEFAULT = int64(1500) // 15%

	// Mathematical constants for calculations
	N0     = decimal.Zero
	N025   = decimal.NewFromFloat(0.25)
	N1     = decimal.NewFromInt(1)
	N2     = decimal.NewFromInt(2)
	N3     = decimal.NewFromFloat(3)
	N20    = decimal.NewFromInt(20)
	N100   = decimal.NewFromInt(100)
	N10000 = decimal.NewFromInt(10000)

	N99_999_999_999  = decimal.NewFromInt(99_999_999_999)
	N100_000_000_000 = decimal.NewFromInt(100_000_000_000)

	Q64  = dmath.Lsh(N1, 64)
	Q128 = dmath.Lsh(N1, 128)
)

// ActivationType defines the type of activation mechanism
type ActivationType uint8

const (
	ActivationTypeSlot ActivationType = iota
	ActivationTypeTimestamp
)

// TokenType defines the type of token standard
type TokenType uint8

const (
	TokenTypeSPL TokenType = iota
	TokenTypeToken2022
)

// DammV2DynamicFeeMode defines the dynamic fee mode for DAMM v2
type DammV2DynamicFeeMode uint8

const (
	DammV2DynamicFeeDisabled DammV2DynamicFeeMode = iota
	DammV2DynamicFeeEnabled
)

// MigrationOption defines the available migration options
type MigrationOption uint8

const (
	MigrationOptionMETDAMM MigrationOption = iota
	MigrationOptionMETDAMMV2
)

// BaseFeeMode defines the base fee calculation mode
type BaseFeeMode uint8

const (
	BaseFeeModeFeeSchedulerLinear BaseFeeMode = iota
	BaseFeeModeFeeSchedulerExponential
	BaseFeeModeRateLimiter
)

// MigrationFeeOption defines the available migration fee options
type MigrationFeeOption uint8

const (
	MigrationFeeFixedBps25 MigrationFeeOption = iota
	MigrationFeeFixedBps30
	MigrationFeeFixedBps100
	MigrationFeeFixedBps200
	MigrationFeeFixedBps400
	MigrationFeeFixedBps600
	MigrationFeeCustomizable // only for DAMM v2
)

// TokenDecimal defines the supported token decimal places
type TokenDecimal uint8

const (
	TokenDecimalSix   TokenDecimal = 6
	TokenDecimalSeven TokenDecimal = 7
	TokenDecimalEight TokenDecimal = 8
	TokenDecimalNine  TokenDecimal = 9
)

// CollectFeeMode defines the fee collection mode
type CollectFeeMode uint8

const (
	CollectFeeModeQuoteToken CollectFeeMode = iota
	CollectFeeModeOutputToken
)

// TradeDirection defines the direction of a trade
type TradeDirection uint8

const (
	TradeDirectionBaseToQuote TradeDirection = iota
	TradeDirectionQuoteToBase
)

// TokenUpdateAuthorityOption defines the token update authority options
type TokenUpdateAuthorityOption uint8

const (
	TokenUpdateAuthorityCreatorUpdateAuthority TokenUpdateAuthorityOption = iota
	TokenUpdateAuthorityImmutable
	TokenUpdateAuthorityPartnerUpdateAuthority
	TokenUpdateAuthorityCreatorUpdateAndMintAuthority
	TokenUpdateAuthorityPartnerUpdateAndMintAuthority
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
