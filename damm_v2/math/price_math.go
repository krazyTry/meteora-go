package math

import (
	"errors"
	"math/big"

	"github.com/shopspring/decimal"
)

func CalculateInitSqrtPrice(tokenAAmount, tokenBAmount, minSqrtPrice, maxSqrtPrice *big.Int) (*big.Int, error) {
	if tokenAAmount.Sign() == 0 || tokenBAmount.Sign() == 0 {
		return nil, errors.New("amount cannot be zero")
	}
	amountA := decimal.NewFromBigInt(tokenAAmount, 0)
	amountB := decimal.NewFromBigInt(tokenBAmount, 0)
	minSqrt := decimal.NewFromBigInt(minSqrtPrice, 0).Div(decimal.NewFromBigInt(
		new(big.Int).Lsh(big.NewInt(1), 64),
		0,
	))
	maxSqrt := decimal.NewFromBigInt(maxSqrtPrice, 0).Div(decimal.NewFromBigInt(
		new(big.Int).Lsh(big.NewInt(1), 64),
		0,
	))

	x := decimal.NewFromInt(1).Div(maxSqrt)
	y := amountB.Div(amountA)
	xy := x.Mul(y)

	paMinusXY := minSqrt.Sub(xy)
	xyMinusPa := xy.Sub(minSqrt)
	fourY := decimal.NewFromInt(4).Mul(y)
	discriminant := xyMinusPa.Mul(xyMinusPa).Add(fourY)
	discFloat, _ := new(big.Float).SetPrec(256).SetString(discriminant.String())
	if discFloat == nil {
		return nil, errors.New("invalid discriminant")
	}
	sqrtDiscFloat := new(big.Float).SetPrec(256).Sqrt(discFloat)
	sqrtDiscStr := sqrtDiscFloat.Text('f', 40)
	sqrtDisc, err := decimal.NewFromString(sqrtDiscStr)
	if err != nil {
		return nil, err
	}
	result := paMinusXY.Add(sqrtDisc).Div(decimal.NewFromInt(2)).Mul(decimal.NewFromBigInt(
		new(big.Int).Lsh(big.NewInt(1), 64),
		0,
	))
	return result.Floor().BigInt(), nil
}
