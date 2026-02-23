package dammv2

import (
	"context"
	"errors"
	"math/big"

	solanago "github.com/gagliardetto/solana-go"

	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/math"
	"github.com/krazyTry/meteora-go/damm_v2/math/pool_fees"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

// IsPoolExist checks whether a pool account exists.
func (c *CpAmm) IsPoolExist(ctx context.Context, pool solanago.PublicKey) (bool, error) {
	acc, err := c.Client.GetAccountInfoWithOpts(ctx, pool, nil)
	if err != nil {
		return false, err
	}
	return acc != nil && acc.Value != nil, nil
}

// GetLiquidityDelta computes liquidity delta for max token inputs.
func (c *CpAmm) GetLiquidityDelta(params LiquidityDeltaParams) *big.Int {
	liquidityFromA := math.GetLiquidityDeltaFromAmountA(params.MaxAmountTokenA, params.SqrtPrice, params.SqrtMaxPrice)
	liquidityFromB := math.GetLiquidityDeltaFromAmountB(params.MaxAmountTokenB, params.SqrtMinPrice, params.SqrtPrice)
	return minBig(liquidityFromA, liquidityFromB)
}

// GetQuote calculates swap quote based on exact input.
func (c *CpAmm) GetQuote(params GetQuoteParams) (QuoteResult, error) {
	poolState := params.PoolState
	aToB := poolState.TokenAMint.Equals(params.InputTokenMint)
	swapResult, err := math.SwapQuoteExactInput(
		poolState,
		params.CurrentPoint,
		params.InAmount,
		params.Slippage,
		aToB,
		params.HasReferral,
		params.TokenADecimal,
		params.TokenBDecimal,
		params.InputTokenInfo,
		params.OutputTokenInfo,
	)
	if err != nil {
		return QuoteResult{}, err
	}
	totalFee := new(big.Int).Add(new(big.Int).SetUint64(swapResult.TradingFee), new(big.Int).SetUint64(swapResult.ProtocolFee))
	totalFee.Add(totalFee, new(big.Int).SetUint64(swapResult.PartnerFee))
	totalFee.Add(totalFee, new(big.Int).SetUint64(swapResult.ReferralFee))
	return QuoteResult{
		SwapInAmount:     params.InAmount,
		ConsumedInAmount: new(big.Int).SetUint64(swapResult.IncludedFeeInputAmount),
		SwapOutAmount:    new(big.Int).SetUint64(swapResult.OutputAmount),
		MinSwapOutAmount: swapResult.MinimumAmountOut,
		TotalFee:         totalFee,
		PriceImpact:      swapResult.PriceImpact,
	}, nil
}

// GetQuote2 calculates quote for multiple swap modes.
func (c *CpAmm) GetQuote2(params GetQuote2Params) (Quote2Result, error) {
	aToB := params.PoolState.TokenAMint.Equals(params.InputTokenMint)
	switch params.SwapMode {
	case SwapModeExactIn:
		if params.AmountIn == nil {
			return Quote2Result{}, errors.New("amountIn is required for ExactIn swap mode")
		}
		return math.SwapQuoteExactInput(params.PoolState, params.CurrentPoint, params.AmountIn, params.Slippage, aToB, params.HasReferral, params.TokenADecimal, params.TokenBDecimal, params.InputTokenInfo, params.OutputTokenInfo)
	case SwapModeExactOut:
		if params.AmountOut == nil {
			return Quote2Result{}, errors.New("amountOut is required for ExactOut swap mode")
		}
		return math.SwapQuoteExactOutput(params.PoolState, params.CurrentPoint, params.AmountOut, params.Slippage, aToB, params.HasReferral, params.TokenADecimal, params.TokenBDecimal, params.InputTokenInfo, params.OutputTokenInfo)
	case SwapModePartialFill:
		if params.AmountIn == nil {
			return Quote2Result{}, errors.New("amountIn is required for PartialFill swap mode")
		}
		return math.SwapQuotePartialInput(params.PoolState, params.CurrentPoint, params.AmountIn, params.Slippage, aToB, params.HasReferral, params.TokenADecimal, params.TokenBDecimal, params.InputTokenInfo, params.OutputTokenInfo)
	default:
		return Quote2Result{}, errors.New("unsupported swap mode")
	}
}

// GetDepositQuote calculates the deposit quote.
func (c *CpAmm) GetDepositQuote(params GetDepositQuoteParams) DepositQuote {
	actualAmountIn := params.InAmount
	if params.InputTokenInfo != nil {
		actualAmountIn = helpers.CalculateTransferFeeExcludedAmount(params.InAmount, params.InputTokenInfo).Amount
	}
	var liquidityDelta *big.Int
	var rawOutputAmount *big.Int
	if params.IsTokenA {
		liquidityDelta = math.GetLiquidityDeltaFromAmountA(actualAmountIn, params.SqrtPrice, params.MaxSqrtPrice)
		rawOutputAmount = math.GetAmountBFromLiquidityDelta(params.MinSqrtPrice, params.SqrtPrice, liquidityDelta, RoundingUp)
	} else {
		liquidityDelta = math.GetLiquidityDeltaFromAmountB(actualAmountIn, params.MinSqrtPrice, params.SqrtPrice)
		rawOutputAmount = math.GetAmountAFromLiquidityDelta(params.SqrtPrice, params.MaxSqrtPrice, liquidityDelta, RoundingUp)
	}
	outputAmount := new(big.Int).Set(rawOutputAmount)
	if params.OutputTokenInfo != nil {
		outputAmount = helpers.CalculateTransferFeeIncludedAmount(rawOutputAmount, params.OutputTokenInfo).Amount
	}
	return DepositQuote{
		ActualInputAmount:   actualAmountIn,
		ConsumedInputAmount: params.InAmount,
		LiquidityDelta:      liquidityDelta,
		OutputAmount:        outputAmount,
	}
}

// GetWithdrawQuote calculates the withdraw quote.
func (c *CpAmm) GetWithdrawQuote(params GetWithdrawQuoteParams) WithdrawQuote {
	amountA := math.GetAmountAFromLiquidityDelta(params.SqrtPrice, params.MaxSqrtPrice, params.LiquidityDelta, RoundingDown)
	amountB := math.GetAmountBFromLiquidityDelta(params.MinSqrtPrice, params.SqrtPrice, params.LiquidityDelta, RoundingDown)
	outA := amountA
	outB := amountB
	if params.TokenATokenInfo != nil {
		outA = helpers.CalculateTransferFeeExcludedAmount(amountA, params.TokenATokenInfo).Amount
	}
	if params.TokenBTokenInfo != nil {
		outB = helpers.CalculateTransferFeeExcludedAmount(amountB, params.TokenBTokenInfo).Amount
	}
	return WithdrawQuote{
		LiquidityDelta: params.LiquidityDelta,
		OutAmountA:     outA,
		OutAmountB:     outB,
	}
}

// PreparePoolCreationSingleSide calculates liquidity for single-sided creation.
func (c *CpAmm) PreparePoolCreationSingleSide(params PreparePoolCreationSingleSide) (*big.Int, error) {
	if params.InitSqrtPrice.Cmp(params.MinSqrtPrice) != 0 {
		return nil, errors.New("only support single side for base token")
	}
	actualAmountIn := params.TokenAAmount
	if params.TokenAInfo != nil {
		feeInfo := helpers.CalculateTransferFeeIncludedAmount(params.TokenAAmount, params.TokenAInfo)
		actualAmountIn = new(big.Int).Sub(params.TokenAAmount, feeInfo.TransferFee)
	}
	liquidityDelta := math.GetLiquidityDeltaFromAmountA(actualAmountIn, params.InitSqrtPrice, params.MaxSqrtPrice)
	return liquidityDelta, nil
}

// PreparePoolCreationParams computes init price and liquidity.
func (c *CpAmm) PreparePoolCreationParams(params PreparePoolCreationParams) (PreparedPoolCreation, error) {
	if params.TokenAAmount.Sign() == 0 && params.TokenBAmount.Sign() == 0 {
		return PreparedPoolCreation{}, errors.New("invalid input amount")
	}
	actualAmountA := params.TokenAAmount
	actualAmountB := params.TokenBAmount
	if params.TokenAInfo != nil {
		feeInfo := helpers.CalculateTransferFeeIncludedAmount(params.TokenAAmount, params.TokenAInfo)
		actualAmountA = new(big.Int).Sub(params.TokenAAmount, feeInfo.TransferFee)
	}
	if params.TokenBInfo != nil {
		feeInfo := helpers.CalculateTransferFeeIncludedAmount(params.TokenBAmount, params.TokenBInfo)
		actualAmountB = new(big.Int).Sub(params.TokenBAmount, feeInfo.TransferFee)
	}
	initSqrtPrice, err := math.CalculateInitSqrtPrice(params.TokenAAmount, params.TokenBAmount, params.MinSqrtPrice, params.MaxSqrtPrice)
	if err != nil {
		return PreparedPoolCreation{}, err
	}
	liquidityA := math.GetLiquidityDeltaFromAmountA(actualAmountA, initSqrtPrice, params.MaxSqrtPrice)
	liquidityB := math.GetLiquidityDeltaFromAmountB(actualAmountB, params.MinSqrtPrice, initSqrtPrice)
	return PreparedPoolCreation{
		InitSqrtPrice:  initSqrtPrice,
		LiquidityDelta: minBig(liquidityA, liquidityB),
	}, nil
}

// CreatePool builds a transaction to create a permissionless pool.
func (c *CpAmm) CreatePool(ctx context.Context, params CreatePoolParams) (TxBuilder, solanago.PublicKey, solanago.PublicKey, solanago.PublicKey, error) {
	pool := DerivePoolAddress(params.Config, params.TokenAMint, params.TokenBMint)
	prepared, err := c.prepareCreatePoolParams(ctx, PrepareCustomizablePoolParams{
		Pool:          pool,
		TokenAMint:    params.TokenAMint,
		TokenBMint:    params.TokenBMint,
		TokenAAmount:  params.TokenAAmount,
		TokenBAmount:  params.TokenBAmount,
		Payer:         params.Payer,
		PositionNft:   params.PositionNft,
		TokenAProgram: params.TokenAProgram,
		TokenBProgram: params.TokenBProgram,
	})
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
	}

	initParams := dammv2gen.InitializePoolParameters{
		Liquidity:       u128FromBig(params.LiquidityDelta),
		SqrtPrice:       u128FromBig(params.InitSqrtPrice),
		ActivationPoint: toU64Ptr(params.ActivationPoint),
	}
	initIx, err := dammv2gen.NewInitializePoolInstruction(
		initParams,
		params.Creator,
		params.PositionNft,
		prepared.PositionNftAccount,
		params.Payer,
		params.Config,
		c.PoolAuthority,
		pool,
		prepared.Position,
		params.TokenAMint,
		params.TokenBMint,
		prepared.TokenAVault,
		prepared.TokenBVault,
		prepared.PayerTokenA,
		prepared.PayerTokenB,
		params.TokenAProgram,
		params.TokenBProgram,
		solanago.Token2022ProgramID,
		solanago.SystemProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	if err := appendRemainingAccounts(initIx, prepared.TokenBadgeAccounts); err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
	}

	builder := solanago.NewTransactionBuilder()
	for _, ix := range prepared.PreInstructions {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(initIx)

	if params.IsLockLiquidity {
		lockIx, err := dammv2gen.NewPermanentLockPositionInstruction(
			u128FromBig(params.LiquidityDelta),
			pool,
			prepared.Position,
			prepared.PositionNftAccount,
			params.Creator,
			c.EventAuthority,
			dammv2gen.ProgramID,
		)
		if err != nil {
			return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
		}
		builder.AddInstruction(lockIx)
	}
	return builder, pool, prepared.Position, prepared.PositionNftAccount, nil
}

// CreateCustomPool builds a transaction to create a customizable pool.
func (c *CpAmm) CreateCustomPool(ctx context.Context, params InitializeCustomizeablePoolParams) (TxBuilder, solanago.PublicKey, solanago.PublicKey, solanago.PublicKey, error) {
	pool := DeriveCustomizablePoolAddress(params.TokenAMint, params.TokenBMint)
	tokenBAmount := params.TokenBAmount
	if params.TokenBMint.Equals(helpers.NativeMint) && tokenBAmount != nil {
		if tokenBAmount.Cmp(big.NewInt(1)) < 0 {
			tokenBAmount = big.NewInt(1)
		}
	}
	prepared, err := c.prepareCreatePoolParams(ctx, PrepareCustomizablePoolParams{
		Pool:          pool,
		TokenAMint:    params.TokenAMint,
		TokenBMint:    params.TokenBMint,
		TokenAAmount:  params.TokenAAmount,
		TokenBAmount:  tokenBAmount,
		Payer:         params.Payer,
		PositionNft:   params.PositionNft,
		TokenAProgram: params.TokenAProgram,
		TokenBProgram: params.TokenBProgram,
	})
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
	}

	poolFees := dammv2gen.PoolFeeParameters{
		BaseFee:    params.PoolFees.BaseFee,
		DynamicFee: params.PoolFees.DynamicFee,
	}
	initParams := dammv2gen.InitializeCustomizablePoolParameters{
		PoolFees:        poolFees,
		SqrtMinPrice:    u128FromBig(params.SqrtMinPrice),
		SqrtMaxPrice:    u128FromBig(params.SqrtMaxPrice),
		HasAlphaVault:   params.HasAlphaVault,
		Liquidity:       u128FromBig(params.LiquidityDelta),
		SqrtPrice:       u128FromBig(params.InitSqrtPrice),
		ActivationType:  uint8(params.ActivationType),
		CollectFeeMode:  uint8(params.CollectFeeMode),
		ActivationPoint: toU64Ptr(params.ActivationPoint),
	}
	initIx, err := dammv2gen.NewInitializeCustomizablePoolInstruction(
		initParams,
		params.Creator,
		params.PositionNft,
		prepared.PositionNftAccount,
		params.Payer,
		c.PoolAuthority,
		pool,
		prepared.Position,
		params.TokenAMint,
		params.TokenBMint,
		prepared.TokenAVault,
		prepared.TokenBVault,
		prepared.PayerTokenA,
		prepared.PayerTokenB,
		params.TokenAProgram,
		params.TokenBProgram,
		solanago.Token2022ProgramID,
		solanago.SystemProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	if err := appendRemainingAccounts(initIx, prepared.TokenBadgeAccounts); err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
	}

	builder := solanago.NewTransactionBuilder()
	for _, ix := range prepared.PreInstructions {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(initIx)
	if params.IsLockLiquidity {
		lockIx, err := dammv2gen.NewPermanentLockPositionInstruction(
			u128FromBig(params.LiquidityDelta),
			pool,
			prepared.Position,
			prepared.PositionNftAccount,
			params.Creator,
			c.EventAuthority,
			dammv2gen.ProgramID,
		)
		if err != nil {
			return nil, solanago.PublicKey{}, solanago.PublicKey{}, solanago.PublicKey{}, err
		}
		builder.AddInstruction(lockIx)
	}
	return builder, pool, prepared.Position, prepared.PositionNftAccount, nil
}

// CreateCustomPoolWithDynamicConfig builds a transaction to create customizable pool with dynamic config.
func (c *CpAmm) CreateCustomPoolWithDynamicConfig(ctx context.Context, params InitializeCustomizeablePoolWithDynamicConfigParams) (TxBuilder, solanago.PublicKey, solanago.PublicKey, error) {
	pool := DerivePoolAddress(params.Config, params.TokenAMint, params.TokenBMint)
	prepared, err := c.prepareCreatePoolParams(ctx, PrepareCustomizablePoolParams{
		Pool:          pool,
		TokenAMint:    params.TokenAMint,
		TokenBMint:    params.TokenBMint,
		TokenAAmount:  params.TokenAAmount,
		TokenBAmount:  params.TokenBAmount,
		Payer:         params.Payer,
		PositionNft:   params.PositionNft,
		TokenAProgram: params.TokenAProgram,
		TokenBProgram: params.TokenBProgram,
	})
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	poolFees := dammv2gen.PoolFeeParameters{
		BaseFee:    params.PoolFees.BaseFee,
		DynamicFee: params.PoolFees.DynamicFee,
	}
	initParams := dammv2gen.InitializeCustomizablePoolParameters{
		PoolFees:        poolFees,
		SqrtMinPrice:    u128FromBig(params.SqrtMinPrice),
		SqrtMaxPrice:    u128FromBig(params.SqrtMaxPrice),
		HasAlphaVault:   params.HasAlphaVault,
		Liquidity:       u128FromBig(params.LiquidityDelta),
		SqrtPrice:       u128FromBig(params.InitSqrtPrice),
		ActivationType:  uint8(params.ActivationType),
		CollectFeeMode:  uint8(params.CollectFeeMode),
		ActivationPoint: toU64Ptr(params.ActivationPoint),
	}
	initIx, err := dammv2gen.NewInitializePoolWithDynamicConfigInstruction(
		initParams,
		params.Creator,
		params.PositionNft,
		prepared.PositionNftAccount,
		params.Payer,
		params.PoolCreatorAuthority,
		params.Config,
		c.PoolAuthority,
		pool,
		prepared.Position,
		params.TokenAMint,
		params.TokenBMint,
		prepared.TokenAVault,
		prepared.TokenBVault,
		prepared.PayerTokenA,
		prepared.PayerTokenB,
		params.TokenAProgram,
		params.TokenBProgram,
		solanago.Token2022ProgramID,
		solanago.SystemProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	if err := appendRemainingAccounts(initIx, prepared.TokenBadgeAccounts); err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range prepared.PreInstructions {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(initIx)
	if params.IsLockLiquidity {
		lockIx, err := dammv2gen.NewPermanentLockPositionInstruction(
			u128FromBig(params.LiquidityDelta),
			pool,
			prepared.Position,
			prepared.PositionNftAccount,
			params.Creator,
			c.EventAuthority,
			dammv2gen.ProgramID,
		)
		if err != nil {
			return nil, solanago.PublicKey{}, solanago.PublicKey{}, err
		}
		builder.AddInstruction(lockIx)
	}
	return builder, pool, prepared.Position, nil
}

// CreatePosition builds a transaction to create a position.
func (c *CpAmm) CreatePosition(ctx context.Context, params CreatePositionParams) (TxBuilder, solanago.PublicKey, solanago.PublicKey, error) {
	ix, position, positionNftAccount, err := c.buildCreatePositionInstruction(params)
	if err != nil {
		return nil, solanago.PublicKey{}, solanago.PublicKey{}, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, position, positionNftAccount, nil
}

// AddLiquidity builds a transaction to add liquidity.
func (c *CpAmm) AddLiquidity(ctx context.Context, params AddLiquidityParams) (TxBuilder, error) {
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)

	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Owner,
		TokenAOwner:   params.Owner,
		TokenBOwner:   params.Owner,
		TokenAMint:    params.PoolState.TokenAMint,
		TokenBMint:    params.PoolState.TokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Owner, tokenAAccount, toU64(params.MaxAmountTokenA))
		preIxs = append(preIxs, wrapIxs...)
	}
	if params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Owner, tokenBAccount, toU64(params.MaxAmountTokenB))
		preIxs = append(preIxs, wrapIxs...)
	}
	var postIxs []solanago.Instruction
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) || params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	addIx, err := c.buildAddLiquidityInstruction(BuildAddLiquidityParams{
		Pool:                  params.Pool,
		Position:              params.Position,
		PositionNftAccount:    params.PositionNftAccount,
		Owner:                 params.Owner,
		TokenAAccount:         tokenAAccount,
		TokenBAccount:         tokenBAccount,
		TokenAMint:            params.PoolState.TokenAMint,
		TokenBMint:            params.PoolState.TokenBMint,
		TokenAVault:           params.PoolState.TokenAVault,
		TokenBVault:           params.PoolState.TokenBVault,
		TokenAProgram:         tokenAProgram,
		TokenBProgram:         tokenBProgram,
		LiquidityDelta:        params.LiquidityDelta,
		TokenAAmountThreshold: params.TokenAAmountThreshold,
		TokenBAmountThreshold: params.TokenBAmountThreshold,
	})
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(addIx)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// CreatePositionAndAddLiquidity builds a transaction to create position and add liquidity.
func (c *CpAmm) CreatePositionAndAddLiquidity(ctx context.Context, params CreatePositionAndAddLiquidity) (TxBuilder, error) {
	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Owner,
		TokenAOwner:   params.Owner,
		TokenBOwner:   params.Owner,
		TokenAMint:    params.TokenAMint,
		TokenBMint:    params.TokenBMint,
		TokenAProgram: params.TokenAProgram,
		TokenBProgram: params.TokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	tokenAVault := DeriveTokenVaultAddress(params.TokenAMint, params.Pool)
	tokenBVault := DeriveTokenVaultAddress(params.TokenBMint, params.Pool)
	if params.TokenAMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Owner, tokenAAccount, toU64(params.MaxAmountTokenA))
		preIxs = append(preIxs, wrapIxs...)
	}
	if params.TokenBMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Owner, tokenBAccount, toU64(params.MaxAmountTokenB))
		preIxs = append(preIxs, wrapIxs...)
	}
	var postIxs []solanago.Instruction
	if params.TokenAMint.Equals(helpers.NativeMint) || params.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	createIx, position, positionNftAccount, err := c.buildCreatePositionInstruction(CreatePositionParams{
		Owner:       params.Owner,
		Payer:       params.Owner,
		Pool:        params.Pool,
		PositionNft: params.PositionNft,
	})
	if err != nil {
		return nil, err
	}
	addIx, err := c.buildAddLiquidityInstruction(BuildAddLiquidityParams{
		Pool:                  params.Pool,
		Position:              position,
		PositionNftAccount:    positionNftAccount,
		Owner:                 params.Owner,
		TokenAAccount:         tokenAAccount,
		TokenBAccount:         tokenBAccount,
		TokenAMint:            params.TokenAMint,
		TokenBMint:            params.TokenBMint,
		TokenAVault:           tokenAVault,
		TokenBVault:           tokenBVault,
		TokenAProgram:         params.TokenAProgram,
		TokenBProgram:         params.TokenBProgram,
		LiquidityDelta:        params.LiquidityDelta,
		TokenAAmountThreshold: params.TokenAAmountThreshold,
		TokenBAmountThreshold: params.TokenBAmountThreshold,
	})
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(createIx)
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(addIx)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// RemoveLiquidity builds a transaction to remove liquidity.
func (c *CpAmm) RemoveLiquidity(ctx context.Context, params RemoveLiquidityParams) (TxBuilder, error) {
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)
	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Owner,
		TokenAOwner:   params.Owner,
		TokenBOwner:   params.Owner,
		TokenAMint:    params.PoolState.TokenAMint,
		TokenBMint:    params.PoolState.TokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	if len(params.Vestings) > 0 {
		vestingAccounts := make([]solanago.PublicKey, 0, len(params.Vestings))
		for _, v := range params.Vestings {
			vestingAccounts = append(vestingAccounts, v.Account)
		}
		refreshIx, err := c.buildRefreshVestingInstruction(RefreshVestingParams{
			Owner:              params.Owner,
			Position:           params.Position,
			PositionNftAccount: params.PositionNftAccount,
			Pool:               params.Pool,
			VestingAccounts:    vestingAccounts,
		})
		if err != nil {
			return nil, err
		}
		preIxs = append(preIxs, refreshIx)
	}
	var postIxs []solanago.Instruction
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) || params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	ixParams := dammv2gen.RemoveLiquidityParameters{
		LiquidityDelta:        u128FromBig(params.LiquidityDelta),
		TokenAAmountThreshold: toU64(params.TokenAAmountThreshold),
		TokenBAmountThreshold: toU64(params.TokenBAmountThreshold),
	}
	removeIx, err := dammv2gen.NewRemoveLiquidityInstruction(
		ixParams,
		c.PoolAuthority,
		params.Pool,
		params.Position,
		tokenAAccount,
		tokenBAccount,
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
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(removeIx)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// RemoveAllLiquidity builds a transaction to remove all liquidity.
func (c *CpAmm) RemoveAllLiquidity(ctx context.Context, params RemoveAllLiquidityParams) (TxBuilder, error) {
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)

	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Owner,
		TokenAOwner:   params.Owner,
		TokenBOwner:   params.Owner,
		TokenAMint:    params.PoolState.TokenAMint,
		TokenBMint:    params.PoolState.TokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	if len(params.Vestings) > 0 {
		vestingAccounts := make([]solanago.PublicKey, 0, len(params.Vestings))
		for _, v := range params.Vestings {
			vestingAccounts = append(vestingAccounts, v.Account)
		}
		refreshIx, err := c.buildRefreshVestingInstruction(RefreshVestingParams{
			Owner:              params.Owner,
			Position:           params.Position,
			PositionNftAccount: params.PositionNftAccount,
			Pool:               params.Pool,
			VestingAccounts:    vestingAccounts,
		})
		if err != nil {
			return nil, err
		}
		preIxs = append(preIxs, refreshIx)
	}
	var postIxs []solanago.Instruction
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) || params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	removeIx, err := c.buildRemoveAllLiquidityInstruction(BuildRemoveAllLiquidityInstructionParams{
		PoolAuthority:         c.PoolAuthority,
		Owner:                 params.Owner,
		Pool:                  params.Pool,
		Position:              params.Position,
		PositionNftAccount:    params.PositionNftAccount,
		TokenAAccount:         tokenAAccount,
		TokenBAccount:         tokenBAccount,
		TokenAAmountThreshold: params.TokenAAmountThreshold,
		TokenBAmountThreshold: params.TokenBAmountThreshold,
		TokenAMint:            params.PoolState.TokenAMint,
		TokenBMint:            params.PoolState.TokenBMint,
		TokenAVault:           params.PoolState.TokenAVault,
		TokenBVault:           params.PoolState.TokenBVault,
		TokenAProgram:         tokenAProgram,
		TokenBProgram:         tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(removeIx)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// Swap builds a swap transaction (exact in).
func (c *CpAmm) Swap(ctx context.Context, params SwapParams) (TxBuilder, error) {
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)

	inputTokenProgram := tokenAProgram
	outputTokenProgram := tokenBProgram
	if !params.InputTokenMint.Equals(params.PoolState.TokenAMint) {
		inputTokenProgram, outputTokenProgram = tokenBProgram, tokenAProgram
	}
	tradeDirection := TradeDirectionAtoB
	if !params.InputTokenMint.Equals(params.PoolState.TokenAMint) {
		tradeDirection = TradeDirectionBtoA
	}
	receiver := params.Payer
	if params.Receiver != nil {
		receiver = *params.Receiver
	}
	inputTokenAccount, outputTokenAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Payer,
		TokenAOwner:   receiver,
		TokenBOwner:   receiver,
		TokenAMint:    params.InputTokenMint,
		TokenBMint:    params.OutputTokenMint,
		TokenAProgram: inputTokenProgram,
		TokenBProgram: outputTokenProgram,
	})
	if err != nil {
		return nil, err
	}
	if params.InputTokenMint.Equals(helpers.NativeMint) {
		wrapIxs, _ := helpers.WrapSOLInstruction(receiver, inputTokenAccount, toU64(params.AmountIn))
		preIxs = append(preIxs, wrapIxs...)
	}
	var postIxs []solanago.Instruction
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) || params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(receiver, receiver, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	poolState := params.PoolState
	if poolState == nil {
		fetched, err := c.FetchPoolState(ctx, params.Pool)
		if err != nil {
			return nil, err
		}
		poolState = fetched
	}
	data := poolState.PoolFees.BaseFee.BaseFeeInfo.Data[:]
	baseFeeMode := BaseFeeMode(data[8])
	rateLimiterApplied := false
	if baseFeeMode == BaseFeeModeRateLimiter {
		currentPoint, err := helpers.GetCurrentPoint(ctx, c.Client, shared.ActivationType(poolState.ActivationType))
		if err != nil {
			return nil, err
		}
		rateLimiterPoolFees, err := helpers.DecodePodAlignedFeeRateLimiter(data)
		if err != nil {
			return nil, err
		}
		rateLimiterApplied = pool_fees.IsRateLimiterApplied(
			new(big.Int).SetUint64(rateLimiterPoolFees.ReferenceAmount),
			rateLimiterPoolFees.MaxLimiterDuration,
			uint16(rateLimiterPoolFees.MaxFeeBps),
			rateLimiterPoolFees.FeeIncrementBps,
			currentPoint,
			big.NewInt(int64(poolState.ActivationPoint)),
			tradeDirection,
		)
	}
	swapIx, err := dammv2gen.NewSwapInstruction(
		dammv2gen.SwapParameters{
			AmountIn:         toU64(params.AmountIn),
			MinimumAmountOut: toU64(params.MinimumAmountOut),
		},
		c.PoolAuthority,
		params.Pool,
		inputTokenAccount,
		outputTokenAccount,
		poolState.TokenAVault,
		poolState.TokenBVault,
		poolState.TokenAMint,
		poolState.TokenBMint,
		receiver,
		tokenAProgram,
		tokenBProgram,
		optionalPubkey(params.ReferralTokenAccount),
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	if rateLimiterApplied {
		if err := appendRemainingAccounts(swapIx, []*solanago.AccountMeta{solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false)}); err != nil {
			return nil, err
		}
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(swapIx)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// Swap2 builds a swap transaction with swap modes.
func (c *CpAmm) Swap2(ctx context.Context, params Swap2Params) (TxBuilder, error) {
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)

	inputTokenProgram := tokenAProgram
	outputTokenProgram := tokenBProgram

	if !params.InputTokenMint.Equals(params.PoolState.TokenAMint) {
		inputTokenProgram, outputTokenProgram = tokenBProgram, tokenAProgram
	}
	tradeDirection := TradeDirectionAtoB
	if !params.InputTokenMint.Equals(params.PoolState.TokenAMint) {
		tradeDirection = TradeDirectionBtoA
	}
	receiver := params.Payer
	if params.Receiver != nil {
		receiver = *params.Receiver
	}
	inputTokenAccount, outputTokenAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Payer,
		TokenAOwner:   receiver,
		TokenBOwner:   receiver,
		TokenAMint:    params.InputTokenMint,
		TokenBMint:    params.OutputTokenMint,
		TokenAProgram: inputTokenProgram,
		TokenBProgram: outputTokenProgram,
	})
	if err != nil {
		return nil, err
	}

	var amount0, amount1 *big.Int
	if params.SwapMode == SwapModeExactOut {
		if params.AmountOut == nil || params.MaximumAmountIn == nil {
			return nil, errors.New("amountOut and maximumAmountIn are required for ExactOut")
		}
		amount0 = params.AmountOut
		amount1 = params.MaximumAmountIn
	} else {
		if params.AmountIn == nil || params.MinimumAmountOut == nil {
			return nil, errors.New("amountIn and minimumAmountOut are required for ExactIn/PartialFill")
		}
		amount0 = params.AmountIn
		amount1 = params.MinimumAmountOut
	}
	if params.InputTokenMint.Equals(helpers.NativeMint) {
		wrapAmount := amount0
		if params.SwapMode == SwapModeExactOut {
			wrapAmount = amount1
		}
		wrapIxs, _ := helpers.WrapSOLInstruction(receiver, inputTokenAccount, toU64(wrapAmount))
		preIxs = append(preIxs, wrapIxs...)
	}
	var postIxs []solanago.Instruction
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) || params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(receiver, receiver, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}

	poolState := params.PoolState
	if poolState == nil {
		fetched, err := c.FetchPoolState(ctx, params.Pool)
		if err != nil {
			return nil, err
		}
		poolState = fetched
	}
	data := poolState.PoolFees.BaseFee.BaseFeeInfo.Data[:]
	baseFeeMode := BaseFeeMode(data[8])
	rateLimiterApplied := false
	if baseFeeMode == BaseFeeModeRateLimiter {
		currentPoint, err := helpers.GetCurrentPoint(ctx, c.Client, shared.ActivationType(poolState.ActivationType))
		if err != nil {
			return nil, err
		}
		rateLimiterPoolFees, err := helpers.DecodePodAlignedFeeRateLimiter(data)
		if err != nil {
			return nil, err
		}
		rateLimiterApplied = pool_fees.IsRateLimiterApplied(
			new(big.Int).SetUint64(rateLimiterPoolFees.ReferenceAmount),
			rateLimiterPoolFees.MaxLimiterDuration,
			uint16(rateLimiterPoolFees.MaxFeeBps),
			rateLimiterPoolFees.FeeIncrementBps,
			currentPoint,
			big.NewInt(int64(poolState.ActivationPoint)),
			tradeDirection,
		)
	}

	swapIx, err := dammv2gen.NewSwap2Instruction(
		dammv2gen.SwapParameters2{
			Amount0:  toU64(amount0),
			Amount1:  toU64(amount1),
			SwapMode: uint8(params.SwapMode),
		},
		c.PoolAuthority,
		params.Pool,
		inputTokenAccount,
		outputTokenAccount,
		poolState.TokenAVault,
		poolState.TokenBVault,
		poolState.TokenAMint,
		poolState.TokenBMint,
		receiver,
		tokenAProgram,
		tokenBProgram,
		optionalPubkey(params.ReferralTokenAccount),
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	if rateLimiterApplied {
		if err := appendRemainingAccounts(swapIx, []*solanago.AccountMeta{solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false)}); err != nil {
			return nil, err
		}
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(swapIx)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// LockPosition builds a transaction to lock a position.
func (c *CpAmm) LockPosition(ctx context.Context, params LockPositionParams) (TxBuilder, error) {
	vestingParams := dammv2gen.VestingParameters{
		CliffPoint:           toU64Ptr(params.CliffPoint),
		PeriodFrequency:      toU64(params.PeriodFrequency),
		CliffUnlockLiquidity: u128FromBig(params.CliffUnlockLiquidity),
		LiquidityPerPeriod:   u128FromBig(params.LiquidityPerPeriod),
		NumberOfPeriod:       uint16(params.NumberOfPeriod),
	}
	if params.InnerPosition {
		ix, err := dammv2gen.NewLockInnerPositionInstruction(
			vestingParams,
			params.Pool,
			params.Position,
			params.PositionNftAccount,
			params.Owner,
			c.EventAuthority,
			dammv2gen.ProgramID,
		)
		if err != nil {
			return nil, err
		}
		builder := solanago.NewTransactionBuilder()
		builder.AddInstruction(ix)
		return builder, nil
	}
	if params.VestingAccount == nil {
		return nil, errors.New("vesting account required for lock position")
	}
	ix, err := dammv2gen.NewLockPositionInstruction(
		vestingParams,
		params.Pool,
		params.Position,
		optionalPubkey(params.VestingAccount),
		params.PositionNftAccount,
		params.Owner,
		params.Payer,
		solanago.SystemProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// PermanentLockPosition builds a transaction to permanently lock a position.
func (c *CpAmm) PermanentLockPosition(ctx context.Context, params PermanentLockParams) (TxBuilder, error) {
	ix, err := dammv2gen.NewPermanentLockPositionInstruction(
		u128FromBig(params.UnlockedLiquidity),
		params.Pool,
		params.Position,
		params.PositionNftAccount,
		params.Owner,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// RefreshVesting builds a transaction to refresh vesting.
func (c *CpAmm) RefreshVesting(ctx context.Context, params RefreshVestingParams) (TxBuilder, error) {
	ix, err := c.buildRefreshVestingInstruction(params)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// ClosePosition builds a transaction to close a position.
func (c *CpAmm) ClosePosition(ctx context.Context, params ClosePositionParams) (TxBuilder, error) {
	ix, err := c.buildClosePositionInstruction(ClosePositionInstructionParams{
		Owner:              params.Owner,
		PoolAuthority:      c.PoolAuthority,
		Pool:               params.Pool,
		Position:           params.Position,
		PositionNftMint:    params.PositionNftMint,
		PositionNftAccount: params.PositionNftAccount,
	})
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// RemoveAllLiquidityAndClosePosition builds a transaction to claim, remove, and close.
func (c *CpAmm) RemoveAllLiquidityAndClosePosition(ctx context.Context, params RemoveAllLiquidityAndClosePositionParams) (TxBuilder, error) {
	canUnlock, reason := c.canUnlockPosition(params.PositionState, params.Vestings, params.CurrentPoint)
	if !canUnlock {
		return nil, errors.New("cannot remove liquidity: " + reason)
	}
	pool := params.PositionState.Pool
	tokenAMint := params.PoolState.TokenAMint
	tokenBMint := params.PoolState.TokenBMint
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)
	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Owner,
		TokenAOwner:   params.Owner,
		TokenBOwner:   params.Owner,
		TokenAMint:    tokenAMint,
		TokenBMint:    tokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	if len(params.Vestings) > 0 {
		vestingAccounts := make([]solanago.PublicKey, 0, len(params.Vestings))
		for _, v := range params.Vestings {
			vestingAccounts = append(vestingAccounts, v.Account)
		}
		refreshIx, err := c.buildRefreshVestingInstruction(RefreshVestingParams{
			Owner:              params.Owner,
			Position:           params.Position,
			PositionNftAccount: params.PositionNftAccount,
			Pool:               pool,
			VestingAccounts:    vestingAccounts,
		})
		if err != nil {
			return nil, err
		}
		preIxs = append(preIxs, refreshIx)
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	liquidateIxs, err := c.buildLiquidatePositionInstruction(BuildLiquidatePositionInstructionParams{
		Owner:                 params.Owner,
		Position:              params.Position,
		PositionNftAccount:    params.PositionNftAccount,
		PositionState:         params.PositionState,
		PoolState:             params.PoolState,
		TokenAAccount:         tokenAAccount,
		TokenBAccount:         tokenBAccount,
		TokenAAmountThreshold: params.TokenAAmountThreshold,
		TokenBAmountThreshold: params.TokenBAmountThreshold,
	})
	if err != nil {
		return nil, err
	}
	for _, ix := range liquidateIxs {
		builder.AddInstruction(ix)
	}
	if tokenAMint.Equals(helpers.NativeMint) || tokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if closeIx != nil {
			builder.AddInstruction(closeIx)
		}
	}
	return builder, nil
}

// MergePosition merges liquidity from position B to A.
func (c *CpAmm) MergePosition(ctx context.Context, params MergePositionParams) (TxBuilder, error) {
	canUnlock, reason := c.canUnlockPosition(params.PositionBState, params.PositionBVestings, params.CurrentPoint)
	if !canUnlock {
		return nil, errors.New("cannot remove liquidity: " + reason)
	}
	pool := params.PositionBState.Pool
	tokenAMint := params.PoolState.TokenAMint
	tokenBMint := params.PoolState.TokenBMint
	tokenAVault := params.PoolState.TokenAVault
	tokenBVault := params.PoolState.TokenBVault
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)
	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         params.Owner,
		TokenAOwner:   params.Owner,
		TokenBOwner:   params.Owner,
		TokenAMint:    tokenAMint,
		TokenBMint:    tokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	positionBLiquidityDelta := new(big.Int).Set(params.PositionBState.UnlockedLiquidity.BigInt())
	if len(params.PositionBVestings) > 0 {
		totalAvailable := big.NewInt(0)
		for _, v := range params.PositionBVestings {
			available := helpers.GetAvailableVestingLiquidity(v.VestingState, params.CurrentPoint)
			totalAvailable.Add(totalAvailable, available)
		}
		positionBLiquidityDelta.Add(positionBLiquidityDelta, totalAvailable)
		vestingAccounts := make([]solanago.PublicKey, 0, len(params.PositionBVestings))
		for _, v := range params.PositionBVestings {
			vestingAccounts = append(vestingAccounts, v.Account)
		}
		refreshIx, err := c.buildRefreshVestingInstruction(RefreshVestingParams{
			Owner:              params.Owner,
			Position:           params.PositionB,
			PositionNftAccount: params.PositionBNftAccount,
			Pool:               pool,
			VestingAccounts:    vestingAccounts,
		})
		if err != nil {
			return nil, err
		}
		preIxs = append(preIxs, refreshIx)
	}

	tokenAWithdraw := math.GetAmountAFromLiquidityDelta(params.PoolState.SqrtPrice.BigInt(), params.PoolState.SqrtMaxPrice.BigInt(), positionBLiquidityDelta, RoundingDown)
	tokenBWithdraw := math.GetAmountBFromLiquidityDelta(params.PoolState.SqrtMinPrice.BigInt(), params.PoolState.SqrtPrice.BigInt(), positionBLiquidityDelta, RoundingDown)
	newLiquidityDelta := c.GetLiquidityDelta(LiquidityDeltaParams{
		MaxAmountTokenA: tokenAWithdraw,
		MaxAmountTokenB: tokenBWithdraw,
		SqrtPrice:       params.PoolState.SqrtPrice.BigInt(),
		SqrtMinPrice:    params.PoolState.SqrtMinPrice.BigInt(),
		SqrtMaxPrice:    params.PoolState.SqrtMaxPrice.BigInt(),
	})
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	liquidateIxs, err := c.buildLiquidatePositionInstruction(BuildLiquidatePositionInstructionParams{
		Owner:                 params.Owner,
		Position:              params.PositionB,
		PositionNftAccount:    params.PositionBNftAccount,
		PositionState:         params.PositionBState,
		PoolState:             params.PoolState,
		TokenAAccount:         tokenAAccount,
		TokenBAccount:         tokenBAccount,
		TokenAAmountThreshold: params.TokenAAmountRemoveLiquidityThreshold,
		TokenBAmountThreshold: params.TokenBAmountRemoveLiquidityThreshold,
	})
	if err != nil {
		return nil, err
	}
	for _, ix := range liquidateIxs {
		builder.AddInstruction(ix)
	}
	addIx, err := c.buildAddLiquidityInstruction(BuildAddLiquidityParams{
		Pool:                  pool,
		Position:              params.PositionA,
		PositionNftAccount:    params.PositionANftAccount,
		Owner:                 params.Owner,
		TokenAAccount:         tokenAAccount,
		TokenBAccount:         tokenBAccount,
		TokenAMint:            tokenAMint,
		TokenBMint:            tokenBMint,
		TokenAVault:           tokenAVault,
		TokenBVault:           tokenBVault,
		TokenAProgram:         tokenAProgram,
		TokenBProgram:         tokenBProgram,
		LiquidityDelta:        newLiquidityDelta,
		TokenAAmountThreshold: params.TokenAAmountAddLiquidityThreshold,
		TokenBAmountThreshold: params.TokenBAmountAddLiquidityThreshold,
	})
	if err != nil {
		return nil, err
	}
	builder.AddInstruction(addIx)
	if tokenAMint.Equals(helpers.NativeMint) || tokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Owner, true)
		if closeIx != nil {
			builder.AddInstruction(closeIx)
		}
	}
	return builder, nil
}

// InitializeReward builds a transaction to initialize reward.
func (c *CpAmm) InitializeReward(ctx context.Context, params InitializeRewardParams) (TxBuilder, error) {
	rewardVault := DeriveRewardVaultAddress(params.Pool, params.RewardIndex)
	tokenBadge := DeriveTokenBadgeAddress(params.RewardMint)
	operator := DeriveOperatorAddress(params.Creator)
	remaining := []*solanago.AccountMeta{}
	tokenBadgeInfo, _ := c.Client.GetAccountInfoWithOpts(ctx, tokenBadge, nil)
	operatorInfo, _ := c.Client.GetAccountInfoWithOpts(ctx, operator, nil)
	if tokenBadgeInfo != nil && tokenBadgeInfo.Value != nil {
		remaining = append(remaining, solanago.NewAccountMeta(tokenBadge, false, false))
	} else {
		remaining = append(remaining, solanago.NewAccountMeta(CpAmmProgramID, false, false))
	}
	if operatorInfo != nil && operatorInfo.Value != nil {
		remaining = append(remaining, solanago.NewAccountMeta(operator, false, false))
	}
	ix, err := dammv2gen.NewInitializeRewardInstruction(
		params.RewardIndex,
		toU64(params.RewardDuration),
		params.Funder,
		c.PoolAuthority,
		params.Pool,
		rewardVault,
		params.RewardMint,
		params.Creator,
		params.Payer,
		helpers.GetTokenProgram(params.RewardIndex),
		solanago.SystemProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	if err := appendRemainingAccounts(ix, remaining); err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// InitializeAndFundReward builds a transaction to initialize and fund reward.
func (c *CpAmm) InitializeAndFundReward(ctx context.Context, params InitializeAndFundReward) (TxBuilder, error) {
	builder := solanago.NewTransactionBuilder()

	// Initialize reward.
	rewardVault := DeriveRewardVaultAddress(params.Pool, params.RewardIndex)
	tokenBadge := DeriveTokenBadgeAddress(params.RewardMint)
	operator := DeriveOperatorAddress(params.Creator)
	remaining := []*solanago.AccountMeta{}
	tokenBadgeInfo, _ := c.Client.GetAccountInfoWithOpts(ctx, tokenBadge, nil)
	operatorInfo, _ := c.Client.GetAccountInfoWithOpts(ctx, operator, nil)
	if tokenBadgeInfo != nil && tokenBadgeInfo.Value != nil {
		remaining = append(remaining, solanago.NewAccountMeta(tokenBadge, false, false))
	} else {
		remaining = append(remaining, solanago.NewAccountMeta(CpAmmProgramID, false, false))
	}
	if operatorInfo != nil && operatorInfo.Value != nil {
		remaining = append(remaining, solanago.NewAccountMeta(operator, false, false))
	}
	initIx, err := dammv2gen.NewInitializeRewardInstruction(
		params.RewardIndex,
		toU64(params.RewardDuration),
		params.Payer,
		c.PoolAuthority,
		params.Pool,
		rewardVault,
		params.RewardMint,
		params.Creator,
		params.Payer,
		helpers.GetTokenProgram(params.RewardIndex),
		solanago.SystemProgramID,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	if err := appendRemainingAccounts(initIx, remaining); err != nil {
		return nil, err
	}
	builder.AddInstruction(initIx)

	// Fund reward.
	preIxs := []solanago.Instruction{}
	funderTokenAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, params.RewardMint, params.Payer, params.Payer, helpers.GetTokenProgram(params.RewardIndex))
	if err != nil {
		return nil, err
	}
	if createIx != nil {
		preIxs = append(preIxs, createIx)
	}
	if params.RewardMint.Equals(helpers.NativeMint) && params.Amount.Sign() > 0 {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Payer, funderTokenAccount, toU64(params.Amount))
		preIxs = append(preIxs, wrapIxs...)
	}
	fundIx, err := dammv2gen.NewFundRewardInstruction(
		params.RewardIndex,
		toU64(params.Amount),
		params.CarryForward,
		params.Pool,
		rewardVault,
		params.RewardMint,
		funderTokenAccount,
		params.Payer,
		helpers.GetTokenProgram(params.RewardIndex),
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(fundIx)
	return builder, nil
}

// UpdateRewardDuration builds a transaction to update reward duration.
func (c *CpAmm) UpdateRewardDuration(ctx context.Context, params UpdateRewardDurationParams) (TxBuilder, error) {
	ix, err := dammv2gen.NewUpdateRewardDurationInstruction(
		params.RewardIndex,
		toU64(params.NewDuration),
		params.Pool,
		params.Signer,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// UpdateRewardFunder builds a transaction to update reward funder.
func (c *CpAmm) UpdateRewardFunder(ctx context.Context, params UpdateRewardFunderParams) (TxBuilder, error) {
	ix, err := dammv2gen.NewUpdateRewardFunderInstruction(
		params.RewardIndex,
		params.NewFunder,
		params.Pool,
		params.Signer,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// FundReward builds a transaction to fund reward.
func (c *CpAmm) FundReward(ctx context.Context, params FundRewardParams) (TxBuilder, error) {
	preIxs := []solanago.Instruction{}
	funderTokenAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, params.RewardMint, params.Funder, params.Funder, helpers.GetTokenProgram(params.RewardIndex))
	if err != nil {
		return nil, err
	}
	if createIx != nil {
		preIxs = append(preIxs, createIx)
	}
	if params.RewardMint.Equals(helpers.NativeMint) && params.Amount.Sign() > 0 {
		wrapIxs, _ := helpers.WrapSOLInstruction(params.Funder, funderTokenAccount, toU64(params.Amount))
		preIxs = append(preIxs, wrapIxs...)
	}
	ix, err := dammv2gen.NewFundRewardInstruction(
		params.RewardIndex,
		toU64(params.Amount),
		params.CarryForward,
		params.Pool,
		params.RewardVault,
		params.RewardMint,
		funderTokenAccount,
		params.Funder,
		helpers.GetTokenProgram(params.RewardIndex),
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(ix)
	return builder, nil
}

// WithdrawIneligibleReward builds a transaction to withdraw ineligible reward.
func (c *CpAmm) WithdrawIneligibleReward(ctx context.Context, params WithdrawIneligibleRewardParams) (TxBuilder, error) {
	poolState, err := c.FetchPoolState(ctx, params.Pool)
	if err != nil {
		return nil, err
	}
	rewardInfo := poolState.RewardInfos[int(params.RewardIndex)]
	tokenProgram := helpers.GetTokenProgram(rewardInfo.RewardTokenFlag)
	preIxs := []solanago.Instruction{}
	postIxs := []solanago.Instruction{}
	funderTokenAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, rewardInfo.Mint, params.Funder, params.Funder, tokenProgram)
	if err != nil {
		return nil, err
	}
	if createIx != nil {
		preIxs = append(preIxs, createIx)
	}
	if rewardInfo.Mint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Funder, params.Funder, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	ix, err := dammv2gen.NewWithdrawIneligibleRewardInstruction(
		params.RewardIndex,
		c.PoolAuthority,
		params.Pool,
		rewardInfo.Vault,
		rewardInfo.Mint,
		funderTokenAccount,
		params.Funder,
		tokenProgram,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(ix)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// ClaimPartnerFee builds a transaction to claim partner fee.
func (c *CpAmm) ClaimPartnerFee(ctx context.Context, params ClaimPartnerFeeParams) (TxBuilder, error) {
	poolState, err := c.FetchPoolState(ctx, params.Pool)
	if err != nil {
		return nil, err
	}
	tokenAProgram := helpers.GetTokenProgram(poolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(poolState.TokenBFlag)
	payer := params.Partner
	if params.FeePayer != nil {
		payer = *params.FeePayer
	}
	tokenAAccount, tokenBAccount, preIxs, postIxs, err := c.setupFeeClaimAccounts(ctx, SetupFeeClaimAccountsParams{
		Payer:           payer,
		Owner:           params.Partner,
		TokenAMint:      poolState.TokenAMint,
		TokenBMint:      poolState.TokenBMint,
		TokenAProgram:   tokenAProgram,
		TokenBProgram:   tokenBProgram,
		Receiver:        params.Receiver,
		TempWSolAccount: params.TempWSolAccount,
	})
	if err != nil {
		return nil, err
	}
	ix, err := dammv2gen.NewClaimPartnerFeeInstruction(
		toU64(params.MaxAmountA),
		toU64(params.MaxAmountB),
		c.PoolAuthority,
		params.Pool,
		tokenAAccount,
		tokenBAccount,
		poolState.TokenAVault,
		poolState.TokenBVault,
		poolState.TokenAMint,
		poolState.TokenBMint,
		params.Partner,
		tokenAProgram,
		tokenBProgram,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(ix)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// ClaimPositionFee builds a transaction to claim position fees.
func (c *CpAmm) ClaimPositionFee(ctx context.Context, params ClaimPositionFeeParams) (TxBuilder, error) {
	payer := params.Owner
	if params.FeePayer != nil {
		payer = *params.FeePayer
	}
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)
	tokenAAccount, tokenBAccount, preIxs, postIxs, err := c.setupFeeClaimAccounts(ctx, SetupFeeClaimAccountsParams{
		Payer:           payer,
		Owner:           params.Owner,
		TokenAMint:      params.PoolState.TokenAMint,
		TokenBMint:      params.PoolState.TokenBMint,
		TokenAProgram:   tokenAProgram,
		TokenBProgram:   tokenBProgram,
		Receiver:        params.Receiver,
		TempWSolAccount: params.TempWSolAccount,
	})
	if err != nil {
		return nil, err
	}
	ix, err := c.buildClaimPositionFeeInstruction(ClaimPositionFeeInstructionParams{
		Owner:              params.Owner,
		PoolAuthority:      c.PoolAuthority,
		Pool:               params.Pool,
		Position:           params.Position,
		PositionNftAccount: params.PositionNftAccount,
		TokenAAccount:      tokenAAccount,
		TokenBAccount:      tokenBAccount,
		PoolState:          params.PoolState,
	})
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(ix)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// ClaimPositionFee2 builds a transaction to claim position fees (receiver required).
func (c *CpAmm) ClaimPositionFee2(ctx context.Context, params ClaimPositionFeeParams2) (TxBuilder, error) {
	payer := params.Owner
	if params.FeePayer != nil {
		payer = *params.FeePayer
	}
	tokenAOwner := params.Receiver
	tokenBOwner := params.Receiver
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) {
		tokenAOwner = params.Owner
	}
	if params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		tokenBOwner = params.Owner
	}
	tokenAProgram := helpers.GetTokenProgram(params.PoolState.TokenAFlag)
	tokenBProgram := helpers.GetTokenProgram(params.PoolState.TokenBFlag)
	tokenAAccount, tokenBAccount, preIxs, err := c.prepareTokenAccounts(ctx, PrepareTokenAccountParams{
		Payer:         payer,
		TokenAOwner:   tokenAOwner,
		TokenBOwner:   tokenBOwner,
		TokenAMint:    params.PoolState.TokenAMint,
		TokenBMint:    params.PoolState.TokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	})
	if err != nil {
		return nil, err
	}
	postIxs := []solanago.Instruction{}
	if params.PoolState.TokenAMint.Equals(helpers.NativeMint) || params.PoolState.TokenBMint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.Owner, params.Receiver, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	ix, err := c.buildClaimPositionFeeInstruction(ClaimPositionFeeInstructionParams{
		Owner:              params.Owner,
		PoolAuthority:      c.PoolAuthority,
		Pool:               params.Pool,
		Position:           params.Position,
		PositionNftAccount: params.PositionNftAccount,
		TokenAAccount:      tokenAAccount,
		TokenBAccount:      tokenBAccount,
		PoolState:          params.PoolState,
	})
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(ix)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// ClaimReward builds a transaction to claim reward.
func (c *CpAmm) ClaimReward(ctx context.Context, params ClaimRewardParams) (TxBuilder, error) {
	rewardInfo := params.PoolState.RewardInfos[int(params.RewardIndex)]
	tokenProgram := helpers.GetTokenProgram(rewardInfo.RewardTokenFlag)
	preIxs := []solanago.Instruction{}
	postIxs := []solanago.Instruction{}
	payer := params.User
	if params.FeePayer != nil {
		payer = *params.FeePayer
	}
	userTokenAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, c.Client, rewardInfo.Mint, params.User, payer, tokenProgram)
	if err != nil {
		return nil, err
	}
	if createIx != nil {
		preIxs = append(preIxs, createIx)
	}
	if rewardInfo.Mint.Equals(helpers.NativeMint) {
		closeIx, _ := helpers.UnwrapSOLInstruction(params.User, params.User, true)
		if closeIx != nil {
			postIxs = append(postIxs, closeIx)
		}
	}
	skipReward := uint8(0)
	if params.IsSkipReward {
		skipReward = 1
	}
	ix, err := dammv2gen.NewClaimRewardInstruction(
		params.RewardIndex,
		skipReward,
		c.PoolAuthority,
		params.Pool,
		params.Position,
		rewardInfo.Vault,
		rewardInfo.Mint,
		userTokenAccount,
		params.PositionNftAccount,
		params.User,
		tokenProgram,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	for _, ix := range preIxs {
		builder.AddInstruction(ix)
	}
	builder.AddInstruction(ix)
	for _, ix := range postIxs {
		builder.AddInstruction(ix)
	}
	return builder, nil
}

// SplitPosition builds a transaction to split position.
func (c *CpAmm) SplitPosition(ctx context.Context, params SplitPositionParams) (TxBuilder, error) {
	param := dammv2gen.SplitPositionParameters{
		PermanentLockedLiquidityPercentage: params.PermanentLockedLiquidityPercentage,
		UnlockedLiquidityPercentage:        params.UnlockedLiquidityPercentage,
		FeeAPercentage:                     params.FeeAPercentage,
		FeeBPercentage:                     params.FeeBPercentage,
		Reward0Percentage:                  params.Reward0Percentage,
		Reward1Percentage:                  params.Reward1Percentage,
		InnerVestingLiquidityPercentage:    params.InnerVestingLiquidityPercentage,
		Padding:                            [15]uint8{},
	}
	ix, err := dammv2gen.NewSplitPositionInstruction(
		param,
		params.Pool,
		params.FirstPosition,
		params.FirstPositionNftAccount,
		params.SecondPosition,
		params.SecondPositionNftAccount,
		params.FirstPositionOwner,
		params.SecondPositionOwner,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

// SplitPosition2 builds a transaction to split position with numerator.
func (c *CpAmm) SplitPosition2(ctx context.Context, params SplitPosition2Params) (TxBuilder, error) {
	ix, err := dammv2gen.NewSplitPosition2Instruction(
		params.Numerator,
		params.Pool,
		params.FirstPosition,
		params.FirstPositionNftAccount,
		params.SecondPosition,
		params.SecondPositionNftAccount,
		params.FirstPositionOwner,
		params.SecondPositionOwner,
		c.EventAuthority,
		dammv2gen.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	builder := solanago.NewTransactionBuilder()
	builder.AddInstruction(ix)
	return builder, nil
}

func appendRemainingAccounts(ix solanago.Instruction, metas []*solanago.AccountMeta) error {
	if len(metas) == 0 {
		return nil
	}
	gen, ok := ix.(*solanago.GenericInstruction)
	if !ok {
		return errors.New("unsupported instruction type for remaining accounts")
	}
	gen.AccountValues = append(gen.AccountValues, metas...)
	return nil
}

func toU64Ptr(v *big.Int) *uint64 {
	if v == nil {
		return nil
	}
	val := v.Uint64()
	return &val
}

func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) <= 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}

func optionalPubkey(pk *solanago.PublicKey) solanago.PublicKey {
	if pk == nil {
		return dammv2gen.ProgramID
	}
	return *pk
}
