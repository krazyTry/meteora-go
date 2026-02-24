package dynamic_bonding_curve

import (
	"fmt"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func TestBuildCurve(t *testing.T) {
	migratedPoolBaseFeeMode := shared.DammV2BaseFeeModeFeeTimeSchedulerLinear

	buildCurveBaseParams := shared.BuildCurveBaseParams{
		TotalTokenSupply:  1000000000,
		MigrationOption:   shared.MigrationOptionMetDammV2,
		TokenBaseDecimal:  shared.TokenDecimalSix,
		TokenQuoteDecimal: shared.TokenDecimalNine,
		LockedVestingParams: shared.LockedVestingParams{
			TotalLockedVestingAmount:       0,
			NumberOfVestingPeriod:          0,
			CliffUnlockAmount:              0,
			TotalVestingDuration:           0,
			CliffDurationFromMigrationTime: 0,
		},
		BaseFeeParams: shared.BaseFeeParams{
			BaseFeeMode: shared.BaseFeeModeFeeSchedulerLinear,
			FeeSchedulerParam: &shared.FeeSchedulerParams{
				StartingFeeBps: 100,
				EndingFeeBps:   100,
				NumberOfPeriod: 0,
				TotalDuration:  0,
			},
		},
		DynamicFeeEnabled:                         true,
		ActivationType:                            shared.ActivationTypeSlot,
		CollectFeeMode:                            shared.CollectFeeModeQuoteToken,
		MigrationFeeOption:                        shared.MigrationFeeOptionFixedBps100,
		TokenType:                                 shared.TokenTypeSPL,
		PartnerLiquidityPercentage:                0,
		CreatorLiquidityPercentage:                0,
		PartnerPermanentLockedLiquidityPercentage: 100,
		CreatorPermanentLockedLiquidityPercentage: 0,
		CreatorTradingFeePercentage:               0,
		Leftover:                                  0,
		TokenUpdateAuthority:                      0,
		MigrationFee: struct {
			FeePercentage        uint8
			CreatorFeePercentage uint8
		}{
			FeePercentage:        0,
			CreatorFeePercentage: 0,
		},
		PoolCreationFee:           1,
		MigratedPoolBaseFeeMode:   &migratedPoolBaseFeeMode,
		EnableFirstSwapWithMinFee: false,
	}
	params := shared.BuildCurveParams{
		BuildCurveBaseParams:        buildCurveBaseParams,
		PercentageSupplyOnMigration: 2.983257229832572,
		MigrationQuoteThreshold:     95.07640791476408,
	}

	cfg, err := helpers.BuildCurve(params)
	if err != nil {
		t.Fatal("BuildCurve() fail", err)
	}
	fmt.Println(jsoniter.MarshalToString(cfg))
}
