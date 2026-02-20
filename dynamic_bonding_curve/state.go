package dynamic_bonding_curve

import (
	"context"
	"fmt"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type StateService struct {
	*DynamicBondingCurveProgram
}

func NewStateService(rpcClient *rpc.Client, commitment rpc.CommitmentType) *StateService {
	return &StateService{DynamicBondingCurveProgram: NewDynamicBondingCurveProgram(rpcClient, commitment)}
}

func (s *StateService) GetPoolConfig(ctx context.Context, configAddress solanago.PublicKey) (*PoolConfig, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, configAddress, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("account not found")
	}
	parsed, err := dbcidl.ParseAccount_PoolConfig(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (s *StateService) GetPoolConfigs(ctx context.Context) ([]ProgramAccount[PoolConfig], error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPoolConfig, nil)
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[PoolConfig], 0)
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_PoolConfig(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[PoolConfig]{Pubkey: acc.Pubkey, Account: parsed})
	}
	return out, nil
}

func (s *StateService) GetPoolConfigsByOwner(ctx context.Context, owner solanago.PublicKey) ([]ProgramAccount[PoolConfig], error) {
	// filters := helpers.CreateProgramAccountFilter(owner, 72)
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPoolConfig, nil)
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[PoolConfig], 0)
	for _, acc := range accounts {
		if !owner.Equals(acc.Pubkey) {
			continue
		}
		parsed, err := dbcidl.ParseAccount_PoolConfig(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[PoolConfig]{Pubkey: acc.Pubkey, Account: parsed})
	}
	return out, nil
}

func (s *StateService) GetPool(ctx context.Context, poolAddress solanago.PublicKey) (*VirtualPool, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, poolAddress, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("account not found")
	}
	parsed, err := dbcidl.ParseAccount_VirtualPool(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (s *StateService) GetPools(ctx context.Context) ([]ProgramAccount[VirtualPool], error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyVirtualPool, nil)
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[VirtualPool], 0)
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_VirtualPool(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[VirtualPool]{Pubkey: acc.Pubkey, Account: parsed})
	}
	return out, nil
}

func (s *StateService) GetPoolsByConfig(ctx context.Context, configAddress solanago.PublicKey) ([]ProgramAccount[VirtualPool], error) {
	// filters := helpers.CreateProgramAccountFilter(configAddress, 72)
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyVirtualPool, &helpers.Filter{
		Owner:  configAddress,
		Offset: helpers.ComputeStructOffset(new(shared.VirtualPool), "Config"),
	})
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[VirtualPool], 0)
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_VirtualPool(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[VirtualPool]{Pubkey: acc.Pubkey, Account: parsed})
	}
	return out, nil
}

func (s *StateService) GetPoolsByCreator(ctx context.Context, creatorAddress solanago.PublicKey) ([]ProgramAccount[VirtualPool], error) {
	// filters := helpers.CreateProgramAccountFilter(creatorAddress, 104)
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyVirtualPool, &helpers.Filter{
		Owner:  creatorAddress,
		Offset: helpers.ComputeStructOffset(new(shared.VirtualPool), "Creator"),
	})
	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]ProgramAccount[VirtualPool], 0)
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_VirtualPool(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		out = append(out, ProgramAccount[VirtualPool]{Pubkey: acc.Pubkey, Account: parsed})
	}
	return out, nil
}

func (s *StateService) GetPoolByBaseMint(ctx context.Context, baseMint solanago.PublicKey) (*ProgramAccount[VirtualPool], error) {
	// filters := helpers.CreateProgramAccountFilter(baseMint, 136)

	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyVirtualPool, &helpers.Filter{
		Owner:  baseMint,
		Offset: helpers.ComputeStructOffset(new(shared.VirtualPool), "BaseMint"),
	})

	accounts, err := s.RPC.GetProgramAccountsWithOpts(ctx, helpers.DynamicBondingCurveProgramID, &rpc.GetProgramAccountsOpts{Commitment: s.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	for _, acc := range accounts {
		parsed, err := dbcidl.ParseAccount_VirtualPool(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		return &ProgramAccount[VirtualPool]{Pubkey: acc.Pubkey, Account: parsed}, nil
	}
	return nil, nil
}

func (s *StateService) GetPoolMigrationQuoteThreshold(ctx context.Context, poolAddress solanago.PublicKey) (*big.Int, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return nil, err
	}
	config, err := s.GetPoolConfig(ctx, pool.Config)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetUint64(config.MigrationQuoteThreshold), nil
}

func (s *StateService) GetPoolCurveProgress(ctx context.Context, poolAddress solanago.PublicKey) (float64, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return 0, err
	}
	config, err := s.GetPoolConfig(ctx, pool.Config)
	if err != nil {
		return 0, err
	}
	quoteReserve := decimal.NewFromInt(int64(pool.QuoteReserve))
	threshold := decimal.NewFromInt(int64(config.MigrationQuoteThreshold))
	if threshold.IsZero() {
		return 0, nil
	}
	progress, _ := quoteReserve.Div(threshold).Float64()
	if progress < 0 {
		return 0, nil
	}
	if progress > 1 {
		return 1, nil
	}
	return progress, nil
}

func (s *StateService) GetPoolMetadata(ctx context.Context, poolAddress solanago.PublicKey) ([]VirtualPoolMetadata, error) {
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

func (s *StateService) GetPartnerMetadata(ctx context.Context, partnerAddress solanago.PublicKey) ([]PartnerMetadata, error) {
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

func (s *StateService) GetDammV1LockEscrow(ctx context.Context, lockEscrowAddress solanago.PublicKey) (*LockEscrow, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, lockEscrowAddress, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, nil
	}
	parsed, err := dbcidl.ParseAccount_LockEscrow(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
