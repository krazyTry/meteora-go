package dynamic_bonding_curve

import (
	"context"
	"errors"
	"fmt"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/programs/system"
	"github.com/solana-foundation/solana-go/v2/programs/token"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

// PoolConfigWithKind is a config together with the account variant that holds it.
type PoolConfigWithKind struct {
	PoolConfig         *PoolConfig
	TransferHookConfig *dbcidl.ConfigWithTransferHook
}

// PoolWithKind is a pool together with whether it is a transfer-hook pool.
type PoolWithKind struct {
	Pool         *VirtualPool
	TransferHook bool
}

// PoolSurplus is the surplus split and whether each side has withdrawn it.
type PoolSurplus struct {
	helpers.SurplusBreakdown
	PartnerWithdrawn bool
	CreatorWithdrawn bool
}

// PoolFeeMetrics is the unclaimed partner and creator fees and the lifetime trading fees.
type PoolFeeMetrics struct {
	PartnerBaseFee       uint64
	PartnerQuoteFee      uint64
	CreatorBaseFee       uint64
	CreatorQuoteFee      uint64
	TotalTradingBaseFee  uint64
	TotalTradingQuoteFee uint64
}

// CreateTokenBadgeParams creates the token badge PDA for a quote mint.
type CreateTokenBadgeParams struct {
	TokenMint solanago.PublicKey
	Operator  solanago.PublicKey
	Signer    solanago.PublicKey
	Payer     solanago.PublicKey
}

func appendMetas(ix solanago.Instruction, metas []*solanago.AccountMeta) {
	gi, ok := ix.(*solanago.GenericInstruction)
	if !ok || len(metas) == 0 {
		return
	}
	gi.AccountValues = append(gi.AccountValues, metas...)
}

func withTokenBadge(ix solanago.Instruction, badge *solanago.PublicKey) solanago.Instruction {
	if badge == nil || badge.IsZero() {
		return ix
	}
	appendMetas(ix, []*solanago.AccountMeta{solanago.NewAccountMeta(*badge, false, false)})
	return ix
}

func transferHookInfo(hook *TransferHookAccounts) (dbcidl.TransferHookAccountsInfo, error) {
	if hook == nil {
		hook = &TransferHookAccounts{}
	}
	if len(hook.Base) > 255 || len(hook.Referral) > 255 {
		return dbcidl.TransferHookAccountsInfo{}, fmt.Errorf("transfer hook remaining accounts exceed 255")
	}
	slices := []dbcidl.RemainingAccountsSlice{{
		AccountsType: dbcidl.AccountsType_TransferHookBase,
		Length:       uint8(len(hook.Base)),
	}}
	if len(hook.Referral) > 0 {
		slices = append(slices, dbcidl.RemainingAccountsSlice{
			AccountsType: dbcidl.AccountsType_TransferHookBaseReferral,
			Length:       uint8(len(hook.Referral)),
		})
	}
	return dbcidl.TransferHookAccountsInfo{Slices: slices}, nil
}

func (s *DynamicBondingCurve) CreateConfig2(ctx context.Context, params CreateConfig2Params) (solanago.Instruction, error) {
	opt, badge, err := s.configValidation(ctx, params.CreateConfigParams, helpers.ConfigValidationOptions{
		TransferFeeParameters: params.TransferFeeParameters,
		FeeClaimer:            &params.FeeClaimer,
	}, true)
	if err != nil {
		return nil, err
	}
	if params.TokenBadge == nil {
		params.TokenBadge = badge
	}
	if err := helpers.ValidateConfigParameters(params.CreateConfigParams, opt); err != nil {
		return nil, err
	}
	ix, err := dbcidl.NewCreateConfig2Instruction(
		params.ConfigParameters,
		params.TransferFeeParameters,
		params.Config,
		params.FeeClaimer,
		params.LeftoverReceiver,
		params.QuoteMint,
		params.Payer,
		system.ProgramID,
		s.EventAuthority,
		helpers.DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, err
	}
	return withTokenBadge(ix, params.TokenBadge), nil
}

func (s *DynamicBondingCurve) CreateConfigWithTransferHook(ctx context.Context, params CreateConfigWithTransferHookParams) (solanago.Instruction, error) {
	if params.TransferHookProgram.IsZero() {
		return nil, fmt.Errorf("transfer hook program is required")
	}
	hook := params.TransferHookProgram
	opt, badge, err := s.configValidation(ctx, params.CreateConfigParams, helpers.ConfigValidationOptions{
		IsTransferHook:      true,
		TransferHookProgram: &hook,
		FeeClaimer:          &params.FeeClaimer,
	}, false)
	if err != nil {
		return nil, err
	}
	if params.TokenBadge == nil {
		params.TokenBadge = badge
	}
	if err := helpers.ValidateConfigParameters(params.CreateConfigParams, opt); err != nil {
		return nil, err
	}
	ix, err := dbcidl.NewCreateConfigWithTransferHookInstruction(
		params.ConfigParameters,
		params.Config,
		params.FeeClaimer,
		params.LeftoverReceiver,
		params.QuoteMint,
		params.TransferHookProgram,
		params.Payer,
		system.ProgramID,
		s.EventAuthority,
		helpers.DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, err
	}
	return withTokenBadge(ix, params.TokenBadge), nil
}

func (s *DynamicBondingCurve) CreateTokenBadge(ctx context.Context, params CreateTokenBadgeParams) (solanago.Instruction, error) {
	return dbcidl.NewCreateTokenBadgeInstruction(
		helpers.DeriveTokenBadgeAddress(params.TokenMint),
		params.TokenMint,
		params.Operator,
		params.Signer,
		params.Payer,
		system.ProgramID,
		s.EventAuthority,
		helpers.DynamicBondingCurveProgramID,
	)
}

func (s *DynamicBondingCurve) initializeTransferHookPool(params InitializePoolBaseParams, transferHookProgram, tokenQuoteProgram solanago.PublicKey) (solanago.Instruction, error) {
	ix, err := dbcidl.NewInitializeVirtualPoolWithToken2022TransferHookInstruction(
		InitializePoolParameters{Name: params.Name, Symbol: params.Symbol, Uri: params.URI},
		params.Config,
		s.PoolAuthority,
		params.PoolCreator,
		params.BaseMint,
		params.QuoteMint,
		params.Pool,
		params.BaseVault,
		params.QuoteVault,
		transferHookProgram,
		params.Payer,
		tokenQuoteProgram,
		solanago.Token2022ProgramID,
		system.ProgramID,
		s.EventAuthority,
		helpers.DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, err
	}
	return withTokenBadge(ix, params.TokenBadge), nil
}

// CreatePoolWithTransferHook initializes a Token-2022 pool whose base mint has a transfer hook.
// A zero transferHookProgram is read from the config account.
func (s *DynamicBondingCurve) CreatePoolWithTransferHook(ctx context.Context, params CreatePoolParams, transferHookProgram solanago.PublicKey) (solanago.Instruction, error) {
	found, err := s.GetPoolConfigWithKind(ctx, params.Config)
	if err != nil {
		return nil, err
	}
	if found.TransferHookConfig == nil {
		return nil, fmt.Errorf("config does not use a transfer hook, create the pool with the standard method")
	}
	if transferHookProgram.IsZero() {
		transferHookProgram = found.TransferHookConfig.TransferHookProgram
	} else if !transferHookProgram.Equals(found.TransferHookConfig.TransferHookProgram) {
		return nil, fmt.Errorf("transfer hook program does not match config")
	}
	if err := helpers.AssertNewPoolConfig(found.PoolConfig, true); err != nil {
		return nil, err
	}
	pool := helpers.DeriveDbcPoolAddress(found.PoolConfig.QuoteMint, params.BaseMint, params.Config)
	return s.initializeTransferHookPool(InitializePoolBaseParams{
		Name:        params.Name,
		Symbol:      params.Symbol,
		URI:         params.URI,
		Pool:        pool,
		Config:      params.Config,
		Payer:       params.Payer,
		PoolCreator: params.PoolCreator,
		BaseMint:    params.BaseMint,
		BaseVault:   helpers.DeriveDbcTokenVaultAddress(pool, params.BaseMint),
		QuoteVault:  helpers.DeriveDbcTokenVaultAddress(pool, found.PoolConfig.QuoteMint),
		QuoteMint:   found.PoolConfig.QuoteMint,
		TokenBadge:  params.TokenBadge,
	}, transferHookProgram, helpers.GetTokenProgram(TokenType(found.PoolConfig.QuoteTokenFlag)))
}

// CreateConfigAndPoolWithTransferHook builds create_config_with_transfer_hook and the matching pool initialization.
func (s *DynamicBondingCurve) CreateConfigAndPoolWithTransferHook(ctx context.Context, params CreateConfigWithTransferHookParams, pool PreCreatePoolParams) ([]solanago.Instruction, error) {
	configIx, err := s.CreateConfigWithTransferHook(ctx, params)
	if err != nil {
		return nil, err
	}
	poolAddress := helpers.DeriveDbcPoolAddress(params.QuoteMint, pool.BaseMint, params.Config)
	quoteProgram := params.QuoteTokenProgram
	if quoteProgram.IsZero() {
		quoteProgram = token.ProgramID
	}
	poolIx, err := s.initializeTransferHookPool(InitializePoolBaseParams{
		Name:        pool.Name,
		Symbol:      pool.Symbol,
		URI:         pool.URI,
		Pool:        poolAddress,
		Config:      params.Config,
		Payer:       params.Payer,
		PoolCreator: pool.PoolCreator,
		BaseMint:    pool.BaseMint,
		BaseVault:   helpers.DeriveDbcTokenVaultAddress(poolAddress, pool.BaseMint),
		QuoteVault:  helpers.DeriveDbcTokenVaultAddress(poolAddress, params.QuoteMint),
		QuoteMint:   params.QuoteMint,
		TokenBadge:  params.TokenBadge,
	}, params.TransferHookProgram, quoteProgram)
	if err != nil {
		return nil, err
	}
	return []solanago.Instruction{configIx, poolIx}, nil
}

func (s *DynamicBondingCurve) Swap2WithTransferHook(ctx context.Context, params Swap2Params) ([]solanago.Instruction, solanago.Instruction, []solanago.Instruction, error) {
	if params.TransferHook == nil {
		params.TransferHook = &TransferHookAccounts{}
	}
	return s.Swap2(ctx, params)
}

func (s *DynamicBondingCurve) GetPoolConfigWithKind(ctx context.Context, configAddress solanago.PublicKey) (*PoolConfigWithKind, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, configAddress, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("account not found")
	}
	data := acc.Value.Data.GetBinary()
	if parsed, err := dbcidl.ParseAccount_PoolConfig(data); err == nil {
		return &PoolConfigWithKind{PoolConfig: parsed}, nil
	}
	hook, err := dbcidl.ParseAccount_ConfigWithTransferHook(data)
	if err != nil {
		return nil, err
	}
	return &PoolConfigWithKind{PoolConfig: &hook.Config, TransferHookConfig: hook}, nil
}

func (s *DynamicBondingCurve) GetConfigWithTransferHook(ctx context.Context, configAddress solanago.PublicKey) (*dbcidl.ConfigWithTransferHook, error) {
	found, err := s.GetPoolConfigWithKind(ctx, configAddress)
	if err != nil {
		return nil, err
	}
	return found.TransferHookConfig, nil
}

func (s *DynamicBondingCurve) IsTransferHookConfig(ctx context.Context, configAddress solanago.PublicKey) (bool, error) {
	found, err := s.GetPoolConfigWithKind(ctx, configAddress)
	if err != nil {
		return false, err
	}
	return found.TransferHookConfig != nil, nil
}

func (s *DynamicBondingCurve) GetPoolWithKind(ctx context.Context, poolAddress solanago.PublicKey) (*PoolWithKind, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, poolAddress, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("account not found")
	}
	data := acc.Value.Data.GetBinary()
	if parsed, err := dbcidl.ParseAccount_VirtualPool(data); err == nil {
		return &PoolWithKind{Pool: parsed}, nil
	}
	hook, err := dbcidl.ParseAccount_TransferHookPool(data)
	if err != nil {
		return nil, err
	}
	return &PoolWithKind{Pool: &VirtualPool{PoolState: hook.PoolState}, TransferHook: true}, nil
}

func (s *DynamicBondingCurve) IsTransferHookPool(ctx context.Context, poolAddress solanago.PublicKey) (bool, error) {
	found, err := s.GetPoolWithKind(ctx, poolAddress)
	if err != nil {
		return false, err
	}
	return found.TransferHook, nil
}

func (s *DynamicBondingCurve) GetTokenBadge(ctx context.Context, tokenMint solanago.PublicKey) (*dbcidl.TokenBadge, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, helpers.DeriveTokenBadgeAddress(tokenMint), &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, nil
	}
	return dbcidl.ParseAccount_TokenBadge(acc.Value.Data.GetBinary())
}

func (s *DynamicBondingCurve) GetPoolSurplus(ctx context.Context, poolAddress solanago.PublicKey) (PoolSurplus, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return PoolSurplus{}, err
	}
	config, err := s.GetPoolConfig(ctx, pool.PoolState.Config)
	if err != nil {
		return PoolSurplus{}, err
	}
	return PoolSurplus{
		SurplusBreakdown: helpers.GetSurplusBreakdown(pool.PoolState.QuoteReserve, config.MigrationQuoteThreshold, config.CreatorTradingFeePercentage),
		PartnerWithdrawn: pool.PoolState.IsPartnerWithdrawSurplus == 1,
		CreatorWithdrawn: pool.PoolState.IsCreatorWithdrawSurplus == 1,
	}, nil
}

func (s *DynamicBondingCurve) GetPoolFeeMetrics(ctx context.Context, poolAddress solanago.PublicKey) (PoolFeeMetrics, error) {
	pool, err := s.GetPool(ctx, poolAddress)
	if err != nil {
		return PoolFeeMetrics{}, err
	}
	return PoolFeeMetrics{
		PartnerBaseFee:       pool.PoolState.PartnerBaseFee,
		PartnerQuoteFee:      pool.PoolState.PartnerQuoteFee,
		CreatorBaseFee:       pool.PoolState.CreatorBaseFee,
		CreatorQuoteFee:      pool.PoolState.CreatorQuoteFee,
		TotalTradingBaseFee:  pool.PoolState.Metrics.TotalTradingBaseFee,
		TotalTradingQuoteFee: pool.PoolState.Metrics.TotalTradingQuoteFee,
	}, nil
}

func buildSwap2Instruction(params Swap2Params, amount0, amount1 uint64, poolAuthority, config, ataIn, ataOut, baseVault, quoteVault, baseMint, quoteMint, inputProgram, outputProgram, eventAuthority solanago.PublicKey) (solanago.Instruction, error) {
	baseProgram := outputProgram
	quoteProgram := inputProgram
	if params.SwapBaseForQuote {
		baseProgram = inputProgram
		quoteProgram = outputProgram
	}
	swapParams := dbcidl.SwapParameters2{Amount0: amount0, Amount1: amount1, SwapMode: uint8(params.SwapMode)}
	if params.TransferHook == nil {
		return dbcidl.NewSwap2Instruction(swapParams, poolAuthority, config, params.Pool, ataIn, ataOut, baseVault, quoteVault, baseMint, quoteMint, params.Owner, baseProgram, quoteProgram, optionalAccount(params.ReferralTokenAccount), eventAuthority, helpers.DynamicBondingCurveProgramID)
	}
	info, err := transferHookInfo(params.TransferHook)
	if err != nil {
		return nil, err
	}
	return dbcidl.NewSwap2WithTransferHookInstruction(swapParams, info, poolAuthority, config, params.Pool, ataIn, ataOut, baseVault, quoteVault, baseMint, quoteMint, params.Owner, baseProgram, quoteProgram, optionalAccount(params.ReferralTokenAccount), eventAuthority, helpers.DynamicBondingCurveProgramID)
}

func claimPartnerTradingFeeInstruction(maxBase, maxQuote uint64, hook *TransferHookAccounts, poolAuthority, config, pool, tokenA, tokenB, baseVault, quoteVault, baseMint, quoteMint, feeClaimer, tokenBaseProgram, tokenQuoteProgram, eventAuthority, program solanago.PublicKey) (solanago.Instruction, error) {
	if hook == nil {
		return dbcidl.NewClaimTradingFeeInstruction(maxBase, maxQuote, poolAuthority, config, pool, tokenA, tokenB, baseVault, quoteVault, baseMint, quoteMint, feeClaimer, tokenBaseProgram, tokenQuoteProgram, eventAuthority, program)
	}
	info, err := transferHookInfo(hook)
	if err != nil {
		return nil, err
	}
	ix, err := dbcidl.NewClaimTradingFee2Instruction(maxBase, maxQuote, info, poolAuthority, config, pool, tokenA, tokenB, baseVault, quoteVault, baseMint, quoteMint, feeClaimer, tokenBaseProgram, tokenQuoteProgram, eventAuthority, program)
	if err != nil {
		return nil, err
	}
	appendMetas(ix, hook.Base)
	appendMetas(ix, hook.Referral)
	return ix, nil
}

func (s *DynamicBondingCurve) configValidation(ctx context.Context, params CreateConfigParams, opt helpers.ConfigValidationOptions, allowQuoteTransferFee bool) (helpers.ConfigValidationOptions, *solanago.PublicKey, error) {
	if s.RPC == nil {
		return opt, nil, nil
	}
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, params.QuoteMint, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return opt, nil, err
	}
	if acc == nil || acc.Value == nil {
		return opt, nil, fmt.Errorf("quote mint not found")
	}
	epoch, err := s.RPC.GetEpochInfo(ctx, s.Commitment)
	if err != nil {
		return opt, nil, err
	}
	data := acc.Value.Data.GetBinary()
	hasFee, err := helpers.HasTransferFeeOrConfigAuthority(data, epoch.Epoch)
	if err != nil {
		return opt, nil, err
	}
	if opt.IsTransferHook && hasFee {
		return opt, nil, errors.New("quote mint has a non-zero transfer fee or a live transfer fee config authority")
	}
	fee, err := helpers.EpochTransferFeeFromMint(data, epoch.Epoch)
	if err != nil {
		return opt, nil, err
	}
	opt.QuoteMintHasTransferFee = hasFee
	opt.QuoteEpochTransferFee = fee
	if opt.IsTransferHook && opt.TransferHookProgram != nil {
		ok, err := helpers.ValidateTransferHookProgramExecutable(ctx, s.RPC, *opt.TransferHookProgram)
		if err != nil {
			return opt, nil, err
		}
		if !ok {
			return opt, nil, fmt.Errorf("transfer hook program is not an executable account")
		}
	}
	supported, err := helpers.IsSupportedQuoteMint(params.QuoteMint, data, epoch.Epoch)
	if err != nil {
		return opt, nil, err
	}
	if supported {
		return opt, nil, nil
	}
	if !allowQuoteTransferFee && hasFee {
		return opt, nil, errors.New("quote mint has a non-zero transfer fee or a live transfer fee config authority")
	}
	expected := helpers.DeriveTokenBadgeAddress(params.QuoteMint)
	if params.TokenBadge != nil && !params.TokenBadge.Equals(expected) {
		return opt, nil, fmt.Errorf("invalid token badge for quote mint")
	}
	badgeAcc, err := s.RPC.GetAccountInfoWithOpts(ctx, expected, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil || badgeAcc == nil || badgeAcc.Value == nil || !badgeAcc.Value.Owner.Equals(helpers.DynamicBondingCurveProgramID) {
		return opt, nil, fmt.Errorf("quote mint requires an initialized token badge")
	}
	return opt, &expected, nil
}

func claimCreatorTradingFeeInstruction(maxBase, maxQuote uint64, hook *TransferHookAccounts, poolAuthority, pool, tokenA, tokenB, baseVault, quoteVault, baseMint, quoteMint, creator, tokenBaseProgram, tokenQuoteProgram, eventAuthority, program solanago.PublicKey) (solanago.Instruction, error) {
	if hook == nil {
		return dbcidl.NewClaimCreatorTradingFeeInstruction(maxBase, maxQuote, poolAuthority, pool, tokenA, tokenB, baseVault, quoteVault, baseMint, quoteMint, creator, tokenBaseProgram, tokenQuoteProgram, eventAuthority, program)
	}
	info, err := transferHookInfo(hook)
	if err != nil {
		return nil, err
	}
	ix, err := dbcidl.NewClaimCreatorTradingFee2Instruction(maxBase, maxQuote, info, poolAuthority, pool, tokenA, tokenB, baseVault, quoteVault, baseMint, quoteMint, creator, tokenBaseProgram, tokenQuoteProgram, eventAuthority, program)
	if err != nil {
		return nil, err
	}
	appendMetas(ix, hook.Base)
	appendMetas(ix, hook.Referral)
	return ix, nil
}
