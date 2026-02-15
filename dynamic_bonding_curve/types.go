package dynamic_bonding_curve

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

// IDL type aliases.

type ConfigParameters = helpers.ConfigParameters

type LockedVestingParameters = helpers.LockedVestingParameters

type InitializePoolParameters = helpers.InitializePoolParameters

type PoolFeeParameters = helpers.PoolFeeParameters

type DynamicFeeParameters = helpers.DynamicFeeParameters

type LiquidityDistributionParameters = helpers.LiquidityDistributionParameters

type MigratedPoolMarketCapFeeSchedulerParameters = helpers.MigratedPoolMarketCapFeeSchedulerParameters

type LiquidityVestingInfoParameters = helpers.LiquidityVestingInfoParameters

type CreatePartnerMetadataParameters = helpers.CreatePartnerMetadataParameters
type CreateVirtualPoolMetadataParameters = helpers.CreateVirtualPoolMetadataParameters

type PoolFeesConfig = helpers.PoolFeesConfig

type BaseFeeConfig = helpers.BaseFeeConfig
type BaseFeeParameters = helpers.BaseFeeParameters

type DynamicFeeConfig = helpers.DynamicFeeConfig

type MigratedPoolFee = helpers.MigratedPoolFee

type MigrationFee = helpers.MigrationFee

type TokenSupplyParams = helpers.TokenSupplyParams

type SwapResult = shared.SwapResult

type SwapResult2 = shared.SwapResult2

type VolatilityTracker = helpers.VolatilityTracker

// IDL accounts.

type PoolConfig = helpers.PoolConfig

type VirtualPool = helpers.VirtualPool

type MeteoraDammMigrationMetadata = helpers.MeteoraDammMigrationMetadata

type LockEscrow = helpers.LockEscrow

type PartnerMetadata = helpers.PartnerMetadata

type VirtualPoolMetadata = helpers.VirtualPoolMetadata

// Enums.

type ActivationType = helpers.ActivationType

const (
	ActivationTypeSlot      = helpers.ActivationTypeSlot
	ActivationTypeTimestamp = helpers.ActivationTypeTimestamp
)

type TokenType = helpers.TokenType

const (
	TokenTypeSPL       = helpers.TokenTypeSPL
	TokenTypeToken2022 = helpers.TokenTypeToken2022
)

type CollectFeeMode = helpers.CollectFeeMode

const (
	CollectFeeModeQuoteToken  = helpers.CollectFeeModeQuoteToken
	CollectFeeModeOutputToken = helpers.CollectFeeModeOutputToken
)

type DammV2DynamicFeeMode = helpers.DammV2DynamicFeeMode

const (
	DammV2DynamicFeeModeDisabled = helpers.DammV2DynamicFeeModeDisabled
	DammV2DynamicFeeModeEnabled  = helpers.DammV2DynamicFeeModeEnabled
)

type DammV2BaseFeeMode = helpers.DammV2BaseFeeMode

const (
	DammV2BaseFeeModeFeeTimeSchedulerLinear      = helpers.DammV2BaseFeeModeFeeTimeSchedulerLinear
	DammV2BaseFeeModeFeeTimeSchedulerExponential = helpers.DammV2BaseFeeModeFeeTimeSchedulerExponential
	DammV2BaseFeeModeRateLimiter                 = helpers.DammV2BaseFeeModeRateLimiter
	DammV2BaseFeeModeFeeMarketCapSchedulerLinear = helpers.DammV2BaseFeeModeFeeMarketCapSchedulerLinear
	DammV2BaseFeeModeFeeMarketCapSchedulerExp    = helpers.DammV2BaseFeeModeFeeMarketCapSchedulerExp
)

type MigrationOption = helpers.MigrationOption

const (
	MigrationOptionMetDamm   = helpers.MigrationOptionMetDamm
	MigrationOptionMetDammV2 = helpers.MigrationOptionMetDammV2
)

type BaseFeeMode = helpers.BaseFeeMode

const (
	BaseFeeModeFeeSchedulerLinear      = helpers.BaseFeeModeFeeSchedulerLinear
	BaseFeeModeFeeSchedulerExponential = helpers.BaseFeeModeFeeSchedulerExponential
	BaseFeeModeRateLimiter             = helpers.BaseFeeModeRateLimiter
)

type MigrationFeeOption = helpers.MigrationFeeOption

const (
	MigrationFeeOptionFixedBps25   = helpers.MigrationFeeOptionFixedBps25
	MigrationFeeOptionFixedBps30   = helpers.MigrationFeeOptionFixedBps30
	MigrationFeeOptionFixedBps100  = helpers.MigrationFeeOptionFixedBps100
	MigrationFeeOptionFixedBps200  = helpers.MigrationFeeOptionFixedBps200
	MigrationFeeOptionFixedBps400  = helpers.MigrationFeeOptionFixedBps400
	MigrationFeeOptionFixedBps600  = helpers.MigrationFeeOptionFixedBps600
	MigrationFeeOptionCustomizable = helpers.MigrationFeeOptionCustomizable
)

type TokenDecimal = helpers.TokenDecimal

const (
	TokenDecimalSix   = helpers.TokenDecimalSix
	TokenDecimalSeven = helpers.TokenDecimalSeven
	TokenDecimalEight = helpers.TokenDecimalEight
	TokenDecimalNine  = helpers.TokenDecimalNine
)

type TradeDirection = shared.TradeDirection

const (
	TradeDirectionBaseToQuote = shared.TradeDirectionBaseToQuote
	TradeDirectionQuoteToBase = shared.TradeDirectionQuoteToBase
)

type Rounding = helpers.Rounding

const (
	RoundingUp   = helpers.RoundingUp
	RoundingDown = helpers.RoundingDown
)

type TokenUpdateAuthorityOption = helpers.TokenUpdateAuthorityOption

const (
	TokenUpdateAuthorityCreatorUpdateAuthority        = helpers.TokenUpdateAuthorityCreatorUpdateAuthority
	TokenUpdateAuthorityImmutable                     = helpers.TokenUpdateAuthorityImmutable
	TokenUpdateAuthorityPartnerUpdateAuthority        = helpers.TokenUpdateAuthorityPartnerUpdateAuthority
	TokenUpdateAuthorityCreatorUpdateAndMintAuthority = helpers.TokenUpdateAuthorityCreatorUpdateAndMintAuthority
	TokenUpdateAuthorityPartnerUpdateAndMintAuthority = helpers.TokenUpdateAuthorityPartnerUpdateAndMintAuthority
)

type SwapMode = helpers.SwapMode

const (
	SwapModeExactIn     = helpers.SwapModeExactIn
	SwapModePartialFill = helpers.SwapModePartialFill
	SwapModeExactOut    = helpers.SwapModeExactOut
)

type MigrationProgress = helpers.MigrationProgress

const (
	MigrationProgressPreBondingCurve  = helpers.MigrationProgressPreBondingCurve
	MigrationProgressPostBondingCurve = helpers.MigrationProgressPostBondingCurve
	MigrationProgressLockedVesting    = helpers.MigrationProgressLockedVesting
	MigrationProgressCreatedPool      = helpers.MigrationProgressCreatedPool
)

var GetDammV2Config = helpers.GetDammV2Config

var GetDammV1Config = helpers.GetDammV1Config

// IsMigrated defines the migration status
type IsMigrated = helpers.IsMigrated

const (
	IsMigratedProcess   = helpers.IsMigratedProcess
	IsMigratedCompleted = helpers.IsMigratedCompleted
)

// WithdrawMigrationFeeFlag defines the migration fee withdrawal flags
type WithdrawMigrationFeeFlag = helpers.WithdrawMigrationFeeFlag

const (
	PartnerWithdrawMigrationFeeFlag = helpers.PartnerWithdrawMigrationFeeFlag
	CreatorWithdrawMigrationFeeFlag = helpers.CreatorWithdrawMigrationFeeFlag
)

type MigrationFeeWithdrawStatus = helpers.MigrationFeeWithdrawStatus

// Param/DTO structs mirroring TS types.

type CreateConfigParams = helpers.CreateConfigParams

// BaseFee equals BaseFeeConfig without padding.

type BaseFee = helpers.BaseFeeConfig

type InitializePoolBaseParams struct {
	Name         string
	Symbol       string
	URI          string
	Pool         solanago.PublicKey
	Config       solanago.PublicKey
	Payer        solanago.PublicKey
	PoolCreator  solanago.PublicKey
	BaseMint     solanago.PublicKey
	BaseVault    solanago.PublicKey
	QuoteVault   solanago.PublicKey
	QuoteMint    solanago.PublicKey
	MintMetadata *solanago.PublicKey
}

type CreatePoolParams struct {
	Name        string
	Symbol      string
	URI         string
	Payer       solanago.PublicKey
	PoolCreator solanago.PublicKey
	Config      solanago.PublicKey
	BaseMint    solanago.PublicKey
}

type PreCreatePoolParams struct {
	Name        string
	Symbol      string
	URI         string
	PoolCreator solanago.PublicKey
	BaseMint    solanago.PublicKey
}

type CreateConfigAndPoolParams struct {
	CreateConfigParams
	PreCreatePoolParam PreCreatePoolParams
}

type CreateConfigAndPoolWithFirstBuyParams struct {
	CreateConfigAndPoolParams
	FirstBuyParam *FirstBuyParams
}

type CreatePoolWithFirstBuyParams struct {
	CreatePoolParam CreatePoolParams
	FirstBuyParam   *FirstBuyParams
}

type CreatePoolWithPartnerAndCreatorFirstBuyParams struct {
	CreatePoolParam      CreatePoolParams
	PartnerFirstBuyParam *PartnerFirstBuyParams
	CreatorFirstBuyParam *CreatorFirstBuyParams
}

type FirstBuyParams struct {
	Buyer                solanago.PublicKey
	Receiver             *solanago.PublicKey
	BuyAmount            *big.Int
	MinimumAmountOut     *big.Int
	ReferralTokenAccount *solanago.PublicKey
}

type PartnerFirstBuyParams struct {
	Partner              solanago.PublicKey
	Receiver             solanago.PublicKey
	BuyAmount            *big.Int
	MinimumAmountOut     *big.Int
	ReferralTokenAccount *solanago.PublicKey
}

type CreatorFirstBuyParams struct {
	Creator              solanago.PublicKey
	Receiver             solanago.PublicKey
	BuyAmount            *big.Int
	MinimumAmountOut     *big.Int
	ReferralTokenAccount *solanago.PublicKey
}

type SwapParams struct {
	Owner                solanago.PublicKey
	Pool                 solanago.PublicKey
	AmountIn             *big.Int
	MinimumAmountOut     *big.Int
	SwapBaseForQuote     bool
	ReferralTokenAccount *solanago.PublicKey
	Payer                *solanago.PublicKey
}

type Swap2Params struct {
	Owner                solanago.PublicKey
	Pool                 solanago.PublicKey
	SwapBaseForQuote     bool
	ReferralTokenAccount *solanago.PublicKey
	Payer                *solanago.PublicKey
	SwapMode             SwapMode
	AmountIn             *big.Int
	MinimumAmountOut     *big.Int
	AmountOut            *big.Int
	MaximumAmountIn      *big.Int
}

type SwapQuoteParams struct {
	VirtualPool                    *VirtualPool
	Config                         *PoolConfig
	SwapBaseForQuote               bool
	AmountIn                       *big.Int
	SlippageBps                    uint16
	HasReferral                    bool
	EligibleForFirstSwapWithMinFee bool
	CurrentPoint                   *big.Int
}

type SwapQuote2Params struct {
	VirtualPool                    *VirtualPool
	Config                         *PoolConfig
	SwapBaseForQuote               bool
	HasReferral                    bool
	EligibleForFirstSwapWithMinFee bool
	CurrentPoint                   *big.Int
	SlippageBps                    uint16
	SwapMode                       SwapMode
	AmountIn                       *big.Int
	AmountOut                      *big.Int
}

type MigrateToDammV1Params struct {
	Payer       solanago.PublicKey
	VirtualPool solanago.PublicKey
	DammConfig  solanago.PublicKey
}

type MigrateToDammV2Params = MigrateToDammV1Params

type MigrateToDammV2Response struct {
	Transaction       *solanago.Transaction
	FirstPositionNFT  solanago.PrivateKey
	SecondPositionNFT solanago.PrivateKey
}

type DammLpTokenParams struct {
	Payer       solanago.PublicKey
	VirtualPool solanago.PublicKey
	DammConfig  solanago.PublicKey
	IsPartner   bool
}

type CreateLockerParams struct {
	Payer       solanago.PublicKey
	VirtualPool solanago.PublicKey
}

type ClaimTradingFeeParams struct {
	FeeClaimer     solanago.PublicKey
	Payer          solanago.PublicKey
	Pool           solanago.PublicKey
	MaxBaseAmount  *big.Int
	MaxQuoteAmount *big.Int
	Receiver       *solanago.PublicKey
	TempWSolAcc    *solanago.PublicKey
}

type ClaimTradingFee2Params struct {
	FeeClaimer     solanago.PublicKey
	Payer          solanago.PublicKey
	Pool           solanago.PublicKey
	MaxBaseAmount  *big.Int
	MaxQuoteAmount *big.Int
	Receiver       solanago.PublicKey
}

type ClaimPartnerTradingFeeWithQuoteMintNotSolParams struct {
	FeeClaimer        solanago.PublicKey
	Payer             solanago.PublicKey
	FeeReceiver       solanago.PublicKey
	Config            solanago.PublicKey
	Pool              solanago.PublicKey
	PoolState         *VirtualPool
	PoolConfigState   *PoolConfig
	TokenBaseProgram  solanago.PublicKey
	TokenQuoteProgram solanago.PublicKey
}

type ClaimPartnerTradingFeeWithQuoteMintSolParams struct {
	ClaimPartnerTradingFeeWithQuoteMintNotSolParams
	TempWSolAcc solanago.PublicKey
}

type ClaimCreatorTradingFeeParams struct {
	Creator        solanago.PublicKey
	Payer          solanago.PublicKey
	Pool           solanago.PublicKey
	MaxBaseAmount  *big.Int
	MaxQuoteAmount *big.Int
	Receiver       *solanago.PublicKey
	TempWSolAcc    *solanago.PublicKey
}

type ClaimCreatorTradingFee2Params struct {
	Creator        solanago.PublicKey
	Payer          solanago.PublicKey
	Pool           solanago.PublicKey
	MaxBaseAmount  *big.Int
	MaxQuoteAmount *big.Int
	Receiver       solanago.PublicKey
}

type ClaimCreatorTradingFeeWithQuoteMintNotSolParams struct {
	Creator           solanago.PublicKey
	Payer             solanago.PublicKey
	FeeReceiver       solanago.PublicKey
	Pool              solanago.PublicKey
	PoolState         *VirtualPool
	PoolConfigState   *PoolConfig
	TokenBaseProgram  solanago.PublicKey
	TokenQuoteProgram solanago.PublicKey
}

type ClaimCreatorTradingFeeWithQuoteMintSolParams struct {
	ClaimCreatorTradingFeeWithQuoteMintNotSolParams
	TempWSolAcc solanago.PublicKey
}

type PartnerWithdrawSurplusParams struct {
	FeeClaimer  solanago.PublicKey
	VirtualPool solanago.PublicKey
}

type CreatorWithdrawSurplusParams struct {
	Creator     solanago.PublicKey
	VirtualPool solanago.PublicKey
}

type WithdrawLeftoverParams struct {
	Payer       solanago.PublicKey
	VirtualPool solanago.PublicKey
}

type CreateVirtualPoolMetadataParams struct {
	VirtualPool solanago.PublicKey
	Name        string
	Website     string
	Logo        string
	Creator     solanago.PublicKey
	Payer       solanago.PublicKey
}

type CreateDammV1MigrationMetadataParams struct {
	VirtualPool solanago.PublicKey
	Config      solanago.PublicKey
	Payer       solanago.PublicKey
}

type CreateDammV2MigrationMetadataParams = CreateDammV1MigrationMetadataParams

type CreatePartnerMetadataParams struct {
	Name       string
	Website    string
	Logo       string
	FeeClaimer solanago.PublicKey
	Payer      solanago.PublicKey
}

type TransferPoolCreatorParams struct {
	VirtualPool solanago.PublicKey
	Creator     solanago.PublicKey
	NewCreator  solanago.PublicKey
}

type WithdrawMigrationFeeParams struct {
	VirtualPool solanago.PublicKey
	Sender      solanago.PublicKey
}

type ClaimPartnerPoolCreationFeeParams struct {
	VirtualPool solanago.PublicKey
	FeeReceiver solanago.PublicKey
}

// Interfaces.

type BaseFeeHandler = shared.BaseFeeHandler

type FeeResult struct {
	Amount      *big.Int
	ProtocolFee *big.Int
	TradingFee  *big.Int
	ReferralFee *big.Int
}

type FeeMode = shared.FeeMode

type SwapQuoteResult = shared.SwapQuoteResult

type SwapQuote2Result = shared.SwapQuote2Result

type FeeOnAmountResult = shared.FeeOnAmountResult

type PrepareSwapParams struct {
	InputMint          solanago.PublicKey
	OutputMint         solanago.PublicKey
	InputTokenProgram  solanago.PublicKey
	OutputTokenProgram solanago.PublicKey
}

type SwapAmount = shared.SwapAmount

type ProgramAccount[T any] struct {
	Pubkey  solanago.PublicKey
	Account *T
}
