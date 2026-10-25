package dammv2

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"

	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/programs/system"
	"github.com/solana-foundation/solana-go/v2/rpc"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

// CpAmm SDK class to interact with DAMM-V2.
type CpAmm struct {
	Client         *rpc.Client
	Commitment     rpc.CommitmentType
	PoolAuthority  solanago.PublicKey
	EventAuthority solanago.PublicKey
}

func NewCpAmm(client *rpc.Client, commitment rpc.CommitmentType) *CpAmm {
	return &CpAmm{
		Client:         client,
		Commitment:     commitment,
		PoolAuthority:  DerivePoolAuthority(),
		EventAuthority: DeriveEventAuthority(),
	}
}

// prepareTokenAccounts retrieves or creates token accounts for token A and B.
func (c *CpAmm) prepareTokenAccounts(ctx context.Context, params PrepareTokenAccountParams) (tokenAAta solanago.PublicKey, tokenBAta solanago.PublicKey, instructions []solanago.Instruction, err error) {
	instructions = []solanago.Instruction{}
	ataA, ixA, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, params.TokenAMint, params.TokenAOwner, params.Payer, params.TokenAProgram)
	if err != nil {
		return solanago.PublicKey{}, solanago.PublicKey{}, nil, err
	}
	ataB, ixB, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, params.TokenBMint, params.TokenBOwner, params.Payer, params.TokenBProgram)
	if err != nil {
		return solanago.PublicKey{}, solanago.PublicKey{}, nil, err
	}
	if ixA != nil {
		instructions = append(instructions, ixA)
	}
	if ixB != nil {
		instructions = append(instructions, ixB)
	}
	return ataA, ataB, instructions, nil
}

func (c *CpAmm) getTokenBadgeAccounts(tokenAMint, tokenBMint solanago.PublicKey) []*solanago.AccountMeta {
	return []*solanago.AccountMeta{
		solanago.NewAccountMeta(DeriveTokenBadgeAddress(tokenAMint), false, false),
		solanago.NewAccountMeta(DeriveTokenBadgeAddress(tokenBMint), false, false),
	}
}

// buildAddLiquidityInstruction builds an add liquidity instruction.
func (c *CpAmm) buildAddLiquidityInstruction(params BuildAddLiquidityParams) (solanago.Instruction, error) {
	ixParams := dammv2gen.AddLiquidityParameters{
		LiquidityDelta:        u128FromBig(params.LiquidityDelta),
		TokenAAmountThreshold: toU64(params.TokenAAmountThreshold),
		TokenBAmountThreshold: toU64(params.TokenBAmountThreshold),
	}
	return dammv2gen.NewAddLiquidityInstruction(
		ixParams,
		params.Pool,
		params.Position,
		params.TokenAAccount,
		params.TokenBAccount,
		params.TokenAVault,
		params.TokenBVault,
		params.TokenAMint,
		params.TokenBMint,
		params.PositionNftAccount,
		params.Owner,
		params.TokenAProgram,
		params.TokenBProgram,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
}

// buildRemoveAllLiquidityInstruction builds remove all liquidity instruction.
func (c *CpAmm) buildRemoveAllLiquidityInstruction(params BuildRemoveAllLiquidityInstructionParams) (solanago.Instruction, error) {
	return dammv2gen.NewRemoveAllLiquidityInstruction(
		toU64(params.TokenAAmountThreshold),
		toU64(params.TokenBAmountThreshold),
		params.PoolAuthority,
		params.Pool,
		params.Position,
		params.TokenAAccount,
		params.TokenBAccount,
		params.TokenAVault,
		params.TokenBVault,
		params.TokenAMint,
		params.TokenBMint,
		params.PositionNftAccount,
		params.Owner,
		params.TokenAProgram,
		params.TokenBProgram,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
}

// buildClaimPositionFeeInstruction builds claim position fee instruction.
func (c *CpAmm) buildClaimPositionFeeInstruction(params ClaimPositionFeeInstructionParams) (solanago.Instruction, error) {
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)
	return dammv2gen.NewClaimPositionFeeInstruction(
		params.PoolAuthority,
		params.Pool,
		params.Position,
		params.TokenAAccount,
		params.TokenBAccount,
		params.PoolState.TokenAVault,
		params.PoolState.TokenBVault,
		params.PoolState.TokenAMint,
		params.PoolState.TokenBMint,
		params.PositionNftAccount,
		params.Owner,
		tokenAProgram,
		tokenBProgram,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
}

// buildClosePositionInstruction builds close position instruction.
func (c *CpAmm) buildClosePositionInstruction(params ClosePositionInstructionParams) (solanago.Instruction, error) {
	return dammv2gen.NewClosePositionInstruction(
		params.PositionNftMint,
		params.PositionNftAccount,
		params.Pool,
		params.Position,
		params.PoolAuthority,
		params.Owner,
		params.Owner,
		solanago.Token2022ProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
}

// buildRefreshVestingInstruction builds refresh vesting instruction.
func (c *CpAmm) buildRefreshVestingInstruction(params RefreshVestingParams) (solanago.Instruction, error) {
	ix, err := dammv2gen.NewRefreshVestingInstruction(
		params.Pool,
		params.Position,
		params.PositionNftAccount,
		params.Owner,
	)
	if err != nil {
		return nil, err
	}
	if gen, ok := ix.(*solanago.GenericInstruction); ok {
		for _, v := range params.VestingAccounts {
			gen.AccountValues = append(gen.AccountValues, solanago.NewAccountMeta(v, true, false))
		}
	}
	return ix, nil
}

// buildLiquidatePositionInstruction builds claim fee, remove all liquidity, claim rewards, and close position.
func (c *CpAmm) buildLiquidatePositionInstruction(ctx context.Context, params BuildLiquidatePositionInstructionParams) ([]solanago.Instruction, error) {
	positionNftMint := params.PositionState.NftMint
	pool := params.PositionState.Pool
	poolState := params.PoolState

	tokenAMint := poolState.TokenAMint
	tokenBMint := poolState.TokenBMint
	tokenAVault := poolState.TokenAVault
	tokenBVault := poolState.TokenBVault

	tokenAProgram := helpers.GetTokenProgram(poolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(poolState.TokenBFlag)

	var instructions []solanago.Instruction

	claimIx, err := c.buildClaimPositionFeeInstruction(ClaimPositionFeeInstructionParams{
		Owner:              params.Owner,
		PoolAuthority:      c.PoolAuthority,
		Pool:               pool,
		Position:           params.Position,
		PositionNftAccount: params.PositionNftAccount,
		TokenAAccount:      params.TokenAAccount,
		TokenBAccount:      params.TokenBAccount,
		PoolState:          poolState,
	})
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, claimIx)

	removeIx, err := c.buildRemoveAllLiquidityInstruction(BuildRemoveAllLiquidityInstructionParams{
		PoolAuthority:         c.PoolAuthority,
		Owner:                 params.Owner,
		Pool:                  pool,
		Position:              params.Position,
		PositionNftAccount:    params.PositionNftAccount,
		TokenAAccount:         params.TokenAAccount,
		TokenBAccount:         params.TokenBAccount,
		TokenAAmountThreshold: params.TokenAAmountThreshold,
		TokenBAmountThreshold: params.TokenBAmountThreshold,
		TokenAMint:            tokenAMint,
		TokenBMint:            tokenBMint,
		TokenAVault:           tokenAVault,
		TokenBVault:           tokenBVault,
		TokenAProgram:         tokenAProgram,
		TokenBProgram:         tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, removeIx)

	skipReward := uint8(0)
	if params.IsSkipReward {
		skipReward = 1
	}
	for rewardIndex, rewardInfo := range params.PoolState.RewardInfos {
		if rewardInfo.Initialized == 0 {
			continue
		}
		positionReward := params.PositionState.RewardInfos[rewardIndex]
		rewardFullySettled := rewardInfo.RewardDurationEnd <= rewardInfo.LastUpdateTime
		nothingToClaim := positionReward.RewardPendings == 0 && positionReward.RewardPerTokenCheckpoint == rewardInfo.RewardPerTokenStored
		if rewardFullySettled && nothingToClaim {
			continue
		}
		rewardTokenProgram := helpers.GetTokenProgram(rewardInfo.RewardTokenFlag)
		userRewardAccount, createRewardAtaIx, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, rewardInfo.Mint, params.Owner, params.Owner, rewardTokenProgram)
		if err != nil {
			return nil, err
		}
		if createRewardAtaIx != nil {
			instructions = append(instructions, createRewardAtaIx)
		}
		claimRewardIx, err := dammv2gen.NewClaimRewardInstruction(
			uint8(rewardIndex),
			skipReward,
			c.PoolAuthority,
			pool,
			params.Position,
			rewardInfo.Vault,
			rewardInfo.Mint,
			userRewardAccount,
			params.PositionNftAccount,
			params.Owner,
			rewardTokenProgram,
			c.EventAuthority,
			dammv2gen.ProgramID,
		)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, claimRewardIx)
	}

	closeIx, err := c.buildClosePositionInstruction(ClosePositionInstructionParams{
		Owner:              params.Owner,
		PoolAuthority:      c.PoolAuthority,
		Pool:               pool,
		Position:           params.Position,
		PositionNftMint:    positionNftMint,
		PositionNftAccount: params.PositionNftAccount,
	})
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, closeIx)

	return instructions, nil
}

// buildCreatePositionInstruction builds create position instruction.
func (c *CpAmm) buildCreatePositionInstruction(params CreatePositionParams) (solanago.Instruction, solanago.PublicKey, solanago.PublicKey, error) {
	position := DerivePositionAddress(params.PositionNft)
	positionNftAccount := DerivePositionNftAccount(params.PositionNft)
	ix, err := dammv2gen.NewCreatePositionInstruction(
		params.Owner,
		params.PositionNft,
		positionNftAccount,
		params.Pool,
		position,
		c.PoolAuthority,
		params.Payer,
		solanago.Token2022ProgramID,
		system.ProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	return ix, position, positionNftAccount, nil
}

// prepareCreatePoolParams prepares common pool creation params.
func (c *CpAmm) prepareCreatePoolParams(ctx context.Context, params PrepareCustomizablePoolParams) (PreparedCreatePoolInternal, error) {
	position := DerivePositionAddress(params.PositionNft)
	positionNftAccount := DerivePositionNftAccount(params.PositionNft)
	tokenAVault := DeriveTokenVaultAddress(params.TokenAMint, params.Pool)
	tokenBVault := DeriveTokenVaultAddress(params.TokenBMint, params.Pool)

	payerTokenA, payerTokenB, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Payer,
		TokenAOwner:   params.Payer,
		TokenBOwner:   params.Payer,
		TokenAMint:    params.TokenAMint,
		TokenBMint:    params.TokenBMint,
		TokenAProgram: params.TokenAProgram,
		TokenBProgram: params.TokenBProgram,
	})
	if err != nil {
		return PreparedCreatePoolInternal{}, err
	}

	if params.TokenAMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Payer, payerTokenA, toU64(params.TokenAAmount))
		preIxs = append(preIxs, wrapIxs...)
	}
	if params.TokenBMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Payer, payerTokenB, toU64(params.TokenBAmount))
		preIxs = append(preIxs, wrapIxs...)
	}

	badgeAccounts := c.getTokenBadgeAccounts(params.TokenAMint, params.TokenBMint)

	return PreparedCreatePoolInternal{
		Position:           position,
		PositionNftAccount: positionNftAccount,
		TokenAVault:        tokenAVault,
		TokenBVault:        tokenBVault,
		PayerTokenA:        payerTokenA,
		PayerTokenB:        payerTokenB,
		PreInstructions:    preIxs,
		TokenBadgeAccounts: badgeAccounts,
	}, nil
}

// setupFeeClaimAccounts sets up token accounts and pre/post instructions for fee claims.
func (c *CpAmm) setupFeeClaimAccounts(ctx context.Context, params SetupFeeClaimAccountsParams) (tokenAAccount solanago.PublicKey, tokenBAccount solanago.PublicKey, preInstructions []solanago.Instruction, postInstructions []solanago.Instruction, err error) {
	tokenAIsSOL := params.TokenAMint.Equals(helpers.NativeMint)
	tokenBIsSOL := params.TokenBMint.Equals(helpers.NativeMint)
	hasSolToken := tokenAIsSOL || tokenBIsSOL

	preInstructions = []solanago.Instruction{}
	postInstructions = []solanago.Instruction{}

	tokenAOwner := params.Owner
	tokenBOwner := params.Owner
	if params.Receiver != nil {
		if (tokenAIsSOL || tokenBIsSOL) && params.TempWSolAccount == nil {
			return solanago.PublicKey{}, solanago.PublicKey{}, nil, nil, errors.New("temp wSOL account required when receiver is set for SOL")
		}
		if tokenAIsSOL {
			tokenAOwner = *params.TempWSolAccount
		} else {
			tokenAOwner = *params.Receiver
		}
		if tokenBIsSOL {
			tokenBOwner = *params.TempWSolAccount
		} else {
			tokenBOwner = *params.Receiver
		}
	}

	ataA, ataB, ixs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Payer,
		TokenAOwner:   tokenAOwner,
		TokenBOwner:   tokenBOwner,
		TokenAMint:    params.TokenAMint,
		TokenBMint:    params.TokenBMint,
		TokenAProgram: params.TokenAProgram,
		TokenBProgram: params.TokenBProgram,
	})
	if err != nil {
		return solanago.PublicKey{}, solanago.PublicKey{}, nil, nil, err
	}
	preInstructions = append(preInstructions, ixs...)
	if hasSolToken {
		owner := params.Owner
		if params.TempWSolAccount != nil {
			owner = *params.TempWSolAccount
		}
		receiver := params.Owner
		if params.Receiver != nil {
			receiver = *params.Receiver
		}
		closeIx, _ := helpers.UnwrapSOLInstruction(owner, receiver, true)
		if closeIx != nil {
			postInstructions = append(postInstructions, closeIx)
		}
	}
	return ataA, ataB, preInstructions, postInstructions, nil
}

// FetchConfigState fetches Config account.
func (c *CpAmm) FetchConfigState(ctx context.Context, config solanago.PublicKey) (*ConfigState, error) {
	acc, err := c.Client.GetAccountInfoWithOpts(ctx, config, &rpc.GetAccountInfoOpts{Commitment: c.Commitment})
	if err != nil || acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("config account %s not found", config.String())
	}
	parsed, err := dammv2gen.ParseAnyAccount(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	cfg, ok := parsed.(*dammv2gen.Config)
	if !ok {
		return nil, errors.New("invalid config account")
	}
	return cfg, nil
}

func (c *CpAmm) FetchPoolState(ctx context.Context, pool solanago.PublicKey) (*PoolState, error) {
	acc, err := c.Client.GetAccountInfoWithOpts(ctx, pool, &rpc.GetAccountInfoOpts{Commitment: c.Commitment})
	if err != nil || acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("pool account %s not found", pool.String())
	}
	parsed, err := dammv2gen.ParseAnyAccount(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	pl, ok := parsed.(*dammv2gen.Pool)
	if !ok {
		return nil, errors.New("invalid pool account")
	}
	return pl, nil
}

func (c *CpAmm) FetchPoolStatesByTokenAMint(ctx context.Context, tokenAMint solanago.PublicKey) ([]*AccountWithPool, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPool, &helpers.Filter{
		Owner:  tokenAMint,
		Offset: helpers.ComputeStructOffset(new(dammv2gen.Pool), "TokenAMint"),
	})

	// return c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*AccountWithPool{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if pl, ok := parsed.(*dammv2gen.Pool); ok {
			out = append(out, &AccountWithPool{PublicKey: acc.Pubkey, Account: pl})
		}
	}
	return out, nil
}

func (c *CpAmm) fetchPoolStatesByMintOffset(ctx context.Context, mint solanago.PublicKey, field string) ([]*AccountWithPool, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPool, &helpers.Filter{
		Owner:  mint,
		Offset: helpers.ComputeStructOffset(new(dammv2gen.Pool), field),
	})
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*AccountWithPool{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if pl, ok := parsed.(*dammv2gen.Pool); ok {
			out = append(out, &AccountWithPool{PublicKey: acc.Pubkey, Account: pl})
		}
	}
	return out, nil
}

func (c *CpAmm) FetchPoolStatesByTokenBMint(ctx context.Context, tokenBMint solanago.PublicKey) ([]*AccountWithPool, error) {
	return c.fetchPoolStatesByMintOffset(ctx, tokenBMint, "TokenBMint")
}

func (c *CpAmm) FetchPoolStatesByTokenMint(ctx context.Context, tokenMint solanago.PublicKey) ([]*AccountWithPool, error) {
	tokenAPools, err := c.FetchPoolStatesByTokenAMint(ctx, tokenMint)
	if err != nil {
		return nil, err
	}
	tokenBPools, err := c.FetchPoolStatesByTokenBMint(ctx, tokenMint)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]*AccountWithPool, 0, len(tokenAPools)+len(tokenBPools))
	for _, pool := range append(tokenAPools, tokenBPools...) {
		key := pool.PublicKey.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, pool)
	}
	return out, nil
}

func (c *CpAmm) FetchPoolFees(ctx context.Context, pool solanago.PublicKey) (DecodedPoolFees, error) {
	poolState, err := c.FetchPoolState(ctx, pool)
	if err != nil {
		return nil, err
	}
	data := poolState.PoolFees.BaseFee.BaseFeeInfo.Data[:]
	modeIndex := data[8]
	baseFeeMode := BaseFeeMode(modeIndex)
	switch baseFeeMode {
	case BaseFeeModeFeeTimeSchedulerLinear, BaseFeeModeFeeTimeSchedulerExponential:
		return helpers.DecodePodAlignedFeeTimeScheduler(data)
	case BaseFeeModeRateLimiter:
		return helpers.DecodePodAlignedFeeRateLimiter(data)
	case BaseFeeModeFeeMarketCapSchedulerLinear, BaseFeeModeFeeMarketCapSchedulerExp:
		return helpers.DecodePodAlignedFeeMarketCapScheduler(data)
	default:
		return nil, fmt.Errorf("invalid base fee mode: %d", baseFeeMode)
	}
}

func (c *CpAmm) FetchPositionState(ctx context.Context, position solanago.PublicKey) (*PositionState, error) {
	acc, err := c.Client.GetAccountInfoWithOpts(ctx, position, &rpc.GetAccountInfoOpts{Commitment: c.Commitment})
	if err != nil || acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("position account %s not found", position.String())
	}
	parsed, err := dammv2gen.ParseAnyAccount(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	pos, ok := parsed.(*dammv2gen.Position)
	if !ok {
		return nil, errors.New("invalid position account")
	}
	return pos, nil
}

func (c *CpAmm) GetMultipleConfigs(ctx context.Context, configs []solanago.PublicKey) ([]*ConfigState, error) {
	accs, err := c.Client.GetMultipleAccountsWithOpts(ctx, configs, &rpc.GetMultipleAccountsOpts{Commitment: c.Commitment})
	if err != nil {
		return nil, err
	}
	out := make([]*ConfigState, 0, len(configs))
	for i, acc := range accs.Value {
		if acc == nil {
			return nil, fmt.Errorf("config account %s not found", configs[i].String())
		}
		parsed, err := dammv2gen.ParseAnyAccount(acc.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		cfg, ok := parsed.(*dammv2gen.Config)
		if !ok {
			return nil, errors.New("invalid config account")
		}
		out = append(out, cfg)
	}
	return out, nil
}

func (c *CpAmm) GetMultiplePools(ctx context.Context, pools []solanago.PublicKey) ([]*PoolState, error) {
	accs, err := c.Client.GetMultipleAccountsWithOpts(ctx, pools, &rpc.GetMultipleAccountsOpts{Commitment: c.Commitment})
	if err != nil {
		return nil, err
	}
	out := make([]*PoolState, 0, len(pools))
	for i, acc := range accs.Value {
		if acc == nil {
			return nil, fmt.Errorf("pool account %s not found", pools[i].String())
		}
		parsed, err := dammv2gen.ParseAnyAccount(acc.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		pl, ok := parsed.(*dammv2gen.Pool)
		if !ok {
			return nil, errors.New("invalid pool account")
		}
		out = append(out, pl)
	}
	return out, nil
}

func (c *CpAmm) GetMultiplePositions(ctx context.Context, positions []solanago.PublicKey) ([]*PositionState, error) {
	accs, err := c.Client.GetMultipleAccountsWithOpts(ctx, positions, &rpc.GetMultipleAccountsOpts{Commitment: c.Commitment})
	if err != nil {
		return nil, err
	}
	out := make([]*PositionState, 0, len(positions))
	for i, acc := range accs.Value {
		if acc == nil {
			return nil, fmt.Errorf("position account %s not found", positions[i].String())
		}
		parsed, err := dammv2gen.ParseAnyAccount(acc.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		pos, ok := parsed.(*dammv2gen.Position)
		if !ok {
			return nil, errors.New("invalid position account")
		}
		out = append(out, pos)
	}
	return out, nil
}

func (c *CpAmm) GetAllConfigs(ctx context.Context) ([]*AccountWithConfig, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyConfig, nil)
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*AccountWithConfig{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if cfg, ok := parsed.(*dammv2gen.Config); ok {
			out = append(out, &AccountWithConfig{PublicKey: acc.Pubkey, Account: cfg})
		}
	}
	return out, nil
}

// GetStaticConfigs returns static configs whose vault and creator authority are the default pubkey.
func (c *CpAmm) GetStaticConfigs(ctx context.Context) ([]*AccountWithConfig, error) {
	configs, err := c.GetAllConfigs(ctx)
	if err != nil {
		return nil, err
	}
	out := []*AccountWithConfig{}
	for _, config := range configs {
		if config.Account.ConfigType == 0 && config.Account.VaultConfigKey.Equals(solanago.PublicKey{}) && config.Account.PoolCreatorAuthority.Equals(solanago.PublicKey{}) {
			out = append(out, config)
		}
	}
	return out, nil
}

func (c *CpAmm) GetAllPools(ctx context.Context) ([]*AccountWithPool, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPool, nil)
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*AccountWithPool{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if pl, ok := parsed.(*dammv2gen.Pool); ok {
			out = append(out, &AccountWithPool{PublicKey: acc.Pubkey, Account: pl})
		}
	}
	return out, nil
}

func (c *CpAmm) GetAllPositions(ctx context.Context) ([]*AccountWithPosition, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPosition, nil)
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*AccountWithPosition{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if pos, ok := parsed.(*dammv2gen.Position); ok {
			out = append(out, &AccountWithPosition{PublicKey: acc.Pubkey, Account: pos})
		}
	}
	return out, nil
}

func (c *CpAmm) GetAllPositionsByPool(ctx context.Context, pool solanago.PublicKey) ([]*AccountWithPosition, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyPosition, &helpers.Filter{
		Owner:  pool,
		Offset: helpers.ComputeStructOffset(new(dammv2gen.Position), "Pool"),
	})
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*AccountWithPosition{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if pos, ok := parsed.(*dammv2gen.Position); ok {
			out = append(out, &AccountWithPosition{PublicKey: acc.Pubkey, Account: pos})
		}
	}
	return out, nil
}

func (c *CpAmm) GetUserPositionByPool(ctx context.Context, pool, user solanago.PublicKey) ([]*UserPosition, error) {
	positions, err := c.GetPositionsByUser(ctx, user)
	if err != nil {
		return nil, err
	}
	out := []*UserPosition{}
	for _, pos := range positions {
		if pos.PositionState.Pool.Equals(pool) {
			out = append(out, pos)
		}
	}
	return out, nil
}

func (c *CpAmm) GetPositionsByUser(ctx context.Context, user solanago.PublicKey) ([]*UserPosition, error) {
	userPositionAccounts, err := helpers.GetAllPositionNftAccountByOwner(ctx, c.Client, user)
	if err != nil {
		return nil, err
	}
	if len(userPositionAccounts) == 0 {
		return nil, nil
	}
	positionAddresses := make([]solanago.PublicKey, len(userPositionAccounts))
	for i, account := range userPositionAccounts {
		positionAddresses[i] = DerivePositionAddress(account.PositionNft)
	}
	positionStates, err := c.GetMultiplePositions(ctx, positionAddresses)
	if err != nil {
		return nil, err
	}
	positions := make([]*UserPosition, 0, len(positionAddresses))
	for i, account := range userPositionAccounts {
		posState := positionStates[i]
		positions = append(positions, &UserPosition{PositionNftAccount: account.PositionNftAccount, Position: positionAddresses[i], PositionState: posState})
	}
	sort.Slice(positions, func(i, j int) bool {
		a := totalPositionLiquidity(positions[i].PositionState)
		b := totalPositionLiquidity(positions[j].PositionState)
		return b.Cmp(a) < 0
	})
	return positions, nil
}

type UserPositionWithPool struct {
	UserPosition
	Pool      solanago.PublicKey
	PoolState *PoolState
}

func (c *CpAmm) GetPositionsByUserAndTokenMint(ctx context.Context, user, tokenMint solanago.PublicKey) ([]*UserPositionWithPool, error) {
	userPositions, err := c.GetPositionsByUser(ctx, user)
	if err != nil {
		return nil, err
	}
	if len(userPositions) == 0 {
		return nil, nil
	}
	unique := make([]solanago.PublicKey, 0)
	seen := map[string]struct{}{}
	for _, position := range userPositions {
		key := position.PositionState.Pool.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, position.PositionState.Pool)
	}
	poolStates, err := c.GetMultiplePools(ctx, unique)
	if err != nil {
		return nil, err
	}
	matched := map[string]*PoolState{}
	for i, pool := range unique {
		state := poolStates[i]
		if state == nil {
			continue
		}
		if state.TokenAMint.Equals(tokenMint) || state.TokenBMint.Equals(tokenMint) {
			matched[pool.String()] = state
		}
	}
	out := make([]*UserPositionWithPool, 0)
	for _, position := range userPositions {
		state, ok := matched[position.PositionState.Pool.String()]
		if !ok {
			continue
		}
		out = append(out, &UserPositionWithPool{
			UserPosition: *position,
			Pool:         position.PositionState.Pool,
			PoolState:    state,
		})
	}
	return out, nil
}

func (c *CpAmm) GetPositionsByDelegate(ctx context.Context, delegate solanago.PublicKey) ([]*UserPosition, error) {
	accounts, err := helpers.GetAllPositionNftAccountByDelegate(ctx, c.Client, delegate)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	positionAddresses := make([]solanago.PublicKey, len(accounts))
	for i, account := range accounts {
		positionAddresses[i] = DerivePositionAddress(account.PositionNft)
	}
	positionStates, err := c.GetMultiplePositions(ctx, positionAddresses)
	if err != nil {
		return nil, err
	}
	positions := make([]*UserPosition, 0, len(accounts))
	for i, account := range accounts {
		state := positionStates[i]
		if state == nil || state.DelegatePermission == 0 {
			continue
		}
		positions = append(positions, &UserPosition{
			PositionNftAccount: account.PositionNftAccount,
			Position:           positionAddresses[i],
			PositionState:      state,
		})
	}
	sort.Slice(positions, func(i, j int) bool {
		return totalPositionLiquidity(positions[j].PositionState).Cmp(totalPositionLiquidity(positions[i].PositionState)) < 0
	})
	return positions, nil
}

func (c *CpAmm) GetDelegatePositionByPool(ctx context.Context, pool, delegate solanago.PublicKey) ([]*UserPosition, error) {
	positions, err := c.GetPositionsByDelegate(ctx, delegate)
	if err != nil {
		return nil, err
	}
	out := make([]*UserPosition, 0)
	for _, position := range positions {
		if position.PositionState.Pool.Equals(pool) {
			out = append(out, position)
		}
	}
	return out, nil
}

func (c *CpAmm) GetPositionsByDelegateAndTokenMint(ctx context.Context, delegate, tokenMint solanago.PublicKey) ([]*UserPositionWithPool, error) {
	positions, err := c.GetPositionsByDelegate(ctx, delegate)
	if err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, nil
	}
	unique := make([]solanago.PublicKey, 0)
	seen := map[string]struct{}{}
	for _, position := range positions {
		key := position.PositionState.Pool.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, position.PositionState.Pool)
	}
	poolStates, err := c.GetMultiplePools(ctx, unique)
	if err != nil {
		return nil, err
	}
	matched := map[string]*PoolState{}
	for i, pool := range unique {
		state := poolStates[i]
		if state == nil {
			continue
		}
		if state.TokenAMint.Equals(tokenMint) || state.TokenBMint.Equals(tokenMint) {
			matched[pool.String()] = state
		}
	}
	out := make([]*UserPositionWithPool, 0)
	for _, position := range positions {
		state, ok := matched[position.PositionState.Pool.String()]
		if !ok {
			continue
		}
		out = append(out, &UserPositionWithPool{UserPosition: *position, Pool: position.PositionState.Pool, PoolState: state})
	}
	return out, nil
}

func (c *CpAmm) GetAllVestingsByPosition(ctx context.Context, position solanago.PublicKey) ([]*VestingWithAccount, error) {
	filters := helpers.CreateProgramAccountFilter(helpers.AccountKeyVesting, &helpers.Filter{
		Owner:  position,
		Offset: helpers.ComputeStructOffset(new(dammv2gen.Vesting), "Position"),
	})
	accs, err := c.Client.GetProgramAccountsWithOpts(ctx, dammv2gen.ProgramID, &rpc.GetProgramAccountsOpts{Commitment: c.Commitment, Filters: filters})
	if err != nil {
		return nil, err
	}
	out := []*VestingWithAccount{}
	for _, acc := range accs {
		parsed, err := dammv2gen.ParseAnyAccount(acc.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		if v, ok := parsed.(*dammv2gen.Vesting); ok {
			out = append(out, &VestingWithAccount{Account: acc.Pubkey, VestingState: v})
		}
	}
	return out, nil
}

func (c *CpAmm) IsLockedPosition(position *PositionState) bool {
	totalLocked := new(big.Int).Add(position.VestedLiquidity.BigInt(), position.PermanentLockedLiquidity.BigInt())
	return totalLocked.Sign() > 0
}

func (c *CpAmm) IsPermanentLockedPosition(position *PositionState) bool {
	return position.PermanentLockedLiquidity.BigInt().Sign() > 0
}

func (c *CpAmm) CanUnlockPosition(position *PositionState, vestings []*VestingWithAccount, currentPoint *big.Int) (bool, string) {
	if len(vestings) > 0 {
		if c.IsPermanentLockedPosition(position) {
			return false, "Position is permanently locked"
		}
		for _, v := range vestings {
			if !helpers.IsVestingComplete(v.VestingState, currentPoint) {
				return false, "Position has incomplete vesting schedule"
			}
		}
	}
	return true, ""
}

func totalPositionLiquidity(position *PositionState) *big.Int {
	out := new(big.Int).Add(position.VestedLiquidity.BigInt(), position.PermanentLockedLiquidity.BigInt())
	out.Add(out, position.UnlockedLiquidity.BigInt())
	return out
}

func toU64(v *big.Int) uint64 {
	if v == nil {
		return 0
	}
	return v.Uint64()
}

// Internal helper types.
type PreparedCreatePoolInternal struct {
	Position           solanago.PublicKey
	PositionNftAccount solanago.PublicKey
	TokenAVault        solanago.PublicKey
	TokenBVault        solanago.PublicKey
	PayerTokenA        solanago.PublicKey
	PayerTokenB        solanago.PublicKey
	PreInstructions    []solanago.Instruction
	TokenBadgeAccounts []*solanago.AccountMeta
}

type AccountWithConfig struct {
	PublicKey solanago.PublicKey
	Account   *dammv2gen.Config
}

type AccountWithPool struct {
	PublicKey solanago.PublicKey
	Account   *dammv2gen.Pool
}

type AccountWithPosition struct {
	PublicKey solanago.PublicKey
	Account   *dammv2gen.Position
}

type UserPosition struct {
	PositionNftAccount solanago.PublicKey
	Position           solanago.PublicKey
	PositionState      *dammv2gen.Position
}
