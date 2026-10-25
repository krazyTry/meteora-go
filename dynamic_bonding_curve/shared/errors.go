package shared

// Program error text reproduced by the SDK. The strings match
// dynamic-bonding-curve PoolError messages.
const (
	ErrMathOverflow                 = "Math operation overflow"
	ErrExceededSlippage             = "Exceeded slippage tolerance"
	ErrExceededMaxFeeBps            = "Exceeded max fee bps"
	ErrAmountIsZero                 = "Amount is zero"
	ErrTypeCastFailed               = "Type cast error"
	ErrInvalidCollectFeeMode        = "Invalid collect fee mode"
	ErrInvalidMigrationOption       = "Invalid migration option"
	ErrInvalidInput                 = "Invalid input"
	ErrNotEnoughLiquidity           = "Not enough liquidity"
	ErrPoolIsCompleted              = "Pool is completed"
	ErrInvalidCurve                 = "Invalid curve"
	ErrInsufficientLiquidity        = "Liquidity in bonding curve is insufficient"
	ErrInvalidBaseFeeMode           = "Invalid base fee mode"
	ErrUndeterminedError            = "Undetermined error"
	ErrAmountLeftIsNotZero          = "Amount left is not zero"
	ErrFeeInverseIsIncorrect        = "Transfer fee inverse calculation is incorrect"
	ErrExceededInputAmount          = "Required input amount exceeds the provided amount in"
	ErrInvalidFeeClaimer            = "Invalid fee claimer"
	ErrInvalidMigrationFeeOption    = "Invalid migration fee option"
	ErrInvalidTokenSupply           = "Invalid token supply"
	ErrInvalidVestingParameters     = "Invalid vesting parameters"
	ErrInvalidMigratedPoolFee       = "Invalid migrated pool fee params"
	ErrInvalidTransferFeeParameters = "Invalid transfer fee parameters"
)
