package dbc

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// CreateConfigInstruction generates the instruction needed for creating configuration.
//
// Example:
//
// instructions, _ := CreateConfigInstruction(
//
//	ctx,
//	payer.PublicKey(),
//	m.config.PublicKey(), // config address
//	m.feeClaimer.PublicKey(), // partner
//	m.leftoverReceiver.PublicKey(),// leftover receiver account
//	quoteMint, // quoteMintToken eg: solana.WrappedSol
//	cfg,
//
// )
func CreateConfigInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	config solana.PublicKey,
	partner solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	quoteMint solana.PublicKey,
	configParameters *dbc.ConfigParameters,
) ([]solana.Instruction, error) {

	createConfigIx, err := dbc.NewCreateConfigInstruction(
		configParameters,
		config,
		partner,
		leftoverReceiver,
		quoteMint,
		payer,
		solana.SystemProgramID,
		eventAuthority,
		dbc.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{createConfigIx}, nil
}

// CreateConfig creates a new config key that will dictate the behavior of all pools created with this key.
// This is where you set the pool fees, migration options, the bonding curve shape, and more.
// The function depends on CreateConfigInstruction.
// The function is blocking, it will wait for on-chain confirmation before returning.
//
// Example:
//
// m.CreateConfig(
//
//	ctx,
//	wsClient,
//	payerWallet, // payer account
//	configWallet, // config account
//	quoteMint, // quoteMintToken eg: solana.WrappedSol
//	cfg, // configuration eg: BuildCurve | BuildCurveWithMarketCap | BuildCurveWithTwoSegments | BuildCurveWithLiquidityWeights
//
// )
func (m *DBC) CreateConfig(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	config *solana.Wallet,
	quoteMint solana.PublicKey,
	cfg *dbc.ConfigParameters,
) (string, error) {
	if m.feeClaimer == nil {
		return "", fmt.Errorf("partner is nil")
	}
	if m.leftoverReceiver == nil {
		return "", fmt.Errorf("leftoverReceiver is nil")
	}
	if cfg.CreatorLpPercentage+cfg.CreatorLockedLpPercentage+cfg.PartnerLpPercentage+cfg.PartnerLockedLpPercentage != 100 {
		return "", fmt.Errorf("100 != cfg.CreatorLpPercentage+cfg.CreatorLockedLpPercentage+cfg.PartnerLpPercentage+cfg.PartnerLockedLpPercentage")
	}

	instructions, err := CreateConfigInstruction(
		ctx,
		payer.PublicKey(),
		config.PublicKey(),
		m.feeClaimer.PublicKey(),
		m.leftoverReceiver.PublicKey(),
		quoteMint,
		cfg,
	)
	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(config.PublicKey()):
				return &config.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

// GetConfig gets all details about the config (dbc pool config).
// Depends on GetConfig.
//
// Example:
//
// m.GetConfig(
//
//	ctx,
//	m.config.PublicKey(),
//
// )
func (m *DBC) GetConfig(
	ctx context.Context,
	configAddress solana.PublicKey,
) (*dbc.PoolConfig, error) {
	return GetConfig(ctx, m.rpcClient, configAddress)
}

// GetConfig gets all details about the config (dbc pool config).
//
// Example:
//
// GetConfig(
//
//	ctx,
//	rpcClient,
//	config.PublicKey(),
//
// )
func GetConfig(
	ctx context.Context,
	rpcClient *rpc.Client,
	configAddress solana.PublicKey,
) (*dbc.PoolConfig, error) {
	out, err := solanago.GetAccountInfo(ctx, rpcClient, configAddress)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	obj, err := dbc.ParseAnyAccount(out.GetBinary())
	if err != nil {
		return nil, err
	}

	cfg, ok := obj.(*dbc.PoolConfig)
	if !ok {
		return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
	}

	return cfg, nil
}

// GetConfigs Get all configs.
//
// Example:
//
// configs, _ := GetConfigs(
//
//	ctx,
//	rpcClient,
//
// )
func GetConfigs(
	ctx context.Context,
	rpcClient *rpc.Client,
) ([]*Config, error) {
	opt := solanago.GenProgramAccountFilter(dbc.AccountKeyPoolConfig, nil)

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*Config
	for _, out := range outs {
		obj, err := dbc.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		cfg, ok := obj.(*dbc.PoolConfig)
		if !ok {
			return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
		}
		list = append(list, &Config{cfg, out.Pubkey})
	}

	return list, nil
}

// InitConfig performs initialization check, creates config if it doesn't exist, skips if it exists.
// The function is blocking, it will wait for on-chain confirmation before returning.
//
// Example:
//
//	cfg :=&dynamic_bonding_curve.ConfigParameters{
//		PoolFees: dynamic_bonding_curve.PoolFeeParameters{
//			BaseFee: dynamic_bonding_curve.BaseFeeParameters{
//				CliffFeeNumerator: 5000 * 100_000, // 50% = 5000*0.01%,
//				FirstFactor:       0,
//				SecondFactor:      0,
//				ThirdFactor:       0,
//				BaseFeeMode:       0,
//			},
//			DynamicFee: &dynamic_bonding_curve.DynamicFeeParameters{
//				BinStep:                  1,
//				BinStepU128:              u128.GenUint128FromString("1844674407370955"),
//				FilterPeriod:             10,
//				DecayPeriod:              120,
//				ReductionFactor:          1_000,
//				MaxVolatilityAccumulator: 100_000,
//				VariableFeeControl:       100_000,
//			},
//		},
//		CollectFeeMode:            dynamic_bonding_curve.CollectFeeModeQuoteToken,
//		MigrationOption:           dynamic_bonding_curve.MigrationOptionMETDAMMV2,
//		ActivationType:            dynamic_bonding_curve.ActivationTypeTimestamp,
//		TokenType:                 dynamic_bonding_curve.TokenTypeSPL,
//		TokenDecimal:              dynamic_bonding_curve.TokenDecimalNine,
//		PartnerLpPercentage:       80,
//		PartnerLockedLpPercentage: 0,
//		CreatorLpPercentage:       20,
//		CreatorLockedLpPercentage: 0,
//		MigrationQuoteThreshold:   0.5 * 1e9, // 85 * 1e9, >= 750 USD
//		SqrtStartPrice:            u128.GenUint128FromString("58333726687135158"),
//		LockedVesting: dynamic_bonding_curve.LockedVesting{
//			AmountPerPeriod:                0,
//			CliffDurationFromMigrationTime: 0,
//			Frequency:                      0,
//			NumberOfPeriod:                 0,
//			CliffUnlockAmount:              0,
//		},
//		MigrationFeeOption: dynamic_bonding_curve.MigrationFeeFixedBps200, // 0: Fixed 25bps, 1: Fixed 30bps, 2: Fixed 100bps, 3: Fixed 200bps, 4: Fixed 400bps, 5: Fixed 600bps
//		TokenSupply: &dynamic_bonding_curve.TokenSupplyParams{
//			PreMigrationTokenSupply:  1000000000000000000,
//			PostMigrationTokenSupply: 1000000000000000000,
//		},
//		CreatorTradingFeePercentage: 0,
//		TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityImmutable,
//		MigrationFee: dynamic_bonding_curve.MigrationFee{
//			FeePercentage:        2,
//			CreatorFeePercentage: 0,
//		},
//		// MigratedPoolFee: &dbc.MigratedPoolFee{},
//		Padding: [7]uint64{},
//		// use case
//		Curve: []dynamic_bonding_curve.LiquidityDistributionParameters{
//			{
//				SqrtPrice: u128.GenUint128FromString("233334906748540631"),
//				Liquidity: u128.GenUint128FromString("622226417996106429201027821619672729"),
//			},
//			{
//				SqrtPrice: u128.GenUint128FromString("79226673521066979257578248091"),
//				Liquidity: u128.GenUint128FromString("1"),
//			},
//		},
//	}
//
// meteoraDBC.InitConfig(
//
//	ctx,
//	wsClient,
//	payer,
//	solana.WrappedSol,
//	cfg,
//
// )
func (m *DBC) InitConfig(
	ctx context.Context,
	wsClient *ws.Client,
	payerWallet *solana.Wallet,
	quoteMint solana.PublicKey,
	cfg *dbc.ConfigParameters,
) (string, *dbc.PoolConfig, error) {
	if !m.config.Equals(zeroPublicKey) {
		config, err := m.GetConfig(ctx, m.config)
		if err != nil {
			return "", nil, err
		}

		if config == nil {
			return "", nil, fmt.Errorf("config not exist")
		}

		return m.config.String(), config, nil
	}

	configWallet := solana.NewWallet()
	m.config = configWallet.PublicKey()
	if _, err := m.CreateConfig(ctx, wsClient, payerWallet, configWallet, quoteMint, cfg); err != nil {
		return "", nil, err
	}
	config, err := m.GetConfig(ctx, m.config)
	if err != nil {
		return "", nil, err
	}
	return m.config.String(), config, nil
}
