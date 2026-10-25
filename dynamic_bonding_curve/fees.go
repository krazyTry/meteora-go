package dynamic_bonding_curve

import (
	"context"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

// PoolFeeSummary is the unclaimed and lifetime trading fees of one pool.
type PoolFeeSummary struct {
	PoolAddress          solanago.PublicKey
	PartnerBaseFee       uint64
	PartnerQuoteFee      uint64
	CreatorBaseFee       uint64
	CreatorQuoteFee      uint64
	TotalTradingBaseFee  uint64
	TotalTradingQuoteFee uint64
}

// FeePartyBreakdown is one side of the estimated trading-fee split.
type FeePartyBreakdown struct {
	UnclaimedBaseFee  uint64
	UnclaimedQuoteFee uint64
	ClaimedBaseFee    uint64
	ClaimedQuoteFee   uint64
	TotalBaseFee      uint64
	TotalQuoteFee     uint64
}

// PoolFeeBreakdown estimates claimed and unclaimed fees. The creator share of the lifetime total is floored.
type PoolFeeBreakdown struct {
	Creator FeePartyBreakdown
	Partner FeePartyBreakdown
}

func FeeBreakdownFromPool(pool *VirtualPool, creatorTradingFeePercentage uint8) PoolFeeBreakdown {
	state := pool.PoolState
	baseCreatorClaimed, basePartnerClaimed := claimedShares(state.Metrics.TotalTradingBaseFee, state.CreatorBaseFee, state.PartnerBaseFee, creatorTradingFeePercentage)
	quoteCreatorClaimed, quotePartnerClaimed := claimedShares(state.Metrics.TotalTradingQuoteFee, state.CreatorQuoteFee, state.PartnerQuoteFee, creatorTradingFeePercentage)
	return PoolFeeBreakdown{
		Creator: FeePartyBreakdown{
			UnclaimedBaseFee:  state.CreatorBaseFee,
			UnclaimedQuoteFee: state.CreatorQuoteFee,
			ClaimedBaseFee:    baseCreatorClaimed,
			ClaimedQuoteFee:   quoteCreatorClaimed,
			TotalBaseFee:      state.CreatorBaseFee + baseCreatorClaimed,
			TotalQuoteFee:     state.CreatorQuoteFee + quoteCreatorClaimed,
		},
		Partner: FeePartyBreakdown{
			UnclaimedBaseFee:  state.PartnerBaseFee,
			UnclaimedQuoteFee: state.PartnerQuoteFee,
			ClaimedBaseFee:    basePartnerClaimed,
			ClaimedQuoteFee:   quotePartnerClaimed,
			TotalBaseFee:      state.PartnerBaseFee + basePartnerClaimed,
			TotalQuoteFee:     state.PartnerQuoteFee + quotePartnerClaimed,
		},
	}
}

func claimedShares(total, creatorUnclaimed, partnerUnclaimed uint64, creatorPercentage uint8) (uint64, uint64) {
	creatorTotal := new(big.Int).Div(new(big.Int).Mul(new(big.Int).SetUint64(total), big.NewInt(int64(creatorPercentage))), big.NewInt(100)).Uint64()
	partnerTotal := total - creatorTotal
	return saturatingSub(creatorTotal, creatorUnclaimed), saturatingSub(partnerTotal, partnerUnclaimed)
}

func saturatingSub(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

func (s *DynamicBondingCurve) GetPoolFeeBreakdown(ctx context.Context, poolAddress solanago.PublicKey) (PoolFeeBreakdown, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return PoolFeeBreakdown{}, err
	}
	config, err := s.GetPoolConfig(ctx, pool.PoolState.Config)
	if err != nil {
		return PoolFeeBreakdown{}, err
	}
	return FeeBreakdownFromPool(pool, config.CreatorTradingFeePercentage), nil
}

func (s *DynamicBondingCurve) GetPoolsFeesByConfig(ctx context.Context, configAddress solanago.PublicKey) ([]PoolFeeSummary, error) {
	pools, err := s.GetPoolsByConfig(ctx, configAddress)
	if err != nil {
		return nil, err
	}
	return feeSummaries(pools), nil
}

func (s *DynamicBondingCurve) GetPoolsFeesByCreator(ctx context.Context, creatorAddress solanago.PublicKey) ([]PoolFeeSummary, error) {
	pools, err := s.GetPoolsByCreator(ctx, creatorAddress)
	if err != nil {
		return nil, err
	}
	return feeSummaries(pools), nil
}

func feeSummaries(pools []ProgramAccount[VirtualPool]) []PoolFeeSummary {
	out := make([]PoolFeeSummary, 0, len(pools))
	for _, pool := range pools {
		state := pool.Account.PoolState
		out = append(out, PoolFeeSummary{
			PoolAddress:          pool.Pubkey,
			PartnerBaseFee:       state.PartnerBaseFee,
			PartnerQuoteFee:      state.PartnerQuoteFee,
			CreatorBaseFee:       state.CreatorBaseFee,
			CreatorQuoteFee:      state.CreatorQuoteFee,
			TotalTradingBaseFee:  state.Metrics.TotalTradingBaseFee,
			TotalTradingQuoteFee: state.Metrics.TotalTradingQuoteFee,
		})
	}
	return out
}

// GetSwapQuoteTransferFees reads the current-epoch transfer fees used by a swap quote.
func (s *DynamicBondingCurve) GetSwapQuoteTransferFees(ctx context.Context, quoteMint solanago.PublicKey, quoteTokenFlag TokenType, baseMint *solanago.PublicKey) (*QuoteTransferFees, error) {
	return helpers.GetSwapQuoteTransferFees(ctx, s.RPC, s.Commitment, quoteMint, quoteTokenFlag, baseMint)
}

func (s *DynamicBondingCurve) GetDammV1MigrationMetadata(ctx context.Context, poolAddress solanago.PublicKey) (*dbcidl.MeteoraDammMigrationMetadata, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, helpers.DeriveDammV1MigrationMetadataAddress(poolAddress), &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, nil
	}
	return dbcidl.ParseAccount_MeteoraDammMigrationMetadata(acc.Value.Data.GetBinary())
}
