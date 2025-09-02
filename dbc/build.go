package dbc

import (
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
)

var (
	BuildCurve                     = dbc.BuildCurve
	BuildCurveWithMarketCap        = dbc.BuildCurveWithMarketCap
	BuildCurveWithTwoSegments      = dbc.BuildCurveWithTwoSegments
	BuildCurveWithLiquidityWeights = dbc.BuildCurveWithLiquidityWeights
)
