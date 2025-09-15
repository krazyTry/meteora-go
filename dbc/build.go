package dbc

import (
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
)

// BuildCurve provides a simple way to generate the parameters required for CreateConfig.
// It builds a new constant product curve.
// This function does the math for you to create a curve structure based on the percentage of supply on migration and migration quote threshold.
//
// Example:
//
//	configParameters,_ := meteoraDBC.BuildCurve(dynamic_bonding_curve.BuildCurveParam{
//		BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
//			TotalTokenSupply:  1000000000,
//			MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
//			TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalSix,
//			TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
//			LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
//				TotalLockedVestingAmount:       0,
//				NumberOfVestingPeriod:          0,
//				CliffUnlockAmount:              0,
//				TotalVestingDuration:           0,
//				CliffDurationFromMigrationTime: 0,
//			},
//			BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
//				BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerExponential,
//				FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
//					StartingFeeBps: 100,
//					EndingFeeBps:   100,
//					NumberOfPeriod: 0,
//					TotalDuration:  0,
//				},
//			},
//			DynamicFeeEnabled:           true,
//			ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
//			CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
//			MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
//			TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
//			PartnerLpPercentage:         0,
//			CreatorLpPercentage:         0,
//			PartnerLockedLpPercentage:   100,
//			CreatorLockedLpPercentage:   0,
//			CreatorTradingFeePercentage: 0,
//			Leftover:                    10000,
//			TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityCreatorUpdateAuthority,
//			MigrationFee: dynamic_bonding_curve.MigrationFee{
//				FeePercentage:        0,
//				CreatorFeePercentage: 0,
//			},
//		},
//		PercentageSupplyOnMigration: 2.983257229832572,
//		MigrationQuoteThreshold:     95.07640791476408,
//	})
//
//	meteoraDBC.CreateConfig(ctx,wsClient,payer,quoteMint,configParameters)
var BuildCurve = dbc.BuildCurve

// BuildCurveWithMarketCap provides a simple way to generate the parameters required for CreateConfig.
// It builds a new constant product curve with customizable parameters based on market cap.
// This function does the math for you to create a curve structure based on initial market cap and migration market cap.
//
// Example:
//
//	configParameters,_ := meteoraDBC.BuildCurveWithMarketCap(dynamic_bonding_curve.BuildCurveWithMarketCapParam{
//			BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
//				TotalTokenSupply:  1000000000,
//				MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
//				TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalSix,
//				TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
//				LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
//					TotalLockedVestingAmount:       0,
//					NumberOfVestingPeriod:          0,
//					CliffUnlockAmount:              0,
//					TotalVestingDuration:           0,
//					CliffDurationFromMigrationTime: 0,
//				},
//				BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
//					BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerLinear,
//					FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
//						StartingFeeBps: 100,
//						EndingFeeBps:   100,
//						NumberOfPeriod: 0,
//						TotalDuration:  0,
//					},
//				},
//				DynamicFeeEnabled:           true,
//				ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
//				CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
//				MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
//				TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
//				PartnerLpPercentage:         0,
//				CreatorLpPercentage:         0,
//				PartnerLockedLpPercentage:   100,
//				CreatorLockedLpPercentage:   0,
//				CreatorTradingFeePercentage: 0,
//				Leftover:                    10000,
//				TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityImmutable,
//				MigrationFee: dynamic_bonding_curve.MigrationFee{
//					FeePercentage:        10,
//					CreatorFeePercentage: 50,
//				},
//			},
//			InitialMarketCap:   23.5,
//			MigrationMarketCap: 405.882352941,
//	})
//
//	meteoraDBC.CreateConfig(ctx,wsClient,payer,quoteMint,configParameters)
var BuildCurveWithMarketCap = dbc.BuildCurveWithMarketCap

// BuildCurveWithTwoSegments provides a simple way to generate the parameters required for CreateConfig.
// It builds a new constant product curve with two segments.
// This function does the math for you to create a curve structure based on initial market cap, migration market cap, and percentage of supply on migration.
//
// Example:
//
//	configParameters,_ := meteoraDBC.BuildCurveWithTwoSegments(dynamic_bonding_curve.BuildCurveWithTwoSegmentsParam{
//			BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
//				TotalTokenSupply:  1000000000,
//				MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
//				TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalNine,
//				TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
//				LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
//					TotalLockedVestingAmount:       0,
//					NumberOfVestingPeriod:          0,
//					CliffUnlockAmount:              0,
//					TotalVestingDuration:           0,
//					CliffDurationFromMigrationTime: 0,
//				},
//				BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
//					BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerExponential,
//					FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
//						StartingFeeBps: 5000,
//						EndingFeeBps:   100,
//						NumberOfPeriod: 120,
//						TotalDuration:  120,
//					},
//				},
//				DynamicFeeEnabled:           true,
//				ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
//				CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
//				MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
//				TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
//				PartnerLpPercentage:         0,
//				CreatorLpPercentage:         0,
//				PartnerLockedLpPercentage:   100,
//				CreatorLockedLpPercentage:   0,
//				CreatorTradingFeePercentage: 0,
//				Leftover:                    350000000,
//				TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityCreatorUpdateAuthority,
//				MigrationFee: dynamic_bonding_curve.MigrationFee{
//					FeePercentage:        10,
//					CreatorFeePercentage: 50,
//				},
//			},
//			InitialMarketCap:            20000,
//			MigrationMarketCap:          1000000,
//			PercentageSupplyOnMigration: 20,
//	})
//
//	meteoraDBC.CreateConfig(ctx,wsClient,payer,quoteMint,configParameters)
var BuildCurveWithTwoSegments = dbc.BuildCurveWithTwoSegments

// BuildCurveWithLiquidityWeights provides a simple way to generate the parameters required for CreateConfig.
// It builds a super customizable constant product curve graph configuration based on different liquidity weights.
// This function does the math for you to create a curve structure based on initial market cap, migration market cap, and liquidity weights.
//
// Example:
//
// liquidityWeights := make([]float64, 16)
// base := decimal.NewFromFloat(1.2)
//
//	for i := 0; i < 16; i++ {
//		liquidityWeights[i] = base.Pow(decimal.NewFromInt(int64(i))).InexactFloat64()
//	}
//
//	configParameters,_ := dbc.BuildCurveWithLiquidityWeights(dynamic_bonding_curve.BuildCurveWithLiquidityWeightsParam{
//			BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
//				TotalTokenSupply:  1000000000,
//				MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
//				TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalSix,
//				TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
//				LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
//					TotalLockedVestingAmount:       0,
//					NumberOfVestingPeriod:          0,
//					CliffUnlockAmount:              0,
//					TotalVestingDuration:           0,
//					CliffDurationFromMigrationTime: 0,
//				},
//				BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
//					BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerLinear,
//					FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
//						StartingFeeBps: 100,
//						EndingFeeBps:   100,
//						NumberOfPeriod: 0,
//						TotalDuration:  0,
//					},
//				},
//				DynamicFeeEnabled:           true,
//				ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
//				CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
//				MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
//				TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
//				PartnerLpPercentage:         0,
//				CreatorLpPercentage:         0,
//				PartnerLockedLpPercentage:   100,
//				CreatorLockedLpPercentage:   0,
//				CreatorTradingFeePercentage: 0,
//				Leftover:                    10000,
//				TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityImmutable,
//				MigrationFee: dynamic_bonding_curve.MigrationFee{
//					FeePercentage:        10,
//					CreatorFeePercentage: 50,
//				},
//			},
//			InitialMarketCap:   30,
//			MigrationMarketCap: 300,
//			LiquidityWeights:   liquidityWeights,
//	})
//
//	dbc.CreateConfig(ctx,wsClient,payer,quoteMint,configParameters)
var BuildCurveWithLiquidityWeights = dbc.BuildCurveWithLiquidityWeights
