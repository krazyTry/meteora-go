package dynamic_bonding_curve

import (
	"fmt"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

func TestBuildCurveWithMarketCap(t *testing.T) {
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
		Leftover:                                  10000,
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

	params := shared.BuildCurveWithMarketCapParams{
		BuildCurveBaseParams: buildCurveBaseParams,
		InitialMarketCap:     23.5,
		MigrationMarketCap:   405.882352941,
	}

	cfg, err := helpers.BuildCurveWithMarketCap(params)
	if err != nil {
		t.Fatal("BuildCurveWithMarketCap() fail", err)
	}
	fmt.Println(jsoniter.MarshalToString(cfg))
}
