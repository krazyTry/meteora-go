package dynamic_bonding_curve

import (
	"context"
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcmath "github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/programs/token"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

// TransferHookTransfer is one base-token movement whose extra accounts form one slice.
type TransferHookTransfer struct {
	AccountsType dbcidl.AccountsType
}

// GetRemainingAccountsForTransferHook reads the mint's transfer hook and its literal extra accounts.
func (s *DynamicBondingCurve) GetRemainingAccountsForTransferHook(ctx context.Context, mint solanago.PublicKey, transfers []TransferHookTransfer) (*TransferHookAccounts, error) {
	accounts, err := s.transferHookExtraAccounts(ctx, mint)
	if err != nil {
		return nil, err
	}
	out := &TransferHookAccounts{}
	for _, transfer := range transfers {
		copied := cloneMetas(accounts)
		if transfer.AccountsType == dbcidl.AccountsType_TransferHookBaseReferral {
			out.Referral = append(out.Referral, copied...)
			continue
		}
		out.Base = append(out.Base, copied...)
	}
	return out, nil
}

func (s *DynamicBondingCurve) transferHookExtraAccounts(ctx context.Context, mint solanago.PublicKey) ([]*solanago.AccountMeta, error) {
	acc, err := s.RPC.GetAccountInfoWithOpts(ctx, mint, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		if errors.Is(err, rpc.ErrNotFound) {
			return nil, errMintNotFound
		}
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, errMintNotFound
	}
	if acc.Value.Owner.Equals(token.ProgramID) {
		return nil, nil
	}
	program, ok, err := helpers.TransferHookProgram(acc.Value.Data.GetBinary())
	if err != nil || !ok {
		return nil, err
	}
	metaList := helpers.DeriveExtraAccountMetaAddress(mint, program)
	listAcc, err := s.RPC.GetAccountInfoWithOpts(ctx, metaList, &rpc.GetAccountInfoOpts{Commitment: s.Commitment})
	if err != nil {
		return nil, err
	}
	if listAcc == nil || listAcc.Value == nil {
		return nil, errors.New("transfer hook extra account metas not found")
	}
	resolved, err := helpers.ParseExtraAccountMetas(listAcc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	return append([]*solanago.AccountMeta{solanago.NewAccountMeta(metaList, false, false)}, resolved...), nil
}

var errMintNotFound = errors.New("mint not found")

func cloneMetas(in []*solanago.AccountMeta) []*solanago.AccountMeta {
	out := make([]*solanago.AccountMeta, len(in))
	copy(out, in)
	return out
}

func (s *DynamicBondingCurve) resolveFirstBuyHook(ctx context.Context, baseMint solanago.PublicKey, given *TransferHookAccounts, referral *solanago.PublicKey, collectFeeMode uint8) (*TransferHookAccounts, error) {
	if given != nil {
		return given, nil
	}
	transfers := []TransferHookTransfer{{AccountsType: dbcidl.AccountsType_TransferHookBase}}
	if referral != nil {
		mode, err := dbcmath.GetFeeMode(CollectFeeMode(collectFeeMode), TradeDirectionQuoteToBase, true)
		if err != nil {
			return nil, err
		}
		if mode.FeesOnBaseToken {
			transfers = append(transfers, TransferHookTransfer{AccountsType: dbcidl.AccountsType_TransferHookBaseReferral})
		}
	}
	hook, err := s.GetRemainingAccountsForTransferHook(ctx, baseMint, transfers)
	if err != nil {
		if errors.Is(err, errMintNotFound) {
			return &TransferHookAccounts{}, nil
		}
		return nil, err
	}
	return hook, nil
}

func (s *DynamicBondingCurve) swap2FirstBuyIx(ctx context.Context, buy FirstBuyWithTransferHookParams, baseMint, config, quoteMint solanago.PublicKey, baseFee baseFeeLike, activationType ActivationType, collectFeeMode uint8, enableFirstSwapWithMinFee bool) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	pre, ix, post, err = s.SwapBuyIx(ctx, buy.FirstBuyParams, baseMint, config, baseFee, false, activationType, TokenTypeToken2022, quoteMint, enableFirstSwapWithMinFee)
	if err != nil || ix == nil {
		return
	}
	hook, err := s.resolveFirstBuyHook(ctx, baseMint, buy.TransferHook, buy.ReferralTokenAccount, collectFeeMode)
	if err != nil {
		return nil, nil, nil, err
	}
	accounts := ix.Accounts()
	info, err := transferHookInfo(hook)
	if err != nil {
		return nil, nil, nil, err
	}
	// Rebuild as swap2_with_transfer_hook using the accounts SwapBuyIx already resolved.
	// Account order matches NewSwapInstruction: authority, config, pool, input, output, base vault, quote vault, base mint, quote mint, payer, base program, quote program, referral, event authority, program.
	if len(accounts) < 15 {
		return nil, nil, nil, errors.New("swap accounts are incomplete")
	}
	swapIx, err := dbcidl.NewSwap2WithTransferHookInstruction(
		dbcidl.SwapParameters2{Amount0: buy.BuyAmount.Uint64(), Amount1: buy.MinimumAmountOut.Uint64(), SwapMode: uint8(SwapModeExactIn)},
		info,
		accounts[0].PublicKey,
		accounts[1].PublicKey,
		accounts[2].PublicKey,
		accounts[3].PublicKey,
		accounts[4].PublicKey,
		accounts[5].PublicKey,
		accounts[6].PublicKey,
		accounts[7].PublicKey,
		accounts[8].PublicKey,
		accounts[9].PublicKey,
		accounts[10].PublicKey,
		accounts[11].PublicKey,
		accounts[12].PublicKey,
		accounts[13].PublicKey,
		accounts[14].PublicKey,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	rateLimited, err := RateLimiterApplied(ctx, s.RPC, s.Commitment, baseFee, activationType, big.NewInt(0), false)
	if err != nil {
		return nil, nil, nil, err
	}
	if enableFirstSwapWithMinFee || rateLimited {
		appendMetas(swapIx, []*solanago.AccountMeta{solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false)})
	}
	appendMetas(swapIx, hook.Base)
	appendMetas(swapIx, hook.Referral)
	return pre, swapIx, post, nil
}

func (s *DynamicBondingCurve) CreatePoolWithFirstBuyWithTransferHook(ctx context.Context, params CreatePoolWithFirstBuyWithTransferHookParams) (CreatePoolWithFirstBuyResult, error) {
	poolIx, quoteMint, cfg, err := s.transferHookPoolIx(ctx, params.CreatePoolParam, params.TransferHookProgram)
	if err != nil {
		return CreatePoolWithFirstBuyResult{}, err
	}
	res := CreatePoolWithFirstBuyResult{CreatePoolIx: poolIx}
	if params.FirstBuyParam != nil && params.FirstBuyParam.BuyAmount != nil && params.FirstBuyParam.BuyAmount.Sign() > 0 {
		pre, ix, post, err := s.swap2FirstBuyIx(ctx, *params.FirstBuyParam, params.CreatePoolParam.BaseMint, params.CreatePoolParam.Config, quoteMint, baseFeeFromConfig(cfg.PoolFees.BaseFee), ActivationType(cfg.ActivationType), cfg.CollectFeeMode, cfg.EnableFirstSwapWithMinFee == 1)
		if err != nil {
			return res, err
		}
		res.SwapPre, res.SwapIx, res.SwapPost = pre, ix, post
	}
	return res, nil
}

func (s *DynamicBondingCurve) CreatePoolWithPartnerAndCreatorFirstBuyWithTransferHook(ctx context.Context, params CreatePoolWithPartnerAndCreatorFirstBuyWithTransferHookParams) (CreatePoolWithPartnerAndCreatorFirstBuyResult, error) {
	poolIx, quoteMint, cfg, err := s.transferHookPoolIx(ctx, params.CreatePoolParam, params.TransferHookProgram)
	if err != nil {
		return CreatePoolWithPartnerAndCreatorFirstBuyResult{}, err
	}
	res := CreatePoolWithPartnerAndCreatorFirstBuyResult{CreatePoolIx: poolIx}
	if params.PartnerFirstBuyParam != nil && params.PartnerFirstBuyParam.BuyAmount != nil && params.PartnerFirstBuyParam.BuyAmount.Sign() > 0 {
		pre, ix, post, err := s.swap2FirstBuyIx(ctx, firstBuyFromPartner(*params.PartnerFirstBuyParam), params.CreatePoolParam.BaseMint, params.CreatePoolParam.Config, quoteMint, baseFeeFromConfig(cfg.PoolFees.BaseFee), ActivationType(cfg.ActivationType), cfg.CollectFeeMode, cfg.EnableFirstSwapWithMinFee == 1)
		if err != nil {
			return res, err
		}
		res.PartnerSwapPre, res.PartnerSwapIx, res.PartnerSwapPost = pre, ix, post
	}
	if params.CreatorFirstBuyParam != nil && params.CreatorFirstBuyParam.BuyAmount != nil && params.CreatorFirstBuyParam.BuyAmount.Sign() > 0 {
		pre, ix, post, err := s.swap2FirstBuyIx(ctx, firstBuyFromCreator(*params.CreatorFirstBuyParam), params.CreatePoolParam.BaseMint, params.CreatePoolParam.Config, quoteMint, baseFeeFromConfig(cfg.PoolFees.BaseFee), ActivationType(cfg.ActivationType), cfg.CollectFeeMode, cfg.EnableFirstSwapWithMinFee == 1)
		if err != nil {
			return res, err
		}
		res.CreatorSwapPre, res.CreatorSwapIx, res.CreatorSwapPost = pre, ix, post
	}
	return res, nil
}

func (s *DynamicBondingCurve) CreateConfigAndPoolWithFirstBuyWithTransferHook(ctx context.Context, params CreateConfigAndPoolWithFirstBuyWithTransferHookParams) (CreateConfigAndPoolWithFirstBuyResult, error) {
	configIx, err := s.CreateConfigWithTransferHook(ctx, params.CreateConfigWithTransferHookParams)
	if err != nil {
		return CreateConfigAndPoolWithFirstBuyResult{}, err
	}
	poolAddress := helpers.DeriveDbcPoolAddress(params.QuoteMint, params.PreCreatePoolParam.BaseMint, params.Config)
	quoteProgram := params.QuoteTokenProgram
	if quoteProgram.IsZero() {
		quoteProgram = token.ProgramID
	}
	poolIx, err := s.initializeTransferHookPool(InitializePoolBaseParams{
		Name: params.PreCreatePoolParam.Name, Symbol: params.PreCreatePoolParam.Symbol, URI: params.PreCreatePoolParam.URI,
		Pool: poolAddress, Config: params.Config, Payer: params.Payer, PoolCreator: params.PreCreatePoolParam.PoolCreator,
		BaseMint: params.PreCreatePoolParam.BaseMint, BaseVault: helpers.DeriveDbcTokenVaultAddress(poolAddress, params.PreCreatePoolParam.BaseMint),
		QuoteVault: helpers.DeriveDbcTokenVaultAddress(poolAddress, params.QuoteMint), QuoteMint: params.QuoteMint, TokenBadge: params.TokenBadge,
	}, params.TransferHookProgram, quoteProgram)
	if err != nil {
		return CreateConfigAndPoolWithFirstBuyResult{}, err
	}
	res := CreateConfigAndPoolWithFirstBuyResult{CreateConfigIx: configIx, CreatePoolIx: poolIx}
	if params.FirstBuyParam != nil && params.FirstBuyParam.BuyAmount != nil && params.FirstBuyParam.BuyAmount.Sign() > 0 {
		fee := baseFeeFromParams(params.PoolFees.BaseFee)
		pre, ix, post, err := s.swap2FirstBuyIx(ctx, *params.FirstBuyParam, params.PreCreatePoolParam.BaseMint, params.Config, params.QuoteMint, fee, ActivationType(params.ActivationType), params.CollectFeeMode, params.EnableFirstSwapWithMinFee)
		if err != nil {
			return res, err
		}
		res.SwapPre, res.SwapIx, res.SwapPost = pre, ix, post
	}
	return res, nil
}

func (s *DynamicBondingCurve) transferHookPoolIx(ctx context.Context, params CreatePoolParams, transferHookProgram solanago.PublicKey) (solanago.Instruction, solanago.PublicKey, *PoolConfig, error) {
	ix, err := s.CreatePoolWithTransferHook(ctx, params, transferHookProgram)
	if err != nil {
		return nil, solanago.PublicKey{}, nil, err
	}
	found, err := s.GetPoolConfigWithKind(ctx, params.Config)
	if err != nil {
		return nil, solanago.PublicKey{}, nil, err
	}
	return ix, found.PoolConfig.QuoteMint, found.PoolConfig, nil
}

func firstBuyFromPartner(params PartnerFirstBuyWithTransferHookParams) FirstBuyWithTransferHookParams {
	receiver := params.Receiver
	return FirstBuyWithTransferHookParams{
		FirstBuyParams: FirstBuyParams{Buyer: params.Partner, Receiver: &receiver, BuyAmount: params.BuyAmount, MinimumAmountOut: params.MinimumAmountOut, ReferralTokenAccount: params.ReferralTokenAccount},
		TransferHook:   params.TransferHook,
	}
}

func firstBuyFromCreator(params CreatorFirstBuyWithTransferHookParams) FirstBuyWithTransferHookParams {
	receiver := params.Receiver
	return FirstBuyWithTransferHookParams{
		FirstBuyParams: FirstBuyParams{Buyer: params.Creator, Receiver: &receiver, BuyAmount: params.BuyAmount, MinimumAmountOut: params.MinimumAmountOut, ReferralTokenAccount: params.ReferralTokenAccount},
		TransferHook:   params.TransferHook,
	}
}
