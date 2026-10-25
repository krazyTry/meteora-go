package math

import (
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

type Rounding = shared.Rounding

type SwapAmountFromInput struct {
	OutputAmount  *big.Int
	NextSqrtPrice *big.Int
	AmountLeft    *big.Int
}

type SwapAmountFromOutput struct {
	InputAmount   *big.Int
	NextSqrtPrice *big.Int
}

type InitialPoolInformation struct {
	TokenAAmount     *big.Int
	TokenBAmount     *big.Int
	SqrtPrice        *big.Int
	InitialLiquidity *big.Int
	SqrtMinPrice     *big.Int
	SqrtMaxPrice     *big.Int
}

type LiquidityHandler interface {
	GetAmountsForModifyLiquidity(liquidityDelta *big.Int, round shared.Rounding) (*big.Int, *big.Int)
	CalculateAtoBFromAmountIn(amountIn *big.Int) (SwapAmountFromInput, error)
	CalculateBtoAFromAmountIn(amountIn *big.Int) (SwapAmountFromInput, error)
	CalculateAtoBFromPartialAmountIn(amountIn *big.Int) (SwapAmountFromInput, error)
	CalculateBtoAFromPartialAmountIn(amountIn *big.Int) (SwapAmountFromInput, error)
	CalculateAtoBFromAmountOut(amountOut *big.Int) (SwapAmountFromOutput, error)
	CalculateBtoAFromAmountOut(amountOut *big.Int) (SwapAmountFromOutput, error)
	GetReservesAmount() (*big.Int, *big.Int)
	GetNextSqrtPrice(nextSqrtPrice *big.Int) *big.Int
	GetMaxAmountIn(tradeDirection shared.TradeDirection) *big.Int
}

type compoundingLiquidityHandler struct {
	tokenAAmount *big.Int
	tokenBAmount *big.Int
	liquidity    *big.Int
}

func (h compoundingLiquidityHandler) GetAmountsForModifyLiquidity(liquidityDelta *big.Int, round shared.Rounding) (*big.Int, *big.Int) {
	return GetAmountsForModifyForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, h.liquidity, liquidityDelta, round)
}

func (h compoundingLiquidityHandler) CalculateAtoBFromAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	return CalculateAtoBFromAmountInForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, amountIn)
}

func (h compoundingLiquidityHandler) CalculateBtoAFromAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	return CalculateBtoAFromAmountInForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, amountIn)
}

func (h compoundingLiquidityHandler) CalculateAtoBFromPartialAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	return CalculateAtoBFromAmountInForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, amountIn)
}

func (h compoundingLiquidityHandler) CalculateBtoAFromPartialAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	return CalculateBtoAFromAmountInForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, amountIn)
}

func (h compoundingLiquidityHandler) CalculateAtoBFromAmountOut(amountOut *big.Int) (SwapAmountFromOutput, error) {
	return CalculateAtoBFromAmountOutForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, amountOut)
}

func (h compoundingLiquidityHandler) CalculateBtoAFromAmountOut(amountOut *big.Int) (SwapAmountFromOutput, error) {
	return CalculateBtoAFromAmountOutForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount, amountOut)
}

func (h compoundingLiquidityHandler) GetReservesAmount() (*big.Int, *big.Int) {
	return new(big.Int).Set(h.tokenAAmount), new(big.Int).Set(h.tokenBAmount)
}

func (h compoundingLiquidityHandler) GetNextSqrtPrice(_ *big.Int) *big.Int {
	price, err := GetSqrtPriceFromAmountsForCompoundingLiquidity(h.tokenAAmount, h.tokenBAmount)
	if err != nil {
		return big.NewInt(0)
	}
	return price
}

func (h compoundingLiquidityHandler) GetMaxAmountIn(_ shared.TradeDirection) *big.Int {
	return new(big.Int).Set(shared.U64Max)
}

type concentratedLiquidityHandler struct {
	sqrtMaxPrice *big.Int
	sqrtMinPrice *big.Int
	sqrtPrice    *big.Int
	liquidity    *big.Int
}

func (h concentratedLiquidityHandler) GetAmountsForModifyLiquidity(liquidityDelta *big.Int, round shared.Rounding) (*big.Int, *big.Int) {
	amountA := amountAFromLiquidityDeltaConcentrated(h.sqrtPrice, h.sqrtMaxPrice, liquidityDelta, round)
	amountB := amountBFromLiquidityDeltaConcentrated(h.sqrtMinPrice, h.sqrtPrice, liquidityDelta, round)
	return amountA, amountB
}

func (h concentratedLiquidityHandler) CalculateAtoBFromAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	nextSqrtPrice, err := GetNextSqrtPriceFromInput(h.sqrtPrice, h.liquidity, amountIn, true)
	if err != nil {
		return SwapAmountFromInput{}, err
	}
	if nextSqrtPrice.Cmp(h.sqrtMinPrice) < 0 {
		return SwapAmountFromInput{}, errors.New("price range is violated")
	}
	outputAmount := amountBFromLiquidityDeltaConcentrated(nextSqrtPrice, h.sqrtPrice, h.liquidity, shared.RoundingDown)
	return SwapAmountFromInput{OutputAmount: outputAmount, NextSqrtPrice: nextSqrtPrice, AmountLeft: big.NewInt(0)}, nil
}

func (h concentratedLiquidityHandler) CalculateBtoAFromAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	nextSqrtPrice, err := GetNextSqrtPriceFromInput(h.sqrtPrice, h.liquidity, amountIn, false)
	if err != nil {
		return SwapAmountFromInput{}, err
	}
	if nextSqrtPrice.Cmp(h.sqrtMaxPrice) > 0 {
		return SwapAmountFromInput{}, errors.New("price range is violated")
	}
	outputAmount := amountAFromLiquidityDeltaConcentrated(h.sqrtPrice, nextSqrtPrice, h.liquidity, shared.RoundingDown)
	return SwapAmountFromInput{OutputAmount: outputAmount, NextSqrtPrice: nextSqrtPrice, AmountLeft: big.NewInt(0)}, nil
}

func (h concentratedLiquidityHandler) CalculateAtoBFromPartialAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	maxAmountIn := amountAFromLiquidityDeltaConcentrated(h.sqrtMinPrice, h.sqrtPrice, h.liquidity, shared.RoundingUp)
	consumed := new(big.Int)
	nextSqrt := new(big.Int)
	if amountIn.Cmp(maxAmountIn) >= 0 {
		consumed.Set(maxAmountIn)
		nextSqrt.Set(h.sqrtMinPrice)
	} else {
		next, err := GetNextSqrtPriceFromInput(h.sqrtPrice, h.liquidity, amountIn, true)
		if err != nil {
			return SwapAmountFromInput{}, err
		}
		nextSqrt.Set(next)
		consumed.Set(amountIn)
	}
	outputAmount := amountBFromLiquidityDeltaConcentrated(nextSqrt, h.sqrtPrice, h.liquidity, shared.RoundingDown)
	return SwapAmountFromInput{OutputAmount: outputAmount, NextSqrtPrice: nextSqrt, AmountLeft: new(big.Int).Sub(amountIn, consumed)}, nil
}

func (h concentratedLiquidityHandler) CalculateBtoAFromPartialAmountIn(amountIn *big.Int) (SwapAmountFromInput, error) {
	maxAmountIn := amountBFromLiquidityDeltaConcentrated(h.sqrtPrice, h.sqrtMaxPrice, h.liquidity, shared.RoundingUp)
	consumed := new(big.Int)
	nextSqrt := new(big.Int)
	if amountIn.Cmp(maxAmountIn) >= 0 {
		consumed.Set(maxAmountIn)
		nextSqrt.Set(h.sqrtMaxPrice)
	} else {
		next, err := GetNextSqrtPriceFromInput(h.sqrtPrice, h.liquidity, amountIn, false)
		if err != nil {
			return SwapAmountFromInput{}, err
		}
		nextSqrt.Set(next)
		consumed.Set(amountIn)
	}
	outputAmount := amountAFromLiquidityDeltaConcentrated(h.sqrtPrice, nextSqrt, h.liquidity, shared.RoundingDown)
	return SwapAmountFromInput{OutputAmount: outputAmount, NextSqrtPrice: nextSqrt, AmountLeft: new(big.Int).Sub(amountIn, consumed)}, nil
}

func (h concentratedLiquidityHandler) CalculateAtoBFromAmountOut(amountOut *big.Int) (SwapAmountFromOutput, error) {
	nextSqrtPrice, err := GetNextSqrtPriceFromOutput(h.sqrtPrice, h.liquidity, amountOut, true)
	if err != nil {
		return SwapAmountFromOutput{}, err
	}
	if nextSqrtPrice.Cmp(h.sqrtMinPrice) < 0 {
		return SwapAmountFromOutput{}, errors.New("price range is violated")
	}
	inputAmount := amountAFromLiquidityDeltaConcentrated(nextSqrtPrice, h.sqrtPrice, h.liquidity, shared.RoundingUp)
	return SwapAmountFromOutput{InputAmount: inputAmount, NextSqrtPrice: nextSqrtPrice}, nil
}

func (h concentratedLiquidityHandler) CalculateBtoAFromAmountOut(amountOut *big.Int) (SwapAmountFromOutput, error) {
	nextSqrtPrice, err := GetNextSqrtPriceFromOutput(h.sqrtPrice, h.liquidity, amountOut, false)
	if err != nil {
		return SwapAmountFromOutput{}, err
	}
	if nextSqrtPrice.Cmp(h.sqrtMaxPrice) > 0 {
		return SwapAmountFromOutput{}, errors.New("price range is violated")
	}
	inputAmount := amountBFromLiquidityDeltaConcentrated(h.sqrtPrice, nextSqrtPrice, h.liquidity, shared.RoundingUp)
	return SwapAmountFromOutput{InputAmount: inputAmount, NextSqrtPrice: nextSqrtPrice}, nil
}

func (h concentratedLiquidityHandler) GetReservesAmount() (*big.Int, *big.Int) {
	reserveA := amountAFromLiquidityDeltaConcentrated(h.sqrtPrice, h.sqrtMaxPrice, h.liquidity, shared.RoundingUp)
	reserveB := amountBFromLiquidityDeltaConcentrated(h.sqrtMinPrice, h.sqrtPrice, h.liquidity, shared.RoundingUp)
	return reserveA, reserveB
}

func (h concentratedLiquidityHandler) GetNextSqrtPrice(nextSqrtPrice *big.Int) *big.Int {
	return new(big.Int).Set(nextSqrtPrice)
}

func (h concentratedLiquidityHandler) GetMaxAmountIn(tradeDirection shared.TradeDirection) *big.Int {
	var amount *big.Int
	if tradeDirection == shared.TradeDirectionAtoB {
		amount = amountAFromLiquidityDeltaConcentrated(h.sqrtMinPrice, h.sqrtPrice, h.liquidity, shared.RoundingUp)
	} else {
		amount = amountBFromLiquidityDeltaConcentrated(h.sqrtPrice, h.sqrtMaxPrice, h.liquidity, shared.RoundingUp)
	}
	if amount.Cmp(shared.U64Max) > 0 {
		return new(big.Int).Set(shared.U64Max)
	}
	return amount
}

func copyBig(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(v)
}

func GetLiquidityHandler(pool *dammv2gen.Pool) LiquidityHandler {
	if shared.CollectFeeMode(pool.CollectFeeMode) == shared.CollectFeeModeCompounding {
		return compoundingLiquidityHandler{
			tokenAAmount: new(big.Int).SetUint64(pool.TokenAAmount),
			tokenBAmount: new(big.Int).SetUint64(pool.TokenBAmount),
			liquidity:    copyBig(pool.Liquidity.BigInt()),
		}
	}
	return concentratedLiquidityHandler{
		sqrtMaxPrice: copyBig(pool.SqrtMaxPrice.BigInt()),
		sqrtMinPrice: copyBig(pool.SqrtMinPrice.BigInt()),
		sqrtPrice:    copyBig(pool.SqrtPrice.BigInt()),
		liquidity:    copyBig(pool.Liquidity.BigInt()),
	}
}

func GetInitialCompoundingPoolInformation(sqrtPrice, liquidity *big.Int) (InitialPoolInformation, error) {
	if liquidity.Cmp(shared.DeadLiquidity) <= 0 {
		return InitialPoolInformation{}, errors.New("invalid minimum liquidity")
	}
	tokenA := GetPoolCreationAmountAFromLiquidityDeltaForCompounding(sqrtPrice, liquidity)
	tokenB := GetPoolCreationAmountBFromLiquidityDeltaForCompounding(sqrtPrice, liquidity)
	price, err := GetSqrtPriceFromAmountsForCompoundingLiquidity(tokenA, tokenB)
	if err != nil {
		return InitialPoolInformation{}, err
	}
	return InitialPoolInformation{
		TokenAAmount:     tokenA,
		TokenBAmount:     tokenB,
		SqrtPrice:        price,
		InitialLiquidity: new(big.Int).Sub(liquidity, shared.DeadLiquidity),
		SqrtMinPrice:     big.NewInt(0),
		SqrtMaxPrice:     new(big.Int).Set(shared.U128Max),
	}, nil
}

func GetInitialConcentratedLiquidityPoolInformation(sqrtMinPrice, sqrtMaxPrice, sqrtPrice, liquidity *big.Int) InitialPoolInformation {
	return InitialPoolInformation{
		TokenAAmount:     amountAFromLiquidityDeltaConcentrated(sqrtPrice, sqrtMaxPrice, liquidity, shared.RoundingUp),
		TokenBAmount:     amountBFromLiquidityDeltaConcentrated(sqrtMinPrice, sqrtPrice, liquidity, shared.RoundingUp),
		SqrtPrice:        new(big.Int).Set(sqrtPrice),
		InitialLiquidity: new(big.Int).Set(liquidity),
		SqrtMinPrice:     new(big.Int).Set(sqrtMinPrice),
		SqrtMaxPrice:     new(big.Int).Set(sqrtMaxPrice),
	}
}

func GetInitialPoolInformation(collectFeeMode shared.CollectFeeMode, sqrtMinPrice, sqrtMaxPrice, sqrtPrice, liquidity *big.Int) (InitialPoolInformation, error) {
	if collectFeeMode == shared.CollectFeeModeCompounding {
		return GetInitialCompoundingPoolInformation(sqrtPrice, liquidity)
	}
	return GetInitialConcentratedLiquidityPoolInformation(sqrtMinPrice, sqrtMaxPrice, sqrtPrice, liquidity), nil
}

func GetAmountsForModifyForCompoundingLiquidity(tokenAAmount, tokenBAmount, liquidity, liquidityDelta *big.Int, round shared.Rounding) (*big.Int, *big.Int) {
	return MulDiv(liquidityDelta, tokenAAmount, liquidity, round), MulDiv(liquidityDelta, tokenBAmount, liquidity, round)
}

func CalculateAtoBFromAmountInForCompoundingLiquidity(tokenAAmount, tokenBAmount, amountIn *big.Int) (SwapAmountFromInput, error) {
	denominator := new(big.Int).Add(tokenAAmount, amountIn)
	output := MulDiv(tokenBAmount, amountIn, denominator, shared.RoundingDown)
	return SwapAmountFromInput{AmountLeft: big.NewInt(0), OutputAmount: output, NextSqrtPrice: big.NewInt(0)}, nil
}

func CalculateBtoAFromAmountInForCompoundingLiquidity(tokenAAmount, tokenBAmount, amountIn *big.Int) (SwapAmountFromInput, error) {
	denominator := new(big.Int).Add(tokenBAmount, amountIn)
	output := MulDiv(tokenAAmount, amountIn, denominator, shared.RoundingDown)
	return SwapAmountFromInput{AmountLeft: big.NewInt(0), OutputAmount: output, NextSqrtPrice: big.NewInt(0)}, nil
}

func CalculateAtoBFromAmountOutForCompoundingLiquidity(tokenAAmount, tokenBAmount, amountOut *big.Int) (SwapAmountFromOutput, error) {
	if amountOut.Cmp(tokenBAmount) >= 0 {
		return SwapAmountFromOutput{}, errors.New("insufficient liquidity")
	}
	input := MulDiv(tokenAAmount, amountOut, new(big.Int).Sub(tokenBAmount, amountOut), shared.RoundingUp)
	return SwapAmountFromOutput{InputAmount: input, NextSqrtPrice: big.NewInt(0)}, nil
}

func CalculateBtoAFromAmountOutForCompoundingLiquidity(tokenAAmount, tokenBAmount, amountOut *big.Int) (SwapAmountFromOutput, error) {
	if amountOut.Cmp(tokenAAmount) >= 0 {
		return SwapAmountFromOutput{}, errors.New("insufficient liquidity")
	}
	input := MulDiv(tokenBAmount, amountOut, new(big.Int).Sub(tokenAAmount, amountOut), shared.RoundingUp)
	return SwapAmountFromOutput{InputAmount: input, NextSqrtPrice: big.NewInt(0)}, nil
}

func GetNextSqrtPriceForCompoundingLiquidity(tokenAAmount, tokenBAmount *big.Int) (*big.Int, error) {
	return GetSqrtPriceFromAmountsForCompoundingLiquidity(tokenAAmount, tokenBAmount)
}

func GetSqrtPriceFromAmountsForCompoundingLiquidity(tokenAAmount, tokenBAmount *big.Int) (*big.Int, error) {
	if tokenAAmount == nil || tokenAAmount.Sign() <= 0 || tokenBAmount == nil || tokenBAmount.Sign() < 0 {
		return nil, errors.New("math overflow")
	}
	shifted := new(big.Int).Lsh(new(big.Int).Set(tokenBAmount), 128)
	price := shifted.Div(shifted, tokenAAmount)
	return Sqrt(price), nil
}

func GetAmountAFromLiquidityDeltaForCompounding(liquidityDelta, tokenAAmount, liquidity *big.Int, rounding shared.Rounding) *big.Int {
	return MulDiv(liquidityDelta, tokenAAmount, liquidity, rounding)
}

func GetAmountBFromLiquidityDeltaForCompounding(liquidityDelta, tokenBAmount, liquidity *big.Int, rounding shared.Rounding) *big.Int {
	return MulDiv(liquidityDelta, tokenBAmount, liquidity, rounding)
}

func GetPoolCreationAmountAFromLiquidityDeltaForCompounding(sqrtPrice, liquidity *big.Int) *big.Int {
	numerator := new(big.Int).Add(liquidity, new(big.Int).Sub(sqrtPrice, big.NewInt(1)))
	return numerator.Div(numerator, sqrtPrice)
}

func GetPoolCreationAmountBFromLiquidityDeltaForCompounding(sqrtPrice, liquidity *big.Int) *big.Int {
	numerator := new(big.Int).Mul(liquidity, sqrtPrice)
	denominator := new(big.Int).Lsh(big.NewInt(1), 128)
	numerator.Add(numerator, new(big.Int).Sub(denominator, big.NewInt(1)))
	return numerator.Div(numerator, denominator)
}

func GetLiquidityDeltaFromAmountAForCompounding(amountA, tokenAAmount, liquidity *big.Int) *big.Int {
	return MulDiv(amountA, liquidity, tokenAAmount, shared.RoundingDown)
}

func GetLiquidityDeltaFromAmountBForCompounding(amountB, tokenBAmount, liquidity *big.Int) *big.Int {
	return MulDiv(amountB, liquidity, tokenBAmount, shared.RoundingDown)
}

func GetPoolCreationLiquidityDeltaFromAmountAForCompounding(amountA, sqrtPrice *big.Int) *big.Int {
	return new(big.Int).Mul(amountA, sqrtPrice)
}

func GetPoolCreationLiquidityDeltaFromAmountBForCompounding(amountB, sqrtPrice *big.Int) *big.Int {
	return new(big.Int).Div(new(big.Int).Lsh(new(big.Int).Set(amountB), 128), sqrtPrice)
}

func GetLiquidityDeltaFromAmountA(amountA, sqrtPrice, sqrtMaxPrice *big.Int, collectFeeMode shared.CollectFeeMode, tokenAAmount, liquidity *big.Int) *big.Int {
	if collectFeeMode == shared.CollectFeeModeCompounding {
		if tokenAAmount != nil && liquidity != nil {
			return GetLiquidityDeltaFromAmountAForCompounding(amountA, tokenAAmount, liquidity)
		}
		return GetPoolCreationLiquidityDeltaFromAmountAForCompounding(amountA, sqrtPrice)
	}
	return liquidityDeltaFromAmountAConcentrated(amountA, sqrtPrice, sqrtMaxPrice)
}

func GetLiquidityDeltaFromAmountB(amountB, sqrtMinPrice, sqrtPrice *big.Int, collectFeeMode shared.CollectFeeMode, tokenBAmount, liquidity *big.Int) *big.Int {
	if collectFeeMode == shared.CollectFeeModeCompounding {
		if tokenBAmount != nil && liquidity != nil {
			return GetLiquidityDeltaFromAmountBForCompounding(amountB, tokenBAmount, liquidity)
		}
		return GetPoolCreationLiquidityDeltaFromAmountBForCompounding(amountB, sqrtPrice)
	}
	return liquidityDeltaFromAmountBConcentrated(amountB, sqrtMinPrice, sqrtPrice)
}

func GetAmountAFromLiquidityDelta(sqrtPrice, sqrtMaxPrice, liquidityDelta *big.Int, rounding shared.Rounding, collectFeeMode shared.CollectFeeMode, tokenAAmount, liquidity *big.Int) *big.Int {
	if collectFeeMode == shared.CollectFeeModeCompounding {
		if tokenAAmount != nil && liquidity != nil {
			return GetAmountAFromLiquidityDeltaForCompounding(liquidityDelta, tokenAAmount, liquidity, rounding)
		}
		return GetPoolCreationAmountAFromLiquidityDeltaForCompounding(sqrtPrice, liquidityDelta)
	}
	return amountAFromLiquidityDeltaConcentrated(sqrtPrice, sqrtMaxPrice, liquidityDelta, rounding)
}

func GetAmountBFromLiquidityDelta(sqrtMinPrice, sqrtPrice, liquidityDelta *big.Int, rounding shared.Rounding, collectFeeMode shared.CollectFeeMode, tokenBAmount, liquidity *big.Int) *big.Int {
	if collectFeeMode == shared.CollectFeeModeCompounding {
		if tokenBAmount != nil && liquidity != nil {
			return GetAmountBFromLiquidityDeltaForCompounding(liquidityDelta, tokenBAmount, liquidity, rounding)
		}
		return GetPoolCreationAmountBFromLiquidityDeltaForCompounding(sqrtPrice, liquidityDelta)
	}
	return amountBFromLiquidityDeltaConcentrated(sqrtMinPrice, sqrtPrice, liquidityDelta, rounding)
}
