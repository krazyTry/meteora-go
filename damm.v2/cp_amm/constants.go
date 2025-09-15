package cp_amm

import (
	"math/big"

	dmath "github.com/krazyTry/meteora-go/decimal_math"
	"github.com/shopspring/decimal"
)

// Account key constants for different account types in the CP AMM program
var (
	// AccountKeyClaimFeeOperator is the account key for claim fee operator accounts
	AccountKeyClaimFeeOperator = "ClaimFeeOperator"
	// AccountKeyConfig is the account key for configuration accounts
	AccountKeyConfig           = "Config"
	// AccountKeyPool is the account key for liquidity pool accounts
	AccountKeyPool             = "Pool"
	// AccountKeyPosition is the account key for position accounts
	AccountKeyPosition         = "Position"
	// AccountKeyTokenBadge is the account key for token badge accounts
	AccountKeyTokenBadge       = "TokenBadge"
	// AccountKeyVesting is the account key for vesting accounts
	AccountKeyVesting          = "Vesting"
)

// Fee calculation and mathematical constants for the CP AMM
var (
	// DYNAMIC_FEE_FILTER_PERIOD_DEFAULT is the default filter period for dynamic fee calculation
	DYNAMIC_FEE_FILTER_PERIOD_DEFAULT    uint16 = 10
	// DYNAMIC_FEE_DECAY_PERIOD_DEFAULT is the default decay period for dynamic fee calculation
	DYNAMIC_FEE_DECAY_PERIOD_DEFAULT     uint16 = 120
	// DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT is the default reduction factor for dynamic fees (50%)
	DYNAMIC_FEE_REDUCTION_FACTOR_DEFAULT uint16 = 5000 // 50%

	// BASIS_POINT_MAX represents the maximum basis points (10,000 = 100%)
	BASIS_POINT_MAX = decimal.NewFromInt(10_000)

	// MAX_FEE_NUMERATOR is the maximum fee numerator allowed
	MAX_FEE_NUMERATOR = decimal.NewFromInt(500_000_000)

	// FEE_DENOMINATOR is the denominator used for fee calculations
	FEE_DENOMINATOR = decimal.NewFromInt(1_000_000_000)

	// MAX_FEE_BASIS_POINTS is the maximum fee in basis points
	MAX_FEE_BASIS_POINTS uint16 = 10000

	// ONE_IN_BASIS_POINTS represents one unit in basis points
	ONE_IN_BASIS_POINTS = decimal.NewFromUint64(uint64(MAX_FEE_BASIS_POINTS))

	// BIN_STEP_BPS_DEFAULT is the default bin step in basis points
	BIN_STEP_BPS_DEFAULT = decimal.NewFromInt(1)

	// BIN_STEP_BPS_U128_DEFAULT is the default bin step in U128 format (bin_step << 64 / BASIS_POINT_MAX)
	BIN_STEP_BPS_U128_DEFAULT = decimal.NewFromInt(1844674407370955)

	// MIN_SQRT_PRICE is the minimum square root price allowed
	MIN_SQRT_PRICE    = big.NewInt(4295048016)
	// MAX_SQRT_PRICE is the maximum square root price allowed
	MAX_SQRT_PRICE, _ = new(big.Int).SetString("79226673521066979257578248091", 10)

	// MAX_PRICE_CHANGE_BPS_DEFAULT is the default maximum price change in basis points (15%)
	MAX_PRICE_CHANGE_BPS_DEFAULT = 1500 // 15%

	// MAX_EXPONENTIAL is the maximum exponential value for calculations
	MAX_EXPONENTIAL = decimal.NewFromInt(0x80000)

	// Common decimal constants for mathematical operations
	N0     = decimal.Zero
	N1     = decimal.NewFromInt(1)
	N2     = decimal.NewFromInt(2)
	N10    = decimal.NewFromInt(10)
	N20    = decimal.NewFromInt(20)
	N100   = decimal.NewFromInt(100)
	N128   = decimal.NewFromInt(128)
	N10000 = decimal.NewFromInt(10000)

	// Large number constants for boundary calculations
	N99_999_999_999  = decimal.NewFromInt(99_999_999_999)
	N100_000_000_000 = decimal.NewFromInt(100_000_000_000)

	// MAX represents the maximum value for 128-bit calculations (2^128 - 1)
	MAX = dmath.Exp(N2, N128, decimal.NullDecimal{}).Sub(N1) // 2^128 - 1

	// Q64 represents 2^64 for fixed-point arithmetic
	Q64  = dmath.Lsh(N1, 64)
	// Q128 represents 2^128 for fixed-point arithmetic
	Q128 = dmath.Lsh(N1, 128)
)

// TokenType represents the type of token used in the pool
type TokenType uint8

const (
	// TokenTypeSPL represents standard SPL tokens
	TokenTypeSPL TokenType = iota
	// TokenTypeToken2022 represents Token-2022 program tokens
	TokenTypeToken2022
)

// ActivationType represents the type of activation mechanism used
type ActivationType uint8

const (
	// ActivationTypeSlot represents activation based on slot number
	ActivationTypeSlot ActivationType = iota
	// ActivationTypeTimestamp represents activation based on timestamp
	ActivationTypeTimestamp
)

// FeeSchedulerMode represents the mode of fee scheduling calculation
type FeeSchedulerMode uint8

const (
	// FeeSchedulerModeLinear represents linear fee scheduling
	FeeSchedulerModeLinear FeeSchedulerMode = iota
	// FeeSchedulerModeExponential represents exponential fee scheduling
	FeeSchedulerModeExponential
)

// CollectFeeMode represents the mode for collecting fees from the pool
type CollectFeeMode uint8

const (
	// CollectFeeModeBothToken collects fees from both tokens (0)
	CollectFeeModeBothToken CollectFeeMode = iota // 0
	// CollectFeeModeOnlyA collects fees only from token A (1)
	CollectFeeModeOnlyA                           // 1
	// CollectFeeModeOnlyB collects fees only from token B (2)
	CollectFeeModeOnlyB                           // 2
)
