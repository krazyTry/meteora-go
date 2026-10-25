package dynamic_bonding_curve

import (
	"context"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"

	"github.com/shopspring/decimal"
	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

func (s *DynamicBondingCurve) GetPoolConfig(ctx context.Context, configAddress solanago.PublicKey) (*PoolConfig, error) {
	found, err := s.GetPoolConfigWithKind(ctx, configAddress)
	if err != nil {
		return nil, err
	}
	return found.PoolConfig, nil
}

func (s *DynamicBondingCurve) GetPoolConfigs(ctx context.Context) ([]ProgramAccount[PoolConfig], error) {
	return s.poolConfigs(ctx, nil, 0)
}

func (s *DynamicBondingCurve) GetPoolConfigsByOwner(ctx context.Context, owner solanago.PublicKey) ([]ProgramAccount[PoolConfig], error) {
	return s.poolConfigs(ctx, &owner, helpers.PoolConfigFeeClaimerOffset)
}

func (s *DynamicBondingCurve) GetPool(ctx context.Context, poolAddress solanago.PublicKey) (*VirtualPool, error) {
	found, err := s.GetPoolWithKind(ctx, poolAddress)
	if err != nil {
		return nil, err
	}
	return found.Pool, nil
}

func (s *DynamicBondingCurve) GetPools(ctx context.Context) ([]ProgramAccount[VirtualPool], error) {
	return s.pools(ctx, nil, 0)
}

func (s *DynamicBondingCurve) GetPoolsByConfig(ctx context.Context, configAddress solanago.PublicKey) ([]ProgramAccount[VirtualPool], error) {
	return s.pools(ctx, &configAddress, helpers.VirtualPoolConfigOffset)
}

func (s *DynamicBondingCurve) GetPoolsByCreator(ctx context.Context, creatorAddress solanago.PublicKey) ([]ProgramAccount[VirtualPool], error) {
	return s.pools(ctx, &creatorAddress, helpers.VirtualPoolCreatorOffset)
}

func (s *DynamicBondingCurve) GetPoolByBaseMint(ctx context.Context, baseMint solanago.PublicKey) (*ProgramAccount[VirtualPool], error) {
	pools, err := s.pools(ctx, &baseMint, helpers.VirtualPoolBaseMintOffset)
	if err != nil {
		return nil, err
	}
	if len(pools) == 0 {
		return nil, nil
	}
	return &pools[0], nil
}

func (s *DynamicBondingCurve) GetPoolMigrationQuoteThreshold(ctx context.Context, poolAddress solanago.PublicKey) (*big.Int, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return nil, err
	}
	config, err := s.GetPoolConfig(ctx, pool.PoolState.Config)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetUint64(config.MigrationQuoteThreshold), nil
}

func (s *DynamicBondingCurve) GetPoolCurveProgress(ctx context.Context, poolAddress solanago.PublicKey) (float64, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return 0, err
	}
	config, err := s.GetPoolConfig(ctx, pool.PoolState.Config)
	if err != nil {
		return 0, err
	}
	return clampProgress(decimal.NewFromBigInt(new(big.Int).SetUint64(pool.PoolState.QuoteReserve), 0), decimal.NewFromBigInt(new(big.Int).SetUint64(config.MigrationQuoteThreshold), 0)), nil
}

// GetPoolQuoteTokenCurveProgress is the quote reserve divided by the migration threshold, from 0 to 1.
func (s *DynamicBondingCurve) GetPoolQuoteTokenCurveProgress(ctx context.Context, poolAddress solanago.PublicKey) (float64, error) {
	return s.GetPoolCurveProgress(ctx, poolAddress)
}

// GetPoolBaseTokenCurveProgress is the base sold so far divided by the base the curve sells before migration.
func (s *DynamicBondingCurve) GetPoolBaseTokenCurveProgress(ctx context.Context, poolAddress solanago.PublicKey) (float64, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return 0, err
	}
	config, err := s.GetPoolConfig(ctx, pool.PoolState.Config)
	if err != nil {
		return 0, err
	}
	curve := append([]shared.LiquidityDistributionParameters(nil), configCurve(config)...)
	start := config.SqrtStartPrice.BigInt()
	sold, err := helpers.GetBaseTokenForSwap(start, pool.PoolState.SqrtPrice.BigInt(), curve)
	if err != nil {
		return 0, err
	}
	total, err := helpers.GetBaseTokenForSwap(start, config.MigrationSqrtPrice.BigInt(), curve)
	if err != nil {
		return 0, err
	}
	return clampProgress(decimal.NewFromBigInt(sold, 0), decimal.NewFromBigInt(total, 0)), nil
}

func clampProgress(numerator, denominator decimal.Decimal) float64 {
	if denominator.IsZero() {
		return 0
	}
	progress, _ := numerator.Div(denominator).Float64()
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func configCurve(config *PoolConfig) []shared.LiquidityDistributionParameters {
	out := make([]shared.LiquidityDistributionParameters, len(config.Curve))
	for i, point := range config.Curve {
		out[i] = shared.LiquidityDistributionParameters{SqrtPrice: point.SqrtPrice, Liquidity: point.Liquidity}
	}
	return out
}

func (s *DynamicBondingCurve) GetPoolMetadata(ctx context.Context, poolAddress solanago.PublicKey) ([]VirtualPoolMetadata, error) {
	// filters := helpers.CreateProgramAccountFilter(poolAddress, 8)
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyVirtualPoolMetadata, &helpers.Filter{
		Owner:  poolAddress,
		Offset: helpers.ComputeStructOffset(new(shared.VirtualPoolMetadata), "VirtualPool"),
	})
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]VirtualPoolMetadata, 0)
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_VirtualPoolMetadata(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, *parsed)
	}
	return out, nil
}

func (s *DynamicBondingCurve) GetPartnerMetadata(ctx context.Context, partnerAddress solanago.PublicKey) ([]PartnerMetadata, error) {
	// filters := helpers.CreateProgramAccountFilter(partnerAddress, 8)
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPartnerMetadata, &helpers.Filter{
		Owner:  partnerAddress,
		Offset: helpers.ComputeStructOffset(new(shared.PartnerMetadata), "FeeClaimer"),
	})
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]PartnerMetadata, 0)
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_PartnerMetadata(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, *parsed)
	}
	return out, nil
}

type rawProgramAccount struct {
	Pubkey solanago.PublicKey
	Data   []byte
}

func (s *DynamicBondingCurve) programAccounts(ctx context.Context, disc [8]byte, key *solanago.PublicKey, offset uint64) ([]rawProgramAccount, error) {
	filters := []rpc.RPCFilter{{
		Memcmp: &rpc.RPCFilterMemcmp{Offset: 0, Bytes: disc[:]},
	}}
	if key != nil {
		filters = append(filters, rpc.RPCFilter{
			Memcmp: &rpc.RPCFilterMemcmp{Offset: offset, Bytes: key.Bytes()},
		})
	}
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]rawProgramAccount, 0, len(accounts))
	for _, acc := range accounts {
		out = append(out, rawProgramAccount{Pubkey: acc.Pubkey, Data: acc.Account.Data.GetBinary()})
	}
	return out, nil
}

func (s *DynamicBondingCurve) poolConfigs(ctx context.Context, key *solanago.PublicKey, offset uint64) ([]ProgramAccount[PoolConfig], error) {
	plain, err := s.programAccounts(ctx, dbcidl.Account_PoolConfig, key, offset)
	if err != nil {
		return nil, err
	}
	hooked, err := s.programAccounts(ctx, dbcidl.Account_ConfigWithTransferHook, key, offset)
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[PoolConfig], 0, len(plain)+len(hooked))
	for _, acc := range plain {
		parsed, err := dbcidl.ParseAccount_PoolConfig(acc.Data)
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[PoolConfig]{Pubkey: acc.Pubkey, Account: parsed})
	}
	for _, acc := range hooked {
		parsed, err := dbcidl.ParseAccount_ConfigWithTransferHook(acc.Data)
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[PoolConfig]{Pubkey: acc.Pubkey, Account: &parsed.Config})
	}
	return out, nil
}

func (s *DynamicBondingCurve) pools(ctx context.Context, key *solanago.PublicKey, offset uint64) ([]ProgramAccount[VirtualPool], error) {
	plain, err := s.programAccounts(ctx, dbcidl.Account_VirtualPool, key, offset)
	if err != nil {
		return nil, err
	}
	hooked, err := s.programAccounts(ctx, dbcidl.Account_TransferHookPool, key, offset)
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[VirtualPool], 0, len(plain)+len(hooked))
	for _, acc := range plain {
		parsed, err := dbcidl.ParseAccount_VirtualPool(acc.Data)
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[VirtualPool]{Pubkey: acc.Pubkey, Account: parsed})
	}
	for _, acc := range hooked {
		parsed, err := dbcidl.ParseAccount_TransferHookPool(acc.Data)
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[VirtualPool]{Pubkey: acc.Pubkey, Account: &VirtualPool{PoolState: parsed.PoolState}})
	}
	return out, nil
}
