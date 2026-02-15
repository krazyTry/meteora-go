package dynamic_bonding_curve

import (
	"context"
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/math/pool_fees"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type PoolService struct {
	*DynamicBondingCurveProgram
	State *StateService
}

type baseFeeLike struct {
	BaseFeeMode  uint8
	FirstFactor  uint16
	SecondFactor uint64
	ThirdFactor  uint64
}

func baseFeeFromParams(p BaseFeeParameters) baseFeeLike {
	return baseFeeLike{BaseFeeMode: p.BaseFeeMode, FirstFactor: p.FirstFactor, SecondFactor: p.SecondFactor, ThirdFactor: p.ThirdFactor}
}

func baseFeeFromConfig(p BaseFeeConfig) baseFeeLike {
	return baseFeeLike{BaseFeeMode: p.BaseFeeMode, FirstFactor: p.FirstFactor, SecondFactor: p.SecondFactor, ThirdFactor: p.ThirdFactor}
}

func NewPoolService(rpcClient *rpc.Client, commitment rpc.CommitmentType) *PoolService {
	return &PoolService{
		DynamicBondingCurveProgram: NewDynamicBondingCurveProgram(rpcClient, commitment),
		State:                      NewStateService(rpcClient, commitment),
	}
}

func (s *PoolService) initializeSplPool(params InitializePoolBaseParams) (solanago.Instruction, error) {
	p := InitializePoolParameters{Name: params.Name, Symbol: params.Symbol, Uri: params.URI}
	mintMetadata := params.MintMetadata
	if mintMetadata == nil {
		return nil, errors.New("mint metadata required for SPL pool")
	}
	return dbcidl.NewInitializeVirtualPoolWithSplTokenInstruction(
		p,
		params.Config,
		s.PoolAuthority,
		params.PoolCreator,
		params.BaseMint,
		params.QuoteMint,
		params.Pool,
		params.BaseVault,
		params.QuoteVault,
		*mintMetadata,
		MetaplexProgramID,
		params.Payer,
		token.ProgramID,
		token.ProgramID,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
}

func (s *PoolService) initializeToken2022Pool(params InitializePoolBaseParams) (solanago.Instruction, error) {
	p := InitializePoolParameters{Name: params.Name, Symbol: params.Symbol, Uri: params.URI}
	return dbcidl.NewInitializeVirtualPoolWithToken2022Instruction(
		p,
		params.Config,
		s.PoolAuthority,
		params.PoolCreator,
		params.BaseMint,
		params.QuoteMint,
		params.Pool,
		params.BaseVault,
		params.QuoteVault,
		params.Payer,
		token.ProgramID,
		solanago.Token2022ProgramID,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
}

func (s *PoolService) CreateConfigIx(params CreateConfigParams) (solanago.Instruction, error) {
	if err := helpers.ValidateConfigParameters(params); err != nil {
		return nil, err
	}
	return dbcidl.NewCreateConfigInstruction(
		params.ConfigParameters,
		params.Config,
		params.FeeClaimer,
		params.LeftoverReceiver,
		params.QuoteMint,
		params.Payer,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
}

func (s *PoolService) CreatePoolIx(createPoolParam CreatePoolParams, tokenType TokenType, quoteMint solanago.PublicKey) (solanago.Instruction, error) {
	pool := helpers.DeriveDbcPoolAddress(quoteMint, createPoolParam.BaseMint, createPoolParam.Config)
	baseVault := helpers.DeriveDbcTokenVaultAddress(pool, createPoolParam.BaseMint)
	quoteVault := helpers.DeriveDbcTokenVaultAddress(pool, quoteMint)

	baseParams := InitializePoolBaseParams{
		Name:        createPoolParam.Name,
		Symbol:      createPoolParam.Symbol,
		URI:         createPoolParam.URI,
		Pool:        pool,
		Config:      createPoolParam.Config,
		Payer:       createPoolParam.Payer,
		PoolCreator: createPoolParam.PoolCreator,
		BaseMint:    createPoolParam.BaseMint,
		BaseVault:   baseVault,
		QuoteVault:  quoteVault,
		QuoteMint:   quoteMint,
	}
	if tokenType == TokenTypeSPL {
		mintMetadata := helpers.DeriveMintMetadata(createPoolParam.BaseMint)
		baseParams.MintMetadata = &mintMetadata
		return s.initializeSplPool(baseParams)
	}
	return s.initializeToken2022Pool(baseParams)
}

func (s *PoolService) CreatePool(ctx context.Context, params CreatePoolParams) (solanago.Instruction, error) {
	poolConfigState, err := s.State.GetPoolConfig(ctx, params.Config)
	if err != nil {
		return nil, err
	}
	return s.CreatePoolIx(params, TokenType(poolConfigState.TokenType), poolConfigState.QuoteMint)
}

func (s *PoolService) CreateConfigAndPool(ctx context.Context, params CreateConfigAndPoolParams) ([]solanago.Instruction, error) {
	configIx, err := s.CreateConfigIx(CreateConfigParams{
		ConfigParameters: params.ConfigParameters,
		Config:           params.Config,
		FeeClaimer:       params.FeeClaimer,
		LeftoverReceiver: params.LeftoverReceiver,
		QuoteMint:        params.QuoteMint,
		Payer:            params.Payer,
	})
	if err != nil {
		return nil, err
	}
	createPoolIx, err := s.CreatePoolIx(CreatePoolParams{
		Name:        params.PreCreatePoolParam.Name,
		Symbol:      params.PreCreatePoolParam.Symbol,
		URI:         params.PreCreatePoolParam.URI,
		Payer:       params.Payer,
		PoolCreator: params.PreCreatePoolParam.PoolCreator,
		Config:      params.Config,
		BaseMint:    params.PreCreatePoolParam.BaseMint,
	}, TokenType(params.TokenType), params.QuoteMint)
	if err != nil {
		return nil, err
	}
	return []solanago.Instruction{configIx, createPoolIx}, nil
}

type CreateConfigAndPoolWithFirstBuyResult struct {
	CreateConfigIx solanago.Instruction
	CreatePoolIx   solanago.Instruction
	SwapPre        []solanago.Instruction
	SwapIx         solanago.Instruction
	SwapPost       []solanago.Instruction
}

func (s *PoolService) CreateConfigAndPoolWithFirstBuy(ctx context.Context, params CreateConfigAndPoolWithFirstBuyParams) (CreateConfigAndPoolWithFirstBuyResult, error) {
	configIx, err := s.CreateConfigIx(CreateConfigParams{
		ConfigParameters: params.ConfigParameters,
		Config:           params.Config,
		FeeClaimer:       params.FeeClaimer,
		LeftoverReceiver: params.LeftoverReceiver,
		QuoteMint:        params.QuoteMint,
		Payer:            params.Payer,
	})
	if err != nil {
		return CreateConfigAndPoolWithFirstBuyResult{}, err
	}
	createPoolIx, err := s.CreatePoolIx(CreatePoolParams{
		Name:        params.PreCreatePoolParam.Name,
		Symbol:      params.PreCreatePoolParam.Symbol,
		URI:         params.PreCreatePoolParam.URI,
		Payer:       params.Payer,
		PoolCreator: params.PreCreatePoolParam.PoolCreator,
		Config:      params.Config,
		BaseMint:    params.PreCreatePoolParam.BaseMint,
	}, TokenType(params.TokenType), params.QuoteMint)
	if err != nil {
		return CreateConfigAndPoolWithFirstBuyResult{}, err
	}
	var pre []solanago.Instruction
	var swapIx solanago.Instruction
	var post []solanago.Instruction
	if params.FirstBuyParam != nil && params.FirstBuyParam.BuyAmount.Sign() > 0 {
		pre, swapIx, post, err = s.SwapBuyIx(ctx, *params.FirstBuyParam, params.PreCreatePoolParam.BaseMint, params.Config, baseFeeFromParams(params.ConfigParameters.PoolFees.BaseFee), false, ActivationType(params.ActivationType), TokenType(params.TokenType), params.QuoteMint, true)
		if err != nil {
			return CreateConfigAndPoolWithFirstBuyResult{}, err
		}
	}
	return CreateConfigAndPoolWithFirstBuyResult{
		CreateConfigIx: configIx,
		CreatePoolIx:   createPoolIx,
		SwapPre:        pre,
		SwapIx:         swapIx,
		SwapPost:       post,
	}, nil
}

type CreatePoolWithFirstBuyResult struct {
	CreatePoolIx solanago.Instruction
	SwapPre      []solanago.Instruction
	SwapIx       solanago.Instruction
	SwapPost     []solanago.Instruction
}

func (s *PoolService) CreatePoolWithFirstBuy(ctx context.Context, params CreatePoolWithFirstBuyParams) (CreatePoolWithFirstBuyResult, error) {
	poolConfigState, err := s.State.GetPoolConfig(ctx, params.CreatePoolParam.Config)
	if err != nil {
		return CreatePoolWithFirstBuyResult{}, err
	}
	createPoolIx, err := s.CreatePoolIx(params.CreatePoolParam, TokenType(poolConfigState.TokenType), poolConfigState.QuoteMint)
	if err != nil {
		return CreatePoolWithFirstBuyResult{}, err
	}
	var pre []solanago.Instruction
	var swapIx solanago.Instruction
	var post []solanago.Instruction
	if params.FirstBuyParam != nil && params.FirstBuyParam.BuyAmount.Sign() > 0 {
		pre, swapIx, post, err = s.SwapBuyIx(ctx, *params.FirstBuyParam, params.CreatePoolParam.BaseMint, params.CreatePoolParam.Config, baseFeeFromConfig(poolConfigState.PoolFees.BaseFee), false, ActivationType(poolConfigState.ActivationType), TokenType(poolConfigState.TokenType), poolConfigState.QuoteMint, true)
		if err != nil {
			return CreatePoolWithFirstBuyResult{}, err
		}
	}
	return CreatePoolWithFirstBuyResult{CreatePoolIx: createPoolIx, SwapPre: pre, SwapIx: swapIx, SwapPost: post}, nil
}

type CreatePoolWithPartnerAndCreatorFirstBuyResult struct {
	CreatePoolIx    solanago.Instruction
	PartnerSwapPre  []solanago.Instruction
	PartnerSwapIx   solanago.Instruction
	PartnerSwapPost []solanago.Instruction
	CreatorSwapPre  []solanago.Instruction
	CreatorSwapIx   solanago.Instruction
	CreatorSwapPost []solanago.Instruction
}

func (s *PoolService) CreatePoolWithPartnerAndCreatorFirstBuy(ctx context.Context, params CreatePoolWithPartnerAndCreatorFirstBuyParams) (CreatePoolWithPartnerAndCreatorFirstBuyResult, error) {
	poolConfigState, err := s.State.GetPoolConfig(ctx, params.CreatePoolParam.Config)
	if err != nil {
		return CreatePoolWithPartnerAndCreatorFirstBuyResult{}, err
	}
	createPoolIx, err := s.CreatePoolIx(params.CreatePoolParam, TokenType(poolConfigState.TokenType), poolConfigState.QuoteMint)
	if err != nil {
		return CreatePoolWithPartnerAndCreatorFirstBuyResult{}, err
	}
	res := CreatePoolWithPartnerAndCreatorFirstBuyResult{CreatePoolIx: createPoolIx}
	if params.PartnerFirstBuyParam != nil && params.PartnerFirstBuyParam.BuyAmount.Sign() > 0 {
		pre, ix, post, err := s.SwapBuyIx(ctx, FirstBuyParams{
			Buyer:                params.PartnerFirstBuyParam.Partner,
			Receiver:             &params.PartnerFirstBuyParam.Receiver,
			BuyAmount:            params.PartnerFirstBuyParam.BuyAmount,
			MinimumAmountOut:     params.PartnerFirstBuyParam.MinimumAmountOut,
			ReferralTokenAccount: params.PartnerFirstBuyParam.ReferralTokenAccount,
		}, params.CreatePoolParam.BaseMint, params.CreatePoolParam.Config, baseFeeFromConfig(poolConfigState.PoolFees.BaseFee), false, ActivationType(poolConfigState.ActivationType), TokenType(poolConfigState.TokenType), poolConfigState.QuoteMint, true)
		if err != nil {
			return res, err
		}
		res.PartnerSwapPre, res.PartnerSwapIx, res.PartnerSwapPost = pre, ix, post
	}
	if params.CreatorFirstBuyParam != nil && params.CreatorFirstBuyParam.BuyAmount.Sign() > 0 {
		pre, ix, post, err := s.SwapBuyIx(ctx, FirstBuyParams{
			Buyer:                params.CreatorFirstBuyParam.Creator,
			Receiver:             &params.CreatorFirstBuyParam.Receiver,
			BuyAmount:            params.CreatorFirstBuyParam.BuyAmount,
			MinimumAmountOut:     params.CreatorFirstBuyParam.MinimumAmountOut,
			ReferralTokenAccount: params.CreatorFirstBuyParam.ReferralTokenAccount,
		}, params.CreatePoolParam.BaseMint, params.CreatePoolParam.Config, baseFeeFromConfig(poolConfigState.PoolFees.BaseFee), false, ActivationType(poolConfigState.ActivationType), TokenType(poolConfigState.TokenType), poolConfigState.QuoteMint, true)
		if err != nil {
			return res, err
		}
		res.CreatorSwapPre, res.CreatorSwapIx, res.CreatorSwapPost = pre, ix, post
	}
	return res, nil
}

func (s *PoolService) SwapBuyIx(ctx context.Context, firstBuyParam FirstBuyParams, baseMint solanago.PublicKey, config solanago.PublicKey, baseFee baseFeeLike, swapBaseForQuote bool, activationType ActivationType, tokenType TokenType, quoteMint solanago.PublicKey, enableFirstSwapWithMinFee bool) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	if err = helpers.ValidateSwapAmount(firstBuyParam.BuyAmount); err != nil {
		return
	}
	// rate limiter check
	rateLimiterApplied := false
	if BaseFeeMode(baseFee.BaseFeeMode) == BaseFeeModeRateLimiter {
		currentPoint := big.NewInt(0)
		if activationType == ActivationTypeSlot {
			slot, _ := s.RPC.GetSlot(ctx, s.Commitment)
			currentPoint = new(big.Int).SetUint64(slot)
		} else {
			slot, _ := s.RPC.GetSlot(ctx, s.Commitment)
			bt, _ := s.RPC.GetBlockTime(ctx, slot)
			if bt != nil {
				currentPoint = big.NewInt(int64(*bt))
			}
		}
		rateLimiterApplied = pool_fees.IsRateLimiterApplied(currentPoint, big.NewInt(0), func() TradeDirection {
			if swapBaseForQuote {
				return TradeDirectionBaseToQuote
			}
			return TradeDirectionQuoteToBase
		}(), new(big.Int).SetUint64(baseFee.SecondFactor), new(big.Int).SetUint64(baseFee.ThirdFactor), new(big.Int).SetUint64(uint64(baseFee.FirstFactor)))
	}

	quoteTokenFlag, err := helpers.GetTokenType(ctx, s.RPC, quoteMint)
	if err != nil {
		return
	}
	inputMint := quoteMint
	outputMint := baseMint
	inputProgram := helpers.GetTokenProgram(quoteTokenFlag)
	outputProgram := helpers.GetTokenProgram(tokenType)

	pool := helpers.DeriveDbcPoolAddress(quoteMint, baseMint, config)
	baseVault := helpers.DeriveDbcTokenVaultAddress(pool, baseMint)
	quoteVault := helpers.DeriveDbcTokenVaultAddress(pool, quoteMint)

	pre = make([]solanago.Instruction, 0)
	post = make([]solanago.Instruction, 0)

	inputTokenAccount, ixA, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, inputMint, firstBuyParam.Buyer, firstBuyParam.Buyer, inputProgram)
	if err != nil {
		return
	}
	outputOwner := firstBuyParam.Buyer
	if firstBuyParam.Receiver != nil {
		outputOwner = *firstBuyParam.Receiver
	}
	outputTokenAccount, ixB, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, outputMint, outputOwner, firstBuyParam.Buyer, outputProgram)
	if err != nil {
		return
	}
	if ixA != nil {
		pre = append(pre, ixA)
	}
	if ixB != nil {
		pre = append(pre, ixB)
	}

	if inputMint.Equals(helpers.NativeMint) {
		wrapIx, werr := helpers.WrapSOLInstruction(firstBuyParam.Buyer, inputTokenAccount, firstBuyParam.BuyAmount.Uint64())
		if werr != nil {
			err = werr
			return
		}
		pre = append(pre, wrapIx...)
	}
	if inputMint.Equals(helpers.NativeMint) || outputMint.Equals(helpers.NativeMint) {
		unwrapIx, uerr := helpers.UnwrapSOLInstruction(firstBuyParam.Buyer, firstBuyParam.Buyer, true)
		if uerr == nil && unwrapIx != nil {
			post = append(post, unwrapIx)
		}
	}

	params := dbcidl.SwapParameters{AmountIn: firstBuyParam.BuyAmount.Uint64(), MinimumAmountOut: firstBuyParam.MinimumAmountOut.Uint64()}

	ix, err = dbcidl.NewSwapInstruction(
		params,
		s.PoolAuthority,
		config,
		pool,
		inputTokenAccount,
		outputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		firstBuyParam.Buyer,
		outputProgram,
		inputProgram,
		optionalPubkey(firstBuyParam.ReferralTokenAccount),
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
	if err != nil {
		return
	}
	if rateLimiterApplied || enableFirstSwapWithMinFee {
		if gi, ok := ix.(*solanago.GenericInstruction); ok {
			gi.AccountValues = append(gi.AccountValues, solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false))
		}
	}
	return
}

func (s *PoolService) Swap(ctx context.Context, params SwapParams) ([]solanago.Instruction, solanago.Instruction, []solanago.Instruction, error) {
	if err := helpers.ValidateSwapAmount(params.AmountIn); err != nil {
		return nil, nil, nil, err
	}
	poolState, err := s.State.GetPool(ctx, params.Pool)
	if err != nil {
		return nil, nil, nil, err
	}
	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	inputMint := poolState.BaseMint
	outputMint := poolConfigState.QuoteMint
	inputProgram := helpers.GetTokenProgram(TokenType(poolState.PoolType))
	outputProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))
	if !params.SwapBaseForQuote {
		inputMint = poolConfigState.QuoteMint
		outputMint = poolState.BaseMint
		inputProgram = helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))
		outputProgram = helpers.GetTokenProgram(TokenType(poolState.PoolType))
	}

	payer := payerOrOwner(params.Payer, params.Owner)
	ataIn, ataOut, pre, err := s.PrepareTokenAccounts(ctx, params.Owner, payer, inputMint, outputMint, inputProgram, outputProgram)
	if err != nil {
		return nil, nil, nil, err
	}

	if inputMint.Equals(helpers.NativeMint) {
		wrapIx, werr := helpers.WrapSOLInstruction(params.Owner, ataIn, params.AmountIn.Uint64())
		if werr != nil {
			return nil, nil, nil, werr
		}
		pre = append(pre, wrapIx...)
	}
	post := make([]solanago.Instruction, 0)
	if inputMint.Equals(helpers.NativeMint) || outputMint.Equals(helpers.NativeMint) {
		unwrapIx, uerr := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if uerr == nil && unwrapIx != nil {
			post = append(post, unwrapIx)
		}
	}

	swapIx, err := dbcidl.NewSwapInstruction(
		dbcidl.SwapParameters{
			AmountIn:         params.AmountIn.Uint64(),
			MinimumAmountOut: params.MinimumAmountOut.Uint64(),
		},
		s.PoolAuthority,
		poolState.Config,
		params.Pool,
		ataIn,
		ataOut,
		poolState.BaseVault,
		poolState.QuoteVault,
		poolState.BaseMint,
		poolConfigState.QuoteMint,
		params.Owner,
		func() solanago.PublicKey {
			if params.SwapBaseForQuote {
				return inputProgram
			}
			return outputProgram
		}(),
		func() solanago.PublicKey {
			if params.SwapBaseForQuote {
				return outputProgram
			}
			return inputProgram
		}(),
		optionalPubkey(params.ReferralTokenAccount),
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	// rate limiter remaining account
	rateLimiterApplied := false
	if BaseFeeMode(poolConfigState.PoolFees.BaseFee.BaseFeeMode) == BaseFeeModeRateLimiter {
		currentPoint := CurrentPointForActivation(ctx, s.RPC, s.Commitment, ActivationType(poolConfigState.ActivationType))
		rateLimiterApplied = pool_fees.IsRateLimiterApplied(currentPoint, new(big.Int).SetUint64(poolState.ActivationPoint), func() TradeDirection {
			if params.SwapBaseForQuote {
				return TradeDirectionBaseToQuote
			}
			return TradeDirectionQuoteToBase
		}(), new(big.Int).SetUint64(poolConfigState.PoolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolConfigState.PoolFees.BaseFee.ThirdFactor), new(big.Int).SetUint64(uint64(poolConfigState.PoolFees.BaseFee.FirstFactor)))
	}
	if rateLimiterApplied || poolConfigState.EnableFirstSwapWithMinFee == 1 {
		if gi, ok := swapIx.(*solanago.GenericInstruction); ok {
			gi.AccountValues = append(gi.AccountValues, solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false))
		}
	}

	return pre, swapIx, post, nil
}

func (s *PoolService) Swap2(ctx context.Context, params Swap2Params) ([]solanago.Instruction, solanago.Instruction, []solanago.Instruction, error) {
	poolState, err := s.State.GetPool(ctx, params.Pool)
	if err != nil {
		return nil, nil, nil, err
	}
	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	var amount0, amount1 uint64
	if params.SwapMode == SwapModeExactOut {
		amount0 = params.AmountOut.Uint64()
		amount1 = params.MaximumAmountIn.Uint64()
	} else {
		amount0 = params.AmountIn.Uint64()
		amount1 = params.MinimumAmountOut.Uint64()
	}
	if err := helpers.ValidateSwapAmount(new(big.Int).SetUint64(amount0)); err != nil {
		return nil, nil, nil, err
	}

	inputMint := poolState.BaseMint
	outputMint := poolConfigState.QuoteMint
	inputProgram := helpers.GetTokenProgram(TokenType(poolState.PoolType))
	outputProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))
	if !params.SwapBaseForQuote {
		inputMint = poolConfigState.QuoteMint
		outputMint = poolState.BaseMint
		inputProgram = helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))
		outputProgram = helpers.GetTokenProgram(TokenType(poolState.PoolType))
	}

	payer := payerOrOwner(params.Payer, params.Owner)
	ataIn, ataOut, pre, err := s.PrepareTokenAccounts(ctx, params.Owner, payer, inputMint, outputMint, inputProgram, outputProgram)
	if err != nil {
		return nil, nil, nil, err
	}

	if inputMint.Equals(helpers.NativeMint) {
		amount := amount0
		if params.SwapMode == SwapModeExactOut {
			amount = amount1
		}
		wrapIx, werr := helpers.WrapSOLInstruction(params.Owner, ataIn, amount)
		if werr != nil {
			return nil, nil, nil, werr
		}
		pre = append(pre, wrapIx...)
	}
	post := make([]solanago.Instruction, 0)
	if inputMint.Equals(helpers.NativeMint) || outputMint.Equals(helpers.NativeMint) {
		unwrapIx, uerr := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if uerr == nil && unwrapIx != nil {
			post = append(post, unwrapIx)
		}
	}

	swapIx, err := dbcidl.NewSwap2Instruction(
		dbcidl.SwapParameters2{
			Amount0:  amount0,
			Amount1:  amount1,
			SwapMode: uint8(params.SwapMode),
		},
		s.PoolAuthority,
		poolState.Config,
		params.Pool,
		ataIn,
		ataOut,
		poolState.BaseVault,
		poolState.QuoteVault,
		poolState.BaseMint,
		poolConfigState.QuoteMint,
		params.Owner,
		func() solanago.PublicKey {
			if params.SwapBaseForQuote {
				return inputProgram
			}
			return outputProgram
		}(),
		func() solanago.PublicKey {
			if params.SwapBaseForQuote {
				return outputProgram
			}
			return inputProgram
		}(),
		optionalPubkey(params.ReferralTokenAccount),
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	rateLimiterApplied := false
	if BaseFeeMode(poolConfigState.PoolFees.BaseFee.BaseFeeMode) == BaseFeeModeRateLimiter {
		currentPoint := CurrentPointForActivation(ctx, s.RPC, s.Commitment, ActivationType(poolConfigState.ActivationType))
		rateLimiterApplied = pool_fees.IsRateLimiterApplied(currentPoint, new(big.Int).SetUint64(poolState.ActivationPoint), func() TradeDirection {
			if params.SwapBaseForQuote {
				return TradeDirectionBaseToQuote
			}
			return TradeDirectionQuoteToBase
		}(), new(big.Int).SetUint64(poolConfigState.PoolFees.BaseFee.SecondFactor), new(big.Int).SetUint64(poolConfigState.PoolFees.BaseFee.ThirdFactor), new(big.Int).SetUint64(uint64(poolConfigState.PoolFees.BaseFee.FirstFactor)))
	}
	if rateLimiterApplied || poolConfigState.EnableFirstSwapWithMinFee == 1 {
		if gi, ok := swapIx.(*solanago.GenericInstruction); ok {
			gi.AccountValues = append(gi.AccountValues, solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false))
		}
	}
	return pre, swapIx, post, nil
}

// SwapQuote helpers
func (s *PoolService) SwapQuote(params SwapQuoteParams) (SwapQuoteResult, error) {
	return math.SwapQuote(params.VirtualPool, params.Config, params.SwapBaseForQuote, params.AmountIn, params.SlippageBps, params.HasReferral, params.CurrentPoint, params.EligibleForFirstSwapWithMinFee)
}

func (s *PoolService) SwapQuote2(params SwapQuote2Params) (SwapQuote2Result, error) {
	if params.SwapMode == SwapModeExactIn {
		return math.SwapQuoteExactIn(params.VirtualPool, params.Config, params.SwapBaseForQuote, params.AmountIn, params.SlippageBps, params.HasReferral, params.CurrentPoint, params.EligibleForFirstSwapWithMinFee)
	}
	if params.SwapMode == SwapModeExactOut {
		return math.SwapQuoteExactOut(params.VirtualPool, params.Config, params.SwapBaseForQuote, params.AmountOut, params.SlippageBps, params.HasReferral, params.CurrentPoint, params.EligibleForFirstSwapWithMinFee)
	}
	if params.SwapMode == SwapModePartialFill {
		return math.SwapQuotePartialFill(params.VirtualPool, params.Config, params.SwapBaseForQuote, params.AmountIn, params.SlippageBps, params.HasReferral, params.CurrentPoint, params.EligibleForFirstSwapWithMinFee)
	}
	return SwapQuote2Result{}, errors.New("Unsupported swap mode")
}

// helper to pick payer
func payerOrOwner(payer *solanago.PublicKey, owner solanago.PublicKey) solanago.PublicKey {
	if payer != nil {
		return *payer
	}
	return owner
}

func CurrentPointForActivation(ctx context.Context, client *rpc.Client, commitment rpc.CommitmentType, activationType ActivationType) *big.Int {
	slot, _ := client.GetSlot(ctx, commitment)
	if activationType == ActivationTypeSlot {
		return new(big.Int).SetUint64(slot)
	}
	bt, _ := client.GetBlockTime(ctx, slot)
	if bt != nil {
		return big.NewInt(int64(*bt))
	}
	return big.NewInt(0)
}

func optionalPubkey(pk *solanago.PublicKey) solanago.PublicKey {
	if pk == nil {
		return dbcidl.ProgramID
	}
	return *pk
}
