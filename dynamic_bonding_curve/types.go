package dynamic_bonding_curve

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

// IDL type aliases.
type ConfigParameters = shared.ConfigParameters

type LockedVestingParameters = shared.LockedVestingParameters

type InitializePoolParameters = shared.InitializePoolParameters

type PoolFeeParameters = shared.PoolFeeParameters

type DynamicFeeParameters = shared.DynamicFeeParameters

type LiquidityDistributionParameters = shared.LiquidityDistributionParameters

type MigratedPoolMarketCapFeeSchedulerParameters = shared.MigratedPoolMarketCapFeeSchedulerParameters

type LiquidityVestingInfoParameters = shared.LiquidityVestingInfoParameters

type CreatePartnerMetadataParameters = shared.CreatePartnerMetadataParameters
type CreateVirtualPoolMetadataParameters = shared.CreateVirtualPoolMetadataParameters

type PoolFeesConfig = shared.PoolFeesConfig

type BaseFeeConfig = shared.BaseFeeConfig
type BaseFeeParameters = shared.BaseFeeParameters

type DynamicFeeConfig = shared.DynamicFeeConfig

type MigratedPoolFee = shared.MigratedPoolFee

type MigrationFee = shared.MigrationFee

type TokenSupplyParams = shared.TokenSupplyParams

type SwapResult = shared.SwapResult

type SwapResult2 = shared.SwapResult2

type VolatilityTracker = shared.VolatilityTracker

// IDL accounts.

type PoolConfig = shared.PoolConfig

type VirtualPool = shared.VirtualPool

type MeteoraDammMigrationMetadata = shared.MeteoraDammMigrationMetadata

type LockEscrow = shared.LockEscrow

type PartnerMetadata = shared.PartnerMetadata

type VirtualPoolMetadata = shared.VirtualPoolMetadata

// Enums.

type ActivationType = shared.ActivationType

const (
	ActivationTypeSlot      = shared.ActivationTypeSlot
	ActivationTypeTimestamp = shared.ActivationTypeTimestamp
)

type TokenType = shared.TokenType

const (
	TokenTypeSPL       = shared.TokenTypeSPL
	TokenTypeToken2022 = shared.TokenTypeToken2022
)

type CollectFeeMode = shared.CollectFeeMode

const (
	CollectFeeModeQuoteToken  = shared.CollectFeeModeQuoteToken
	CollectFeeModeOutputToken = shared.CollectFeeModeOutputToken
)

type DammV2DynamicFeeMode = shared.DammV2DynamicFeeMode

const (
	DammV2DynamicFeeModeDisabled = shared.DammV2DynamicFeeModeDisabled
	DammV2DynamicFeeModeEnabled  = shared.DammV2DynamicFeeModeEnabled
)

type DammV2BaseFeeMode = shared.DammV2BaseFeeMode

const (
	DammV2BaseFeeModeFeeTimeSchedulerLinear      = shared.DammV2BaseFeeModeFeeTimeSchedulerLinear
	DammV2BaseFeeModeFeeTimeSchedulerExponential = shared.DammV2BaseFeeModeFeeTimeSchedulerExponential
	DammV2BaseFeeModeRateLimiter                 = shared.DammV2BaseFeeModeRateLimiter
	DammV2BaseFeeModeFeeMarketCapSchedulerLinear = shared.DammV2BaseFeeModeFeeMarketCapSchedulerLinear
	DammV2BaseFeeModeFeeMarketCapSchedulerExp    = shared.DammV2BaseFeeModeFeeMarketCapSchedulerExp
)

type MigrationOption = shared.MigrationOption

const (
	MigrationOptionMetDamm   = shared.MigrationOptionMetDamm
	MigrationOptionMetDammV2 = shared.MigrationOptionMetDammV2
)

type BaseFeeMode = shared.BaseFeeMode

const (
	BaseFeeModeFeeSchedulerLinear      = shared.BaseFeeModeFeeSchedulerLinear
	BaseFeeModeFeeSchedulerExponential = shared.BaseFeeModeFeeSchedulerExponential
	BaseFeeModeRateLimiter             = shared.BaseFeeModeRateLimiter
)

type MigrationFeeOption = shared.MigrationFeeOption

const (
	MigrationFeeOptionFixedBps25   = shared.MigrationFeeOptionFixedBps25
	MigrationFeeOptionFixedBps30   = shared.MigrationFeeOptionFixedBps30
	MigrationFeeOptionFixedBps100  = shared.MigrationFeeOptionFixedBps100
	MigrationFeeOptionFixedBps200  = shared.MigrationFeeOptionFixedBps200
	MigrationFeeOptionFixedBps400  = shared.MigrationFeeOptionFixedBps400
	MigrationFeeOptionFixedBps600  = shared.MigrationFeeOptionFixedBps600
	MigrationFeeOptionCustomizable = shared.MigrationFeeOptionCustomizable
)

type TokenDecimal = shared.TokenDecimal

const (
	TokenDecimalSix   = shared.TokenDecimalSix
	TokenDecimalSeven = shared.TokenDecimalSeven
	TokenDecimalEight = shared.TokenDecimalEight
	TokenDecimalNine  = shared.TokenDecimalNine
)

type TradeDirection = shared.TradeDirection

const (
	TradeDirectionBaseToQuote = shared.TradeDirectionBaseToQuote
	TradeDirectionQuoteToBase = shared.TradeDirectionQuoteToBase
)

type Rounding = shared.Rounding

const (
	RoundingUp   = shared.RoundingUp
	RoundingDown = shared.RoundingDown
)

type TokenUpdateAuthorityOption = shared.TokenUpdateAuthorityOption

const (
	TokenUpdateAuthorityCreatorUpdateAuthority        = shared.TokenUpdateAuthorityCreatorUpdateAuthority
	TokenUpdateAuthorityImmutable                     = shared.TokenUpdateAuthorityImmutable
	TokenUpdateAuthorityPartnerUpdateAuthority        = shared.TokenUpdateAuthorityPartnerUpdateAuthority
	TokenUpdateAuthorityCreatorUpdateAndMintAuthority = shared.TokenUpdateAuthorityCreatorUpdateAndMintAuthority
	TokenUpdateAuthorityPartnerUpdateAndMintAuthority = shared.TokenUpdateAuthorityPartnerUpdateAndMintAuthority
)

type SwapMode = shared.SwapMode

const (
	SwapModeExactIn     = shared.SwapModeExactIn
	SwapModePartialFill = shared.SwapModePartialFill
	SwapModeExactOut    = shared.SwapModeExactOut
)

type MigrationProgress = shared.MigrationProgress

const (
	MigrationProgressPreBondingCurve  = shared.MigrationProgressPreBondingCurve
	MigrationProgressPostBondingCurve = shared.MigrationProgressPostBondingCurve
	MigrationProgressLockedVesting    = shared.MigrationProgressLockedVesting
	MigrationProgressCreatedPool      = shared.MigrationProgressCreatedPool
)

// IsMigrated defines the migration status
type IsMigrated = shared.IsMigrated

const (
	IsMigratedProcess   = shared.IsMigratedProcess
	IsMigratedCompleted = shared.IsMigratedCompleted
)

// WithdrawMigrationFeeFlag defines the migration fee withdrawal flags
type WithdrawMigrationFeeFlag = shared.WithdrawMigrationFeeFlag

const (
	PartnerWithdrawMigrationFeeFlag = shared.PartnerWithdrawMigrationFeeFlag
	CreatorWithdrawMigrationFeeFlag = shared.CreatorWithdrawMigrationFeeFlag
)

type MigrationFeeWithdrawStatus = shared.MigrationFeeWithdrawStatus

// Param/DTO structs mirroring TS types.

type CreateConfigParams = shared.CreateConfigParams

// BaseFee equals BaseFeeConfig without padding.

type BaseFee = shared.BaseFeeConfig

type BaseFeeHandler = shared.BaseFeeHandler

type FeeMode = shared.FeeMode

type SwapQuoteResult = shared.SwapQuoteResult

type SwapQuote2Result = shared.SwapQuote2Result

type FeeOnAmountResult = shared.FeeOnAmountResult

type SwapAmount = shared.SwapAmount

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
type FeeResult struct {
	Amount      *big.Int
	ProtocolFee *big.Int
	TradingFee  *big.Int
	ReferralFee *big.Int
}

type PrepareSwapParams struct {
	InputMint          solanago.PublicKey
	OutputMint         solanago.PublicKey
	InputTokenProgram  solanago.PublicKey
	OutputTokenProgram solanago.PublicKey
}

type ProgramAccount[T any] struct {
	Pubkey  solanago.PublicKey
	Account *T
}
