package dammv2

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

// TxBuilder mirrors the TS builder style using a transaction builder.
type TxBuilder = *solanago.TransactionBuilder

// Enums.
type Rounding = shared.Rounding

const (
	RoundingUp   = shared.RoundingUp
	RoundingDown = shared.RoundingDown
)

type ActivationPoint uint8

const (
	ActivationPointTimestamp ActivationPoint = 0
	ActivationPointSlot      ActivationPoint = 1
)

type BaseFeeMode = shared.BaseFeeMode

const (
	BaseFeeModeFeeTimeSchedulerLinear      = shared.BaseFeeModeFeeTimeSchedulerLinear
	BaseFeeModeFeeTimeSchedulerExponential = shared.BaseFeeModeFeeTimeSchedulerExponential
	BaseFeeModeRateLimiter                 = shared.BaseFeeModeRateLimiter
	BaseFeeModeFeeMarketCapSchedulerLinear = shared.BaseFeeModeFeeMarketCapSchedulerLinear
	BaseFeeModeFeeMarketCapSchedulerExp    = shared.BaseFeeModeFeeMarketCapSchedulerExp
)

type CollectFeeMode = shared.CollectFeeMode

const (
	CollectFeeModeBothToken = shared.CollectFeeModeBothToken
	CollectFeeModeOnlyB     = shared.CollectFeeModeOnlyB
)

type TradeDirection = shared.TradeDirection

const (
	TradeDirectionAtoB = shared.TradeDirectionAtoB
	TradeDirectionBtoA = shared.TradeDirectionBtoA
)

type ActivationType = shared.ActivationType

const (
	ActivationTypeSlot      = shared.ActivationTypeSlot
	ActivationTypeTimestamp = shared.ActivationTypeTimestamp
)

type PoolVersion = shared.PoolVersion

const (
	PoolVersionV0 = shared.PoolVersionV0
	PoolVersionV1 = shared.PoolVersionV1
)

type PoolStatus = shared.PoolStatus

const (
	PoolStatusEnable  = shared.PoolStatusEnable
	PoolStatusDisable = shared.PoolStatusDisable
)

type SwapMode = shared.SwapMode

const (
	SwapModeExactIn     = shared.SwapModeExactIn
	SwapModePartialFill = shared.SwapModePartialFill
	SwapModeExactOut    = shared.SwapModeExactOut
)

// Fee mode helpers.
type FeeMode = shared.FeeMode

// IDL Account aliases.
type PoolState = dammv2gen.Pool

type PositionState = dammv2gen.Position

type VestingState = dammv2gen.Vesting

type ConfigState = dammv2gen.Config

type TokenBadgeState = dammv2gen.TokenBadge

// IDL types.
type BorshFeeTimeScheduler = dammv2gen.BorshFeeTimeScheduler

type BorshFeeMarketCapScheduler = dammv2gen.BorshFeeMarketCapScheduler

type BorshFeeRateLimiter = dammv2gen.BorshFeeRateLimiter

type PodAlignedFeeTimeScheduler = dammv2gen.PodAlignedFeeTimeScheduler

type PodAlignedFeeMarketCapScheduler = dammv2gen.PodAlignedFeeMarketCapScheduler

type PodAlignedFeeRateLimiter = dammv2gen.PodAlignedFeeRateLimiter

type RewardInfo = dammv2gen.RewardInfo

type UserRewardInfo = dammv2gen.UserRewardInfo

type InnerVesting = dammv2gen.InnerVesting

type DynamicFee = dammv2gen.DynamicFeeParameters

type DynamicFeeStruct = dammv2gen.DynamicFeeStruct

type BaseFee = dammv2gen.BaseFeeParameters

type PoolFeesStruct = dammv2gen.PoolFeesStruct

// DecodedPoolFees is a union of decoded fee structs.
type DecodedPoolFees any

// Parameters.
type PoolFeesParams struct {
	BaseFee    BaseFee
	Padding    []uint8
	DynamicFee *DynamicFee
}

type PrepareTokenAccountParams struct {
	Payer         solanago.PublicKey
	TokenAOwner   solanago.PublicKey
	TokenBOwner   solanago.PublicKey
	TokenAMint    solanago.PublicKey
	TokenBMint    solanago.PublicKey
	TokenAProgram solanago.PublicKey
	TokenBProgram solanago.PublicKey
}

type PrepareCustomizablePoolParams struct {
	Pool          solanago.PublicKey
	TokenAMint    solanago.PublicKey
	TokenBMint    solanago.PublicKey
	TokenAAmount  *big.Int
	TokenBAmount  *big.Int
	Payer         solanago.PublicKey
	PositionNft   solanago.PublicKey
	TokenAProgram solanago.PublicKey
	TokenBProgram solanago.PublicKey
}

type InitializeCustomizeablePoolParams struct {
	Payer           solanago.PublicKey
	Creator         solanago.PublicKey
	PositionNft     solanago.PublicKey
	TokenAMint      solanago.PublicKey
	TokenBMint      solanago.PublicKey
	TokenAAmount    *big.Int
	TokenBAmount    *big.Int
	SqrtMinPrice    *big.Int
	SqrtMaxPrice    *big.Int
	LiquidityDelta  *big.Int
	InitSqrtPrice   *big.Int
	PoolFees        PoolFeesParams
	HasAlphaVault   bool
	ActivationType  uint8
	CollectFeeMode  uint8
	ActivationPoint *big.Int
	TokenAProgram   solanago.PublicKey
	TokenBProgram   solanago.PublicKey
	IsLockLiquidity bool
}

type InitializeCustomizeablePoolWithDynamicConfigParams struct {
	InitializeCustomizeablePoolParams
	Config               solanago.PublicKey
	PoolCreatorAuthority solanago.PublicKey
}

type PreparePoolCreationParams struct {
	TokenAAmount *big.Int
	TokenBAmount *big.Int
	MinSqrtPrice *big.Int
	MaxSqrtPrice *big.Int
	TokenAInfo   *TokenInfo
	TokenBInfo   *TokenInfo
}

type PreparedPoolCreation struct {
	InitSqrtPrice  *big.Int
	LiquidityDelta *big.Int
}

type PreparePoolCreationSingleSide struct {
	TokenAAmount  *big.Int
	MinSqrtPrice  *big.Int
	MaxSqrtPrice  *big.Int
	InitSqrtPrice *big.Int
	TokenAInfo    *TokenInfo
}

// TokenInfo mirrors needed fields for Token2022 fee calculations.
type TokenInfo = helpers.TokenInfo

var GetTokenInfo = helpers.GetTokenInfo

// Pool creation & position params.
type CreatePoolParams struct {
	Creator         solanago.PublicKey
	Payer           solanago.PublicKey
	Config          solanago.PublicKey
	PositionNft     solanago.PublicKey
	TokenAMint      solanago.PublicKey
	TokenBMint      solanago.PublicKey
	InitSqrtPrice   *big.Int
	LiquidityDelta  *big.Int
	TokenAAmount    *big.Int
	TokenBAmount    *big.Int
	ActivationPoint *big.Int
	TokenAProgram   solanago.PublicKey
	TokenBProgram   solanago.PublicKey
	IsLockLiquidity bool
}

type CreatePositionParams struct {
	Owner       solanago.PublicKey
	Payer       solanago.PublicKey
	Pool        solanago.PublicKey
	PositionNft solanago.PublicKey
}

type AddLiquidityParams struct {
	Owner                 solanago.PublicKey
	Pool                  solanago.PublicKey
	PoolState             *PoolState
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	LiquidityDelta        *big.Int
	MaxAmountTokenA       *big.Int
	MaxAmountTokenB       *big.Int
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
}

type CreatePositionAndAddLiquidity struct {
	Owner                 solanago.PublicKey
	Pool                  solanago.PublicKey
	PositionNft           solanago.PublicKey
	LiquidityDelta        *big.Int
	MaxAmountTokenA       *big.Int
	MaxAmountTokenB       *big.Int
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
	TokenAMint            solanago.PublicKey
	TokenBMint            solanago.PublicKey
	TokenAProgram         solanago.PublicKey
	TokenBProgram         solanago.PublicKey
}

type LiquidityDeltaParams struct {
	MaxAmountTokenA *big.Int
	MaxAmountTokenB *big.Int
	SqrtPrice       *big.Int
	SqrtMinPrice    *big.Int
	SqrtMaxPrice    *big.Int
	TokenAInfo      *TokenInfo
	TokenBInfo      *TokenInfo
}

type RemoveLiquidityParams struct {
	Owner                 solanago.PublicKey
	Pool                  solanago.PublicKey
	PoolState             *PoolState
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	LiquidityDelta        *big.Int
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
	Vestings              []VestingWithAccount
	CurrentPoint          *big.Int
}

type RemoveAllLiquidityParams struct {
	Owner                 solanago.PublicKey
	Pool                  solanago.PublicKey
	PoolState             PoolState
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
	Vestings              []VestingWithAccount
	CurrentPoint          *big.Int
}

type BuildAddLiquidityParams struct {
	Pool                  solanago.PublicKey
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	Owner                 solanago.PublicKey
	TokenAAccount         solanago.PublicKey
	TokenBAccount         solanago.PublicKey
	TokenAMint            solanago.PublicKey
	TokenBMint            solanago.PublicKey
	TokenAVault           solanago.PublicKey
	TokenBVault           solanago.PublicKey
	TokenAProgram         solanago.PublicKey
	TokenBProgram         solanago.PublicKey
	LiquidityDelta        *big.Int
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
}

type BuildLiquidatePositionInstructionParams struct {
	Owner                 solanago.PublicKey
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	PositionState         *PositionState
	PoolState             *PoolState
	TokenAAccount         solanago.PublicKey
	TokenBAccount         solanago.PublicKey
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
}

type BuildRemoveAllLiquidityInstructionParams struct {
	PoolAuthority         solanago.PublicKey
	Owner                 solanago.PublicKey
	Pool                  solanago.PublicKey
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	TokenAAccount         solanago.PublicKey
	TokenBAccount         solanago.PublicKey
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
	TokenAMint            solanago.PublicKey
	TokenBMint            solanago.PublicKey
	TokenAVault           solanago.PublicKey
	TokenBVault           solanago.PublicKey
	TokenAProgram         solanago.PublicKey
	TokenBProgram         solanago.PublicKey
}

type ClosePositionParams struct {
	Owner              solanago.PublicKey
	Pool               solanago.PublicKey
	Position           solanago.PublicKey
	PositionNftMint    solanago.PublicKey
	PositionNftAccount solanago.PublicKey
}

type RemoveAllLiquidityAndClosePositionParams struct {
	Owner                 solanago.PublicKey
	Position              solanago.PublicKey
	PositionNftAccount    solanago.PublicKey
	PoolState             *PoolState
	PositionState         *PositionState
	TokenAAmountThreshold *big.Int
	TokenBAmountThreshold *big.Int
	Vestings              []VestingWithAccount
	CurrentPoint          *big.Int
}

type MergePositionParams struct {
	Owner                                solanago.PublicKey
	PositionA                            solanago.PublicKey
	PositionB                            solanago.PublicKey
	PoolState                            *PoolState
	PositionBNftAccount                  solanago.PublicKey
	PositionANftAccount                  solanago.PublicKey
	PositionBState                       *PositionState
	TokenAAmountAddLiquidityThreshold    *big.Int
	TokenBAmountAddLiquidityThreshold    *big.Int
	TokenAAmountRemoveLiquidityThreshold *big.Int
	TokenBAmountRemoveLiquidityThreshold *big.Int
	PositionBVestings                    []VestingWithAccount
	CurrentPoint                         *big.Int
}

type GetQuoteParams struct {
	InAmount        *big.Int
	InputTokenMint  solanago.PublicKey
	Slippage        uint16
	PoolState       *PoolState
	CurrentPoint    *big.Int
	InputTokenInfo  *TokenInfo
	OutputTokenInfo *TokenInfo
	TokenADecimal   uint8
	TokenBDecimal   uint8
	HasReferral     bool
}

type GetQuote2Params struct {
	InputTokenMint  solanago.PublicKey
	Slippage        uint16
	CurrentPoint    *big.Int
	PoolState       *PoolState
	InputTokenInfo  *TokenInfo
	OutputTokenInfo *TokenInfo
	TokenADecimal   uint8
	TokenBDecimal   uint8
	HasReferral     bool
	SwapMode        SwapMode
	AmountIn        *big.Int
	AmountOut       *big.Int
}

type SwapAmount struct {
	AmountIn  *big.Int
	AmountOut *big.Int
}

// IdlSwapResult2 is the IDL-generated swap result type.
type IdlSwapResult2 = dammv2gen.SwapResult2

type SwapResult2 = shared.SwapResult2

type Quote2Result = shared.Quote2Result

// QuoteResult mirrors getQuote return structure.
type QuoteResult struct {
	SwapInAmount     *big.Int
	ConsumedInAmount *big.Int
	SwapOutAmount    *big.Int
	MinSwapOutAmount *big.Int
	TotalFee         *big.Int
	PriceImpact      decimal.Decimal
}

type SwapParams struct {
	Payer                solanago.PublicKey
	Pool                 solanago.PublicKey
	InputTokenMint       solanago.PublicKey
	OutputTokenMint      solanago.PublicKey
	AmountIn             *big.Int
	MinimumAmountOut     *big.Int
	ReferralTokenAccount *solanago.PublicKey
	Receiver             *solanago.PublicKey
	PoolState            *PoolState
}

type Swap2Params struct {
	Payer                solanago.PublicKey
	Pool                 solanago.PublicKey
	InputTokenMint       solanago.PublicKey
	OutputTokenMint      solanago.PublicKey
	ReferralTokenAccount *solanago.PublicKey
	Receiver             *solanago.PublicKey
	PoolState            *PoolState
	SwapMode             SwapMode
	AmountIn             *big.Int
	MinimumAmountOut     *big.Int
	AmountOut            *big.Int
	MaximumAmountIn      *big.Int
}

type LockPositionParams struct {
	Owner                solanago.PublicKey
	Payer                solanago.PublicKey
	Position             solanago.PublicKey
	PositionNftAccount   solanago.PublicKey
	Pool                 solanago.PublicKey
	CliffPoint           *big.Int
	PeriodFrequency      *big.Int
	CliffUnlockLiquidity *big.Int
	LiquidityPerPeriod   *big.Int
	NumberOfPeriod       uint16
	VestingAccount       *solanago.PublicKey
	InnerPosition        bool
}

type SetupFeeClaimAccountsParams struct {
	Payer           solanago.PublicKey
	Owner           solanago.PublicKey
	TokenAMint      solanago.PublicKey
	TokenBMint      solanago.PublicKey
	TokenAProgram   solanago.PublicKey
	TokenBProgram   solanago.PublicKey
	Receiver        *solanago.PublicKey
	TempWSolAccount *solanago.PublicKey
}

type ClaimPositionFeeInstructionParams struct {
	Owner              solanago.PublicKey
	Pool               solanago.PublicKey
	Position           solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	TokenAAccount      solanago.PublicKey
	TokenBAccount      solanago.PublicKey
	PoolState          *PoolState
	PoolAuthority      solanago.PublicKey
}

type ClaimPositionFeeParams struct {
	Owner              solanago.PublicKey
	Position           solanago.PublicKey
	Pool               solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	PoolState          *PoolState
	Receiver           *solanago.PublicKey
	FeePayer           *solanago.PublicKey
	TempWSolAccount    *solanago.PublicKey
}

type ClaimPositionFeeParams2 struct {
	Owner              solanago.PublicKey
	Position           solanago.PublicKey
	Pool               solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	PoolState          *PoolState
	Receiver           solanago.PublicKey
	FeePayer           *solanago.PublicKey
}

type ClosePositionInstructionParams struct {
	Owner              solanago.PublicKey
	Pool               solanago.PublicKey
	Position           solanago.PublicKey
	PositionNftMint    solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	PoolAuthority      solanago.PublicKey
}

type InitializeRewardParams struct {
	RewardIndex    uint8
	RewardDuration *big.Int
	Pool           solanago.PublicKey
	RewardMint     solanago.PublicKey
	Funder         solanago.PublicKey
	Payer          solanago.PublicKey
	Creator        solanago.PublicKey
}

type InitializeAndFundReward struct {
	RewardIndex    uint8
	RewardDuration *big.Int
	Pool           solanago.PublicKey
	Creator        solanago.PublicKey
	Payer          solanago.PublicKey
	RewardMint     solanago.PublicKey
	CarryForward   bool
	Amount         *big.Int
}

type UpdateRewardDurationParams struct {
	Pool        solanago.PublicKey
	Signer      solanago.PublicKey
	RewardIndex uint8
	NewDuration *big.Int
}

type UpdateRewardFunderParams struct {
	Pool        solanago.PublicKey
	Signer      solanago.PublicKey
	RewardIndex uint8
	NewFunder   solanago.PublicKey
}

type FundRewardParams struct {
	Funder       solanago.PublicKey
	RewardIndex  uint8
	Pool         solanago.PublicKey
	CarryForward bool
	Amount       *big.Int
	RewardMint   solanago.PublicKey
	RewardVault  solanago.PublicKey
}

type WithdrawIneligibleRewardParams struct {
	RewardIndex uint8
	Pool        solanago.PublicKey
	Funder      solanago.PublicKey
}

type ClaimPartnerFeeParams struct {
	Partner         solanago.PublicKey
	Pool            solanago.PublicKey
	MaxAmountA      *big.Int
	MaxAmountB      *big.Int
	Receiver        *solanago.PublicKey
	FeePayer        *solanago.PublicKey
	TempWSolAccount *solanago.PublicKey
}

type ClaimRewardParams struct {
	User               solanago.PublicKey
	Position           solanago.PublicKey
	Pool               solanago.PublicKey
	PoolState          *PoolState
	PositionNftAccount solanago.PublicKey
	RewardIndex        uint8
	IsSkipReward       bool
	FeePayer           *solanago.PublicKey
}

type RefreshVestingParams struct {
	Owner              solanago.PublicKey
	Pool               solanago.PublicKey
	Position           solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	VestingAccounts    []solanago.PublicKey
}

type VestingWithAccount struct {
	Account      solanago.PublicKey
	VestingState *VestingState
}

type PermanentLockParams struct {
	Owner              solanago.PublicKey
	Position           solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	Pool               solanago.PublicKey
	UnlockedLiquidity  *big.Int
}

type GetDepositQuoteParams struct {
	InAmount        *big.Int
	IsTokenA        bool
	MinSqrtPrice    *big.Int
	MaxSqrtPrice    *big.Int
	SqrtPrice       *big.Int
	InputTokenInfo  *TokenInfo
	OutputTokenInfo *TokenInfo
}

type GetWithdrawQuoteParams struct {
	LiquidityDelta  *big.Int
	MinSqrtPrice    *big.Int
	MaxSqrtPrice    *big.Int
	SqrtPrice       *big.Int
	TokenATokenInfo *TokenInfo
	TokenBTokenInfo *TokenInfo
}

type DepositQuote struct {
	ActualInputAmount   *big.Int
	ConsumedInputAmount *big.Int
	OutputAmount        *big.Int
	LiquidityDelta      *big.Int
}

type WithdrawQuote struct {
	LiquidityDelta *big.Int
	OutAmountA     *big.Int
	OutAmountB     *big.Int
}

type DynamicFeeParams struct {
	VolatilityAccumulator *big.Int
	BinStep               uint16
	VariableFeeControl    uint64
}

type SplitPositionParams struct {
	FirstPositionOwner                 solanago.PublicKey
	SecondPositionOwner                solanago.PublicKey
	Pool                               solanago.PublicKey
	FirstPosition                      solanago.PublicKey
	FirstPositionNftAccount            solanago.PublicKey
	SecondPosition                     solanago.PublicKey
	SecondPositionNftAccount           solanago.PublicKey
	PermanentLockedLiquidityPercentage uint8
	UnlockedLiquidityPercentage        uint8
	FeeAPercentage                     uint8
	FeeBPercentage                     uint8
	Reward0Percentage                  uint8
	Reward1Percentage                  uint8
	InnerVestingLiquidityPercentage    uint8
}

type SplitPosition2Params struct {
	FirstPositionOwner       solanago.PublicKey
	SecondPositionOwner      solanago.PublicKey
	Pool                     solanago.PublicKey
	FirstPosition            solanago.PublicKey
	FirstPositionNftAccount  solanago.PublicKey
	SecondPosition           solanago.PublicKey
	SecondPositionNftAccount solanago.PublicKey
	Numerator                uint32
}

// Interfaces from TS.
type BaseFeeHandler = shared.BaseFeeHandler

type FeeOnAmountResult = shared.FeeOnAmountResult

type SplitFees = shared.SplitFees

type PositionNftAccount = helpers.PositionNftAccount

// Optional numeric wrappers used in fee utils.
type Decimal = decimal.Decimal
