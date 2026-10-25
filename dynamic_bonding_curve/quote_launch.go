package dynamic_bonding_curve

import (
	"fmt"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcmath "github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
)

// SimulatedQuoteParams quotes a buildCurve config as if its pool had just
// been initialized at CurrentPoint.
type SimulatedQuoteParams struct {
	Config                         ConfigParameters
	SwapBaseForQuote               bool
	SwapMode                       SwapMode
	AmountIn                       *big.Int
	AmountOut                      *big.Int
	SlippageBps                    uint16
	HasReferral                    bool
	EligibleForFirstSwapWithMinFee bool
	CurrentPoint                   *big.Int
	QuoteTokenFlag                 uint8
	TransferFees                   *QuoteTransferFees
}

// GetQuoteFromInputAmount quotes an exact-in or partial-fill swap on a curve
// that has not been initialized on chain yet.
func GetQuoteFromInputAmount(params SimulatedQuoteParams) (SwapQuote2Result, error) {
	if params.SwapMode == 0 {
		params.SwapMode = SwapModeExactIn
	}
	return quoteAtInitialization(params)
}

// GetQuoteFromOutputAmount quotes an exact-out swap on a curve that has not
// been initialized on chain yet.
func GetQuoteFromOutputAmount(params SimulatedQuoteParams) (SwapQuote2Result, error) {
	params.SwapMode = SwapModeExactOut
	return quoteAtInitialization(params)
}

func quoteAtInitialization(params SimulatedQuoteParams) (SwapQuote2Result, error) {
	current := params.CurrentPoint
	if current == nil {
		current = big.NewInt(0)
	}
	if !current.IsUint64() {
		return SwapQuote2Result{}, fmt.Errorf("current point exceeds u64")
	}
	config, err := poolConfigFromParameters(params.Config, params.QuoteTokenFlag)
	if err != nil {
		return SwapQuote2Result{}, err
	}
	pool := &VirtualPool{}
	pool.PoolState.SqrtPrice = params.Config.SqrtStartPrice
	pool.PoolState.ActivationPoint = current.Uint64()

	switch params.SwapMode {
	case SwapModeExactOut:
		return dbcmath.SwapQuoteExactOut(pool, config, params.SwapBaseForQuote, params.AmountOut, params.SlippageBps, params.HasReferral, current, params.EligibleForFirstSwapWithMinFee, params.TransferFees)
	case SwapModePartialFill:
		return dbcmath.SwapQuotePartialFill(pool, config, params.SwapBaseForQuote, params.AmountIn, params.SlippageBps, params.HasReferral, current, params.EligibleForFirstSwapWithMinFee, params.TransferFees)
	default:
		return dbcmath.SwapQuoteExactIn(pool, config, params.SwapBaseForQuote, params.AmountIn, params.SlippageBps, params.HasReferral, current, params.EligibleForFirstSwapWithMinFee, params.TransferFees)
	}
}

func poolConfigFromParameters(config ConfigParameters, quoteTokenFlag uint8) (*PoolConfig, error) {
	migrationSqrt, err := helpers.GetMigrationThresholdPrice(new(big.Int).SetUint64(config.MigrationQuoteThreshold), config.SqrtStartPrice.BigInt(), config.Curve)
	if err != nil {
		return nil, err
	}
	out := &PoolConfig{}
	out.PoolFees.BaseFee = dbcidl.BaseFeeConfig{
		CliffFeeNumerator: config.PoolFees.BaseFee.CliffFeeNumerator,
		SecondFactor:      config.PoolFees.BaseFee.SecondFactor,
		ThirdFactor:       config.PoolFees.BaseFee.ThirdFactor,
		FirstFactor:       config.PoolFees.BaseFee.FirstFactor,
		BaseFeeMode:       config.PoolFees.BaseFee.BaseFeeMode,
	}
	if fee := config.PoolFees.DynamicFee; fee != nil {
		out.PoolFees.DynamicFee = dbcidl.DynamicFeeConfig{
			Initialized:              1,
			MaxVolatilityAccumulator: fee.MaxVolatilityAccumulator,
			VariableFeeControl:       fee.VariableFeeControl,
			BinStep:                  fee.BinStep,
			FilterPeriod:             fee.FilterPeriod,
			DecayPeriod:              fee.DecayPeriod,
			ReductionFactor:          fee.ReductionFactor,
			BinStepU128:              fee.BinStepU128,
		}
	}
	out.CollectFeeMode = config.CollectFeeMode
	out.SqrtStartPrice = config.SqrtStartPrice
	out.MigrationQuoteThreshold = config.MigrationQuoteThreshold
	out.MigrationSqrtPrice = helpers.BigToU128(migrationSqrt)
	for i := 0; i < len(config.Curve) && i < len(out.Curve); i++ {
		out.Curve[i] = dbcidl.LiquidityDistributionConfig{
			SqrtPrice: config.Curve[i].SqrtPrice,
			Liquidity: config.Curve[i].Liquidity,
		}
	}
	if config.EnableFirstSwapWithMinFee {
		out.EnableFirstSwapWithMinFee = 1
	}
	out.QuoteTokenFlag = quoteTokenFlag
	return out, nil
}
