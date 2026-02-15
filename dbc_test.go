package meteora

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	jsoniter "github.com/json-iterator/go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/shopspring/decimal"
)

func TestDBC(t *testing.T) {
	return

	stateService := dynamic_bonding_curve.NewStateService(rpcClient, rpc.CommitmentFinalized)
	configAddress := solana.MustPublicKeyFromBase58("")
	configState, err := stateService.GetPoolConfig(context.Background(), configAddress)
	if err != nil {
		t.Fatal("GetConfig() fail", err)
	}

	poolService := dynamic_bonding_curve.NewPoolService(rpcClient, rpc.CommitmentFinalized)

	name := "MeteoraGoTest"
	symbol := "METAGOTEST"
	uri := "https://launch.meteora.ag/icons/logo.svg"

	// creator := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	// fmt.Printf("poolPartner4 address:%s(%s)\n", poolPartner4.PublicKey(), poolPartner4.PrivateKey)
	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	// payer := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	// fmt.Println("payer address:", payer.PublicKey())

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

	{
		mintWallet := solana.NewWallet()
		baseMint := mintWallet.PublicKey()

		fmt.Println("try to create token mint address:", baseMint, mintWallet)
		// ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
		// defer cancel1()
		ctx1 := context.Background()

		createParams := dynamic_bonding_curve.CreatePoolParams{
			Name:        name,
			Symbol:      symbol,
			URI:         uri,
			Payer:       owner,
			PoolCreator: owner,
			Config:      configAddress,
			BaseMint:    baseMint,
		}

		createIx, err := poolService.CreatePool(ctx1, createParams)
		if err != nil {
			t.Fatal("dbc.CreatePool() fail", err)
		}

		instructions := []solana.Instruction{createIx}
		sig, err := SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(mintWallet.PublicKey()):
				return &mintWallet.PrivateKey
			case key.Equals(ownerWallet.PublicKey()):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("create SendTransaction() fail", err)
		}
		fmt.Println("创建 token success Success sig:", sig.String())

		poolState, err := stateService.GetPoolByBaseMint(ctx1, baseMint)
		if err != nil {
			t.Fatal("GetPoolByPoolAddress() fail", err)
		}
		fmt.Println("创建 token success Pool:", poolState)

		swapResult, err := poolService.SwapQuote(dynamic_bonding_curve.SwapQuoteParams{
			VirtualPool:      poolState.Account,
			Config:           configState,
			SwapBaseForQuote: false,
			AmountIn:         big.NewInt(0.1 * 1e9),
			SlippageBps:      10000,
			// HasReferral                    bool
			// EligibleForFirstSwapWithMinFee bool
			CurrentPoint: dynamic_bonding_curve.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dynamic_bonding_curve.ActivationType(configState.ActivationType)),
		})
		if err != nil {
			t.Fatal("SwapQuote() fail", err)
		}

		swapParams := dynamic_bonding_curve.SwapParams{
			Owner:            owner,
			Pool:             poolState.Pubkey,
			AmountIn:         big.NewInt(0.1 * 1e9),
			MinimumAmountOut: swapResult.MinimumAmountOut,
			SwapBaseForQuote: false,
			// ReferralTokenAccount *solanago.PublicKey
			// Payer                *solanago.PublicKey
		}
		pre, swapIx, post, err := poolService.Swap(ctx1, swapParams)
		if err != nil {
			t.Fatal("Swap() fail", err)
		}

		instructions = []solana.Instruction{}
		instructions = append(pre, swapIx)
		instructions = append(instructions, post...)

		sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(ownerWallet.PublicKey()):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("swap SendTransaction() fail", err)
		}
		fmt.Println("swap token success Success sig:", sig.String())

		swapResult2, err := poolService.SwapQuote2(dynamic_bonding_curve.SwapQuote2Params{
			VirtualPool:      poolState.Account,
			Config:           configState,
			SwapBaseForQuote: false,
			// HasReferral                    bool
			// EligibleForFirstSwapWithMinFee bool
			CurrentPoint: dynamic_bonding_curve.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dynamic_bonding_curve.ActivationType(configState.ActivationType)),
			SlippageBps:  10000,
			SwapMode:     dynamic_bonding_curve.SwapModeExactIn,
			AmountIn:     big.NewInt(0.1 * 1e9),
			// AmountOut                      *big.Int
		})
		if err != nil {
			t.Fatal("SwapQuote2() fail", err)
		}

		swap2Params := dynamic_bonding_curve.Swap2Params{
			Owner:            owner,
			Pool:             poolState.Pubkey,
			SwapBaseForQuote: false,
			// ReferralTokenAccount *solanago.PublicKey
			// Payer                *solanago.PublicKey
			SwapMode:         dynamic_bonding_curve.SwapModeExactIn,
			AmountIn:         big.NewInt(0.1 * 1e9),
			MinimumAmountOut: swapResult2.MinimumAmountOut,
			// AmountOut            *big.Int
			// MaximumAmountIn      *big.Int
		}

		pre2, swapIx2, post2, err := poolService.Swap2(ctx1, swap2Params)
		if err != nil {
			t.Fatal("Swap2() fail", err)
		}

		instructions = []solana.Instruction{}
		instructions = append(pre2, swapIx2)
		instructions = append(instructions, post2...)

		sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(ownerWallet.PublicKey()):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})

		if err != nil {
			t.Fatal("swap2 SendTransaction() fail", err)
		}
		fmt.Println("swap2 token success Success sig:", sig.String())

		partnerService := dynamic_bonding_curve.NewPartnerService(rpcClient, rpc.CommitmentFinalized)

		migrationService := dynamic_bonding_curve.NewMigrationService(rpcClient, rpc.CommitmentFinalized)

		poolState, err = stateService.GetPoolByBaseMint(ctx1, baseMint)
		if err != nil {
			t.Fatal("GetPoolByPoolAddress() fail", err)
		}
		fmt.Println("token info Pool:", poolState)

		if poolState.Account.PartnerQuoteFee > 0 {
			claimParams := dynamic_bonding_curve.ClaimTradingFeeParams{
				Pool:       poolState.Pubkey,
				FeeClaimer: partner.PublicKey(),
				Payer:      partner.PublicKey(),
				// MaxBaseAmount  *big.Int
				MaxQuoteAmount: new(big.Int).SetUint64(poolState.Account.PartnerQuoteFee),
				// Receiver       *solanago.PublicKey
				// TempWSolAcc    *solanago.PublicKey
			}
			pre, claimIx, post, err := partnerService.ClaimPartnerTradingFee(ctx1, claimParams)
			if err != nil {
				t.Fatal("ClaimPartnerTradingFee() fail", err)
			}
			instructions = []solana.Instruction{}
			instructions = append(pre, claimIx)
			instructions = append(instructions, post...)

			sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				case key.Equals(partner.PublicKey()):
					return &partner.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("claim SendTransaction() fail", err)
			}

			fmt.Println("claim token success Success sig:", sig.String())
		}

		migrationQuote := decimal.NewFromUint64(configState.MigrationQuoteThreshold)
		QuoteReserve := decimal.NewFromUint64(poolState.Account.QuoteReserve)
		cliffFeeNumerator := configState.PoolFees.BaseFee.CliffFeeNumerator
		baseFeeBps := decimal.NewFromUint64(cliffFeeNumerator).Div(decimal.NewFromInt(1e5))
		quoteFeeBps := decimal.NewFromInt(10000).Sub(baseFeeBps)

		{
			availableQuote := migrationQuote.Sub(QuoteReserve).Div(quoteFeeBps.Div(decimal.NewFromInt(10000))).Ceil().BigInt().Uint64()

			swapParams := dynamic_bonding_curve.SwapParams{
				Owner:            owner,
				Pool:             poolState.Pubkey,
				AmountIn:         new(big.Int).SetUint64(availableQuote),
				MinimumAmountOut: new(big.Int).SetUint64(1),
				SwapBaseForQuote: false,
				// ReferralTokenAccount *solanago.PublicKey
				// Payer                *solanago.PublicKey
			}
			pre, swapIx, post, err := poolService.Swap(ctx1, swapParams)
			if err != nil {
				t.Fatal("Swap() fail", err)
			}

			instructions = []solana.Instruction{}
			instructions = append(pre, swapIx)
			instructions = append(instructions, post...)

			sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("swap SendTransaction() fail", err)
			}
			fmt.Println("swap full token success Success sig:", sig.String())
		}

		poolState, err = stateService.GetPoolByBaseMint(ctx1, baseMint)
		if err != nil {
			t.Fatal("GetPoolByPoolAddress() fail", err)
		}
		fmt.Println("token QuoteReserve:", poolState.Account.QuoteReserve)
		fmt.Println("token MigrationQuoteThreshold:", configState.MigrationQuoteThreshold)

		if poolState.Account.QuoteReserve == configState.MigrationQuoteThreshold {

			params := dynamic_bonding_curve.CreateDammV2MigrationMetadataParams{
				VirtualPool: poolState.Pubkey,
				Payer:       ownerWallet.PublicKey(),
				Config:      configAddress,
			}
			ix, err := migrationService.CreateDammV2MigrationMetadata(ctx1, params)
			if err != nil {
				t.Fatal("CreateDammV2MigrationMetadata() fail", err)
			}
			instructions = []solana.Instruction{ix}
			sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("swap SendTransaction() fail", err)
			}
			fmt.Println("create migration metadata success Success sig:", sig.String())

			if dynamic_bonding_curve.MigrationProgress(poolState.Account.MigrationProgress) < dynamic_bonding_curve.MigrationProgressPostBondingCurve {
				lockerParams := dynamic_bonding_curve.CreateLockerParams{
					VirtualPool: poolState.Pubkey,
					Payer:       ownerWallet.PublicKey(),
				}
				pre, lockerIx, post, err := migrationService.CreateLocker(ctx1, lockerParams)
				if err != nil {
					t.Fatal("CreateLocker() fail", err)
				}

				if len(pre) > 0 {
					// 没上锁过
					instructions = []solana.Instruction{}
					instructions = append(pre, lockerIx)
					instructions = append(instructions, post...)

					sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
						switch {
						case key.Equals(ownerWallet.PublicKey()):
							return &ownerWallet.PrivateKey
						default:
							return nil
						}
					})
					if err != nil {
						t.Fatal("swap SendTransaction() fail", err)
					}
					fmt.Println("create locker success Success sig:", sig.String())
				}
			}

			resp, err := migrationService.MigrateToDammV2(ctx1, dynamic_bonding_curve.MigrateToDammV2Params{
				VirtualPool: poolState.Pubkey,
				Payer:       ownerWallet.PublicKey(),
				DammConfig:  dynamic_bonding_curve.GetDammV2Config(dynamic_bonding_curve.MigrationFeeOption(configState.MigrationFeeOption)),
			})
			if err != nil {
				t.Fatal("MigrateToDammV2() fail", err)
			}

			sig, err = SendTransaction(ctx1, rpcClient, wsClient, resp.Transaction, func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				case key.Equals(resp.FirstPositionNFT.PublicKey()):
					return &resp.FirstPositionNFT
				case key.Equals(resp.SecondPositionNFT.PublicKey()):
					return &resp.SecondPositionNFT
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("MigrateToDammV2 SendTransaction() fail", err)
			}
			fmt.Println("migrate to damm v2 success Success sig:", sig.String())
		}

		if poolState.Account.IsWithdrawLeftover == 0 {
			leftoverParams := dynamic_bonding_curve.WithdrawLeftoverParams{
				VirtualPool: poolState.Pubkey,
				Payer:       leftover.PublicKey(),
			}
			pre, withdrawIx, post, err := migrationService.WithdrawLeftover(ctx1, leftoverParams)
			if err != nil {
				t.Fatal("WithdrawLeftover() fail", err)
			}

			instructions = []solana.Instruction{}
			instructions = append(pre, withdrawIx)
			instructions = append(instructions, post...)

			sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				case key.Equals(leftover.PublicKey()):
					return &leftover.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("withdraw SendInstruction() fail", err)
			}

			fmt.Println("withdraw token success Success sig:", sig.String())
		}

		poolState, err = stateService.GetPoolByBaseMint(ctx1, baseMint)
		if err != nil {
			t.Fatal("GetPoolByPoolAddress() fail", err)
		}
		fmt.Println("token info Pool:", poolState)

		if poolState.Account.PartnerQuoteFee > 0 {
			claimParams := dynamic_bonding_curve.ClaimTradingFeeParams{
				Pool:       poolState.Pubkey,
				FeeClaimer: partner.PublicKey(),
				Payer:      partner.PublicKey(),
				// MaxBaseAmount  *big.Int
				MaxQuoteAmount: new(big.Int).SetUint64(poolState.Account.PartnerQuoteFee),
				// Receiver       *solanago.PublicKey
				// TempWSolAcc    *solanago.PublicKey
			}
			pre, claimIx, post, err := partnerService.ClaimPartnerTradingFee(ctx1, claimParams)
			if err != nil {
				t.Fatal("ClaimPartnerTradingFee() fail", err)
			}
			instructions = []solana.Instruction{}
			instructions = append(pre, claimIx)
			instructions = append(instructions, post...)

			sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				case key.Equals(partner.PublicKey()):
					return &partner.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("claim SendTransaction() fail", err)
			}

			fmt.Println("claim token success Success sig:", sig.String())
		}

	}

}

func TestBuildCurve(t *testing.T) {
	return
	migratedPoolBaseFeeMode := helpers.DammV2BaseFeeModeFeeTimeSchedulerLinear

	buildCurveBaseParams := helpers.BuildCurveBaseParams{
		TotalTokenSupply:  1000000000,
		MigrationOption:   helpers.MigrationOptionMetDammV2,
		TokenBaseDecimal:  helpers.TokenDecimalSix,
		TokenQuoteDecimal: helpers.TokenDecimalNine,
		LockedVestingParams: helpers.LockedVestingParams{
			TotalLockedVestingAmount:       0,
			NumberOfVestingPeriod:          0,
			CliffUnlockAmount:              0,
			TotalVestingDuration:           0,
			CliffDurationFromMigrationTime: 0,
		},
		BaseFeeParams: helpers.BaseFeeParams{
			BaseFeeMode: helpers.BaseFeeModeFeeSchedulerLinear,
			FeeSchedulerParam: &helpers.FeeSchedulerParams{
				StartingFeeBps: 100,
				EndingFeeBps:   100,
				NumberOfPeriod: 0,
				TotalDuration:  0,
			},
		},
		DynamicFeeEnabled:                         true,
		ActivationType:                            helpers.ActivationTypeSlot,
		CollectFeeMode:                            helpers.CollectFeeModeQuoteToken,
		MigrationFeeOption:                        helpers.MigrationFeeOptionFixedBps100,
		TokenType:                                 helpers.TokenTypeSPL,
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
	params := helpers.BuildCurveParams{
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

func TestBuildCurveWithCustomSqrtPrices(t *testing.T) {
	return
	migratedPoolBaseFeeMode := helpers.DammV2BaseFeeModeFeeTimeSchedulerLinear

	buildCurveBaseParams := helpers.BuildCurveBaseParams{
		TotalTokenSupply:  1000000000,
		MigrationOption:   helpers.MigrationOptionMetDammV2,
		TokenBaseDecimal:  helpers.TokenDecimalSix,
		TokenQuoteDecimal: helpers.TokenDecimalNine,
		LockedVestingParams: helpers.LockedVestingParams{
			TotalLockedVestingAmount:       0,
			NumberOfVestingPeriod:          0,
			CliffUnlockAmount:              0,
			TotalVestingDuration:           0,
			CliffDurationFromMigrationTime: 0,
		},
		BaseFeeParams: helpers.BaseFeeParams{
			BaseFeeMode: helpers.BaseFeeModeFeeSchedulerLinear,
			FeeSchedulerParam: &helpers.FeeSchedulerParams{
				StartingFeeBps: 100,
				EndingFeeBps:   100,
				NumberOfPeriod: 0,
				TotalDuration:  0,
			},
		},
		DynamicFeeEnabled:                         true,
		ActivationType:                            helpers.ActivationTypeSlot,
		CollectFeeMode:                            helpers.CollectFeeModeQuoteToken,
		MigrationFeeOption:                        helpers.MigrationFeeOptionFixedBps100,
		TokenType:                                 helpers.TokenTypeSPL,
		PartnerLiquidityPercentage:                0,
		CreatorLiquidityPercentage:                0,
		PartnerPermanentLockedLiquidityPercentage: 100,
		CreatorPermanentLockedLiquidityPercentage: 0,
		CreatorTradingFeePercentage:               0,
		Leftover:                                  1000,
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

	// prices := []string{"0.001", "0.005", "0.01"}
	prices := []string{"0.0001", "0.0005", "0.001", "0.002", "0.004", "0.006", "0.008", "0.01"}

	sqrtPrices, _ := helpers.CreateSqrtPrices(
		prices,
		helpers.TokenDecimalNine,
		helpers.TokenDecimalNine,
	)

	fmt.Println(sqrtPrices)

	params := helpers.BuildCurveWithCustomSqrtPricesParams{
		BuildCurveBaseParams: buildCurveBaseParams,
		SqrtPrices:           sqrtPrices,
	}

	cfg, err := helpers.BuildCurveWithCustomSqrtPrices(params)
	if err != nil {
		t.Fatal("BuildCurveWithCustomSqrtPrices() fail", err)
	}
	fmt.Println(jsoniter.MarshalToString(cfg))
}

func TestBuildCurveWithLiquidityWeights(t *testing.T) {
	return
	migratedPoolBaseFeeMode := helpers.DammV2BaseFeeModeFeeTimeSchedulerLinear

	buildCurveBaseParams := helpers.BuildCurveBaseParams{
		TotalTokenSupply:  1000000000,
		MigrationOption:   helpers.MigrationOptionMetDammV2,
		TokenBaseDecimal:  helpers.TokenDecimalSix,
		TokenQuoteDecimal: helpers.TokenDecimalNine,
		LockedVestingParams: helpers.LockedVestingParams{
			TotalLockedVestingAmount:       0,
			NumberOfVestingPeriod:          0,
			CliffUnlockAmount:              0,
			TotalVestingDuration:           0,
			CliffDurationFromMigrationTime: 0,
		},
		BaseFeeParams: helpers.BaseFeeParams{
			BaseFeeMode: helpers.BaseFeeModeFeeSchedulerLinear,
			FeeSchedulerParam: &helpers.FeeSchedulerParams{
				StartingFeeBps: 100,
				EndingFeeBps:   100,
				NumberOfPeriod: 0,
				TotalDuration:  0,
			},
		},
		DynamicFeeEnabled:                         true,
		ActivationType:                            helpers.ActivationTypeSlot,
		CollectFeeMode:                            helpers.CollectFeeModeQuoteToken,
		MigrationFeeOption:                        helpers.MigrationFeeOptionFixedBps100,
		TokenType:                                 helpers.TokenTypeSPL,
		PartnerLiquidityPercentage:                0,
		CreatorLiquidityPercentage:                0,
		PartnerPermanentLockedLiquidityPercentage: 100,
		CreatorPermanentLockedLiquidityPercentage: 0,
		CreatorTradingFeePercentage:               0,
		Leftover:                                  1000,
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

	liquidityWeights := make([]float64, 16)

	for i := 0; i < 16; i++ {
		liquidityWeights[i] = decimal.NewFromFloat(1.2).Pow(decimal.NewFromInt(int64(i))).InexactFloat64()
	}

	fmt.Println(liquidityWeights)

	params := helpers.BuildCurveWithLiquidityWeightsParams{
		BuildCurveBaseParams: buildCurveBaseParams,
		InitialMarketCap:     30,
		MigrationMarketCap:   300,
		LiquidityWeights:     liquidityWeights,
	}

	cfg, err := helpers.BuildCurveWithLiquidityWeights(params)
	if err != nil {
		t.Fatal("BuildCurveWithCustomSqrtPrices() fail", err)
	}
	fmt.Println(jsoniter.MarshalToString(cfg))
}

func TestBuildCurveWithMarketCap(t *testing.T) {
	return
	migratedPoolBaseFeeMode := helpers.DammV2BaseFeeModeFeeTimeSchedulerLinear

	buildCurveBaseParams := helpers.BuildCurveBaseParams{
		TotalTokenSupply:  1000000000,
		MigrationOption:   helpers.MigrationOptionMetDammV2,
		TokenBaseDecimal:  helpers.TokenDecimalSix,
		TokenQuoteDecimal: helpers.TokenDecimalNine,
		LockedVestingParams: helpers.LockedVestingParams{
			TotalLockedVestingAmount:       0,
			NumberOfVestingPeriod:          0,
			CliffUnlockAmount:              0,
			TotalVestingDuration:           0,
			CliffDurationFromMigrationTime: 0,
		},
		BaseFeeParams: helpers.BaseFeeParams{
			BaseFeeMode: helpers.BaseFeeModeFeeSchedulerLinear,
			FeeSchedulerParam: &helpers.FeeSchedulerParams{
				StartingFeeBps: 100,
				EndingFeeBps:   100,
				NumberOfPeriod: 0,
				TotalDuration:  0,
			},
		},
		DynamicFeeEnabled:                         true,
		ActivationType:                            helpers.ActivationTypeSlot,
		CollectFeeMode:                            helpers.CollectFeeModeQuoteToken,
		MigrationFeeOption:                        helpers.MigrationFeeOptionFixedBps100,
		TokenType:                                 helpers.TokenTypeSPL,
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

	params := helpers.BuildCurveWithMarketCapParams{
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

func TestBuildCurveWithTwoSegments(t *testing.T) {
	return
	migratedPoolBaseFeeMode := helpers.DammV2BaseFeeModeFeeTimeSchedulerLinear

	buildCurveBaseParams := helpers.BuildCurveBaseParams{
		TotalTokenSupply:  1000000000,
		MigrationOption:   helpers.MigrationOptionMetDammV2,
		TokenBaseDecimal:  helpers.TokenDecimalSix,
		TokenQuoteDecimal: helpers.TokenDecimalNine,
		LockedVestingParams: helpers.LockedVestingParams{
			TotalLockedVestingAmount:       0,
			NumberOfVestingPeriod:          0,
			CliffUnlockAmount:              0,
			TotalVestingDuration:           0,
			CliffDurationFromMigrationTime: 0,
		},
		BaseFeeParams: helpers.BaseFeeParams{
			BaseFeeMode: helpers.BaseFeeModeFeeSchedulerLinear,
			FeeSchedulerParam: &helpers.FeeSchedulerParams{
				StartingFeeBps: 100,
				EndingFeeBps:   100,
				NumberOfPeriod: 0,
				TotalDuration:  0,
			},
		},
		DynamicFeeEnabled:                         true,
		ActivationType:                            helpers.ActivationTypeSlot,
		CollectFeeMode:                            helpers.CollectFeeModeQuoteToken,
		MigrationFeeOption:                        helpers.MigrationFeeOptionFixedBps100,
		TokenType:                                 helpers.TokenTypeSPL,
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

	params := helpers.BuildCurveWithTwoSegmentsParams{
		BuildCurveBaseParams:        buildCurveBaseParams,
		InitialMarketCap:            20000,
		MigrationMarketCap:          1000000,
		PercentageSupplyOnMigration: 20,
	}

	cfg, err := helpers.BuildCurveWithTwoSegments(params)
	if err != nil {
		t.Fatal("BuildCurveWithTwoSegments() fail", err)
	}
	fmt.Println(jsoniter.MarshalToString(cfg))
}

func TestWallet(t *testing.T) {
	wallet := solana.NewWallet()
	fmt.Println(wallet.PublicKey(), wallet.PrivateKey)

	wallet1 := solana.NewWallet()
	fmt.Println(wallet1.PublicKey(), wallet1.PrivateKey)

	wallet2 := solana.NewWallet()
	fmt.Println(wallet2.PublicKey(), wallet2.PrivateKey)
}
