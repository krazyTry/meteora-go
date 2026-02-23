package meteora

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	jsoniter "github.com/json-iterator/go"
	dammv2 "github.com/krazyTry/meteora-go/damm_v2"
	"github.com/krazyTry/meteora-go/damm_v2/helpers"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
	"github.com/krazyTry/meteora-go/gen/damm_v2"
	"github.com/shopspring/decimal"
)

func TestMergePositionDAMMv2(t *testing.T) {
	return

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	mintWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("mint address:", mintWallet.PublicKey())

	ctx := context.Background()
	TokenMintto(t, ctx, rpcClient, wsClient, mintWallet, ownerWallet, partner)
	fmt.Println("mintto success")

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	baseAmount := big.NewInt(1_000_000)
	quoteAmount := big.NewInt(1) // SOL

	positionFirstNftWallet := solana.NewWallet()

	positionSecondNftWallet := solana.NewWallet()

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, solana.WrappedSol)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	initialPoolTokenBaseAmount := helpers.GetInitialPoolTokenAmount(baseAmount, inputTokenInfo.Decimals)
	initialPoolTokenQuoteAmount := helpers.GetInitialPoolTokenAmount(quoteAmount, outputTokenInfo.Decimals)

	// 1 base token = 1 quote token
	initSqrtPrice := helpers.GetSqrtPriceFromPrice(decimal.NewFromFloat(1), inputTokenInfo.Decimals, outputTokenInfo.Decimals)

	tokenBaseProgram := inputTokenInfo.Owner
	tokenQuoteProgram := outputTokenInfo.Owner

	liquidityDelta := cpAmm.GetLiquidityDelta(dammv2.LiquidityDeltaParams{
		MaxAmountTokenA: initialPoolTokenBaseAmount,
		MaxAmountTokenB: initialPoolTokenQuoteAmount,
		SqrtPrice:       initSqrtPrice,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
	})

	baseFeeParam, err := helpers.GetFeeTimeSchedulerParams(
		5000, // 50%
		25,   // 0.25%
		shared.BaseFeeModeFeeTimeSchedulerExponential,
		60,   // 60 peridos
		3600, // 60 * 60
	)
	if err != nil {
		t.Fatal("helpers.GetFeeTimeSchedulerParams() fail", err)
	}

	var dynamicFeeParam *damm_v2.DynamicFeeParameters
	dynamicFeeParam, err = helpers.GetDynamicFeeParams(25, shared.MaxPriceChangeBpsDefault)
	if err != nil {
		t.Fatal("helpers.GetDynamicFeeParams() fail", err)
	}

	poolFees := dammv2.PoolFeesParams{
		BaseFee:    baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    []uint8{},
	}

	txBuilder, poolAddress, positionNftFirst, positionNftFirstAccount, err := cpAmm.CreateCustomPool(ctx, dammv2.InitializeCustomizeablePoolParams{
		Payer:           owner,
		Creator:         owner,
		PositionNft:     positionFirstNftWallet.PublicKey(),
		TokenAMint:      mintWallet.PublicKey(),
		TokenBMint:      solana.WrappedSol,
		TokenAAmount:    baseAmount,
		TokenBAmount:    quoteAmount,
		InitSqrtPrice:   initSqrtPrice,
		LiquidityDelta:  liquidityDelta,
		TokenAProgram:   tokenBaseProgram,
		TokenBProgram:   tokenQuoteProgram,
		ActivationPoint: nil,
		IsLockLiquidity: false,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
		PoolFees:        poolFees,
		HasAlphaVault:   false,
		ActivationType:  dammv2.ActivationTypeTimestamp,
		CollectFeeMode:  dammv2.CollectFeeModeBothToken,
	})
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreateCustomPool txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionFirstNftWallet.PublicKey()):
			return &positionFirstNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreateCustomPool SendTransaction() fail", err)
	}
	fmt.Println("create custom pool success sig:", sig.String())

	txBuilder, positionNftSecond, positionNftSecondAccount, err := cpAmm.CreatePosition(ctx, dammv2.CreatePositionParams{
		Owner:       owner,
		Payer:       owner,
		Pool:        poolAddress,
		PositionNft: positionSecondNftWallet.PublicKey(),
	})
	if err != nil {
		t.Fatal("cpAmm.CreatePosition() fail", err)
	}
	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreatePosition txBuilder.Build() fail", err)
	}
	sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionSecondNftWallet.PublicKey()):
			return &positionSecondNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreatePosition SendTransaction() fail", err)
	}
	fmt.Println("create position success sig:", sig.String())

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail", err)
	}
	if len(pools) == 0 {
		fmt.Println("pool does not exist:", mintWallet.PublicKey())
		return
	}

	inAmount := new(big.Int).SetUint64(0.1 * 1e9)
	depositQuote := cpAmm.GetDepositQuote(dammv2.GetDepositQuoteParams{
		InAmount:        inAmount,
		IsTokenA:        true,
		MinSqrtPrice:    pools[0].Account.SqrtMinPrice.BigInt(),
		MaxSqrtPrice:    pools[0].Account.SqrtMaxPrice.BigInt(),
		SqrtPrice:       pools[0].Account.SqrtPrice.BigInt(),
		InputTokenInfo:  inputTokenInfo,
		OutputTokenInfo: outputTokenInfo,
	})

	txBuilder, err = cpAmm.AddLiquidity(ctx, dammv2.AddLiquidityParams{
		Owner:                 owner,
		Pool:                  pools[0].PublicKey,
		PoolState:             pools[0].Account,
		Position:              positionNftSecond,
		PositionNftAccount:    positionNftSecondAccount,
		LiquidityDelta:        depositQuote.LiquidityDelta,
		MaxAmountTokenA:       inAmount,
		MaxAmountTokenB:       inAmount,
		TokenAAmountThreshold: shared.U64Max,
		TokenBAmountThreshold: shared.U64Max,
	})

	if err != nil {
		t.Fatal("cpAmm.AddLiquidity() fail", err)
	}
	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("AddLiquidity txBuilder.Build() fail", err)
	}
	sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("AddLiquidity SendTransaction() fail", err)
	}
	fmt.Println("add liquidity success Success sig:", sig.String())

	positionBState, err := cpAmm.FetchPositionState(ctx, positionNftSecond)
	if err != nil {
		t.Fatal("FetchPositionState() fail", err)
	}
	vestings, err := cpAmm.GetAllVestingsByPosition(ctx, positionNftSecond)
	if err != nil {
		t.Fatal("GetAllVestingsByPosition() fail", err)
	}
	poolStates, err := cpAmm.GetMultiplePools(ctx, []solana.PublicKey{poolAddress})
	if err != nil {
		t.Fatal("GetMultiplePools() fail", err)
	}

	fmt.Println("UnlockedLiquidity", positionBState.UnlockedLiquidity.BigInt())

	currentPoint := dammv2.CurrentPointForActivation(ctx, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(poolStates[0].ActivationType))

	txBuilder, err = cpAmm.MergePosition(ctx, dammv2.MergePositionParams{
		Owner:                                owner,
		PositionA:                            positionNftFirst,
		PositionB:                            positionNftSecond,
		PoolState:                            poolStates[0],
		PositionBNftAccount:                  positionNftSecondAccount,
		PositionANftAccount:                  positionNftFirstAccount,
		PositionBState:                       positionBState,
		PositionBVestings:                    vestings,
		CurrentPoint:                         currentPoint,
		TokenAAmountAddLiquidityThreshold:    shared.U64Max,
		TokenBAmountAddLiquidityThreshold:    shared.U64Max,
		TokenAAmountRemoveLiquidityThreshold: new(big.Int).SetUint64(0),
		TokenBAmountRemoveLiquidityThreshold: new(big.Int).SetUint64(0),
	})
	if err != nil {
		t.Fatal("cpAmm.MergePosition() fail", err)
	}

	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("MergePosition txBuilder.Build() fail", err)
	}
	sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("MergePosition SendTransaction() fail", err)
	}
	fmt.Println("merge position success sig:", sig.String())

	currentPoint = dammv2.CurrentPointForActivation(ctx, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(poolStates[0].ActivationType))

	quote2, err := cpAmm.GetQuote2(dammv2.GetQuote2Params{
		InputTokenMint:  poolStates[0].TokenBMint,
		Slippage:        50_00, // 50%
		PoolState:       poolStates[0],
		CurrentPoint:    currentPoint,
		InputTokenInfo:  inputTokenInfo,
		OutputTokenInfo: outputTokenInfo,
		TokenADecimal:   9,
		TokenBDecimal:   9,
		// HasReferral     bool
		SwapMode: dammv2.SwapModeExactIn,
		AmountIn: new(big.Int).SetUint64(1e9 * 0.1),
		// AmountOut       *big.Int
	})
	if err != nil {
		t.Fatal("cpAmm.GetQuote() fail", err)
	}

	txBuilder, err = cpAmm.Swap2(ctx, dammv2.Swap2Params{
		Payer:           owner,
		Pool:            poolAddress,
		PoolState:       poolStates[0],
		InputTokenMint:  poolStates[0].TokenAMint,
		OutputTokenMint: poolStates[0].TokenBMint,
		// ReferralTokenAccount *solanago.PublicKey
		// Receiver             *solanago.PublicKey
		SwapMode:         dammv2.SwapModeExactIn,
		AmountIn:         new(big.Int).SetUint64(1e9 * 0.1),
		MinimumAmountOut: quote2.MinimumAmountOut,
		// AmountOut            *big.Int
		// MaximumAmountIn      *big.Int
	})

	if err != nil {
		t.Fatal("cpAmm.Swap2() fail", err)
	}
	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("Swap2 txBuilder.Build() fail", err)
	}
	sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("Swap2 SendTransaction() fail", err)
	}
	fmt.Println("swap2 success Success sig:", sig.String())

	positions, err := cpAmm.GetPositionsByUser(ctx, ownerWallet.PublicKey())
	if err != nil {
		t.Fatal("cpAmm.GetPositionsByUser() fail", err)
	}

	var ownerPosition dammv2.UserPosition
	for _, v := range positions {
		if !v.PositionState.Pool.Equals(poolAddress) {
			continue
		}
		ownerPosition = v
		break
	}
	baseFee, quoteFee, _, err := helpers.GetUnClaimLpFee(poolStates[0], ownerPosition.PositionState)
	if err != nil {
		t.Fatal("helpers.GetUnclaimedLpFee() fail", err)
	}
	fmt.Println("baseFee:", baseFee)
	fmt.Println("quoteFee:", quoteFee)

	if quoteFee.Sign() > 0 {

		txBuilder, err = cpAmm.ClaimPositionFee(ctx, dammv2.ClaimPositionFeeParams{
			Owner:              owner,
			Position:           ownerPosition.Position,
			Pool:               poolAddress,
			PositionNftAccount: ownerPosition.PositionNftAccount,
			PoolState:          poolStates[0],
			// Receiver           *solanago.PublicKey
			// FeePayer           *solanago.PublicKey
			// TempWSolAccount    *solanago.PublicKey
		})
		if err != nil {
			t.Fatal("cpAmm.ClaimPositionFee() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("claim txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("claim SendTransaction() fail", err)
		}
		fmt.Println("claim success Success sig:", sig.String())

	}
}

func TestCreateCustomPoolDAMMv2(t *testing.T) {
	return

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	mintWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("mint address:", mintWallet.PublicKey())

	ctx := context.Background()
	TokenMintto(t, ctx, rpcClient, wsClient, mintWallet, ownerWallet, partner)
	fmt.Println("mintto success")

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	baseAmount := big.NewInt(1_000_000)
	quoteAmount := big.NewInt(1) // SOL

	positionFirstNftWallet := solana.NewWallet()

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, solana.WrappedSol)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	initialPoolTokenBaseAmount := helpers.GetInitialPoolTokenAmount(baseAmount, inputTokenInfo.Decimals)
	initialPoolTokenQuoteAmount := helpers.GetInitialPoolTokenAmount(quoteAmount, outputTokenInfo.Decimals)

	// 1 base token = 1 quote token
	initSqrtPrice := helpers.GetSqrtPriceFromPrice(decimal.NewFromFloat(1), inputTokenInfo.Decimals, outputTokenInfo.Decimals)

	tokenBaseProgram := inputTokenInfo.Owner
	tokenQuoteProgram := outputTokenInfo.Owner

	liquidityDelta := cpAmm.GetLiquidityDelta(dammv2.LiquidityDeltaParams{
		MaxAmountTokenA: initialPoolTokenBaseAmount,
		MaxAmountTokenB: initialPoolTokenQuoteAmount,
		SqrtPrice:       initSqrtPrice,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
	})

	baseFeeParam, err := helpers.GetFeeTimeSchedulerParams(
		5000, // 50%
		25,   // 0.25%
		shared.BaseFeeModeFeeTimeSchedulerExponential,
		60,   // 60 peridos
		3600, // 60 * 60
	)
	if err != nil {
		t.Fatal("helpers.GetFeeTimeSchedulerParams() fail", err)
	}

	var dynamicFeeParam *damm_v2.DynamicFeeParameters
	dynamicFeeParam, err = helpers.GetDynamicFeeParams(25, shared.MaxPriceChangeBpsDefault)
	if err != nil {
		t.Fatal("helpers.GetDynamicFeeParams() fail", err)
	}

	poolFees := dammv2.PoolFeesParams{
		BaseFee:    baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    []uint8{},
	}

	txBuilder, _, _, _, err := cpAmm.CreateCustomPool(ctx, dammv2.InitializeCustomizeablePoolParams{
		Payer:           owner,
		Creator:         owner,
		PositionNft:     positionFirstNftWallet.PublicKey(),
		TokenAMint:      mintWallet.PublicKey(),
		TokenBMint:      solana.WrappedSol,
		TokenAAmount:    baseAmount,
		TokenBAmount:    quoteAmount,
		InitSqrtPrice:   initSqrtPrice,
		LiquidityDelta:  liquidityDelta,
		TokenAProgram:   tokenBaseProgram,
		TokenBProgram:   tokenQuoteProgram,
		ActivationPoint: nil,
		IsLockLiquidity: false,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
		PoolFees:        poolFees,
		HasAlphaVault:   false,
		ActivationType:  dammv2.ActivationTypeTimestamp,
		CollectFeeMode:  dammv2.CollectFeeModeBothToken,
	})
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreateCustomPool txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionFirstNftWallet.PublicKey()):
			return &positionFirstNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreateCustomPool SendTransaction() fail", err)
	}
	fmt.Println("create custom pool success sig:", sig.String())

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail", err)
	}
	if len(pools) == 0 {
		fmt.Println("pool does not exist:", mintWallet.PublicKey())
		return
	}
	for idx, pool := range pools {
		fmt.Println("===========================")
		fmt.Println("print pool info")
		fmt.Println("dammv2 Index", idx)
		fmt.Println("dammv2.PoolAddress", pool.PublicKey)
		fmt.Println("dammv2.TokenAMint", pool.Account.TokenAMint)
		fmt.Println("dammv2.TokenBMint", pool.Account.TokenBMint)
		fmt.Println("dammv2.SqrtMinPrice", pool.Account.SqrtMinPrice)
		fmt.Println("dammv2.SqrtMaxPrice", pool.Account.SqrtMaxPrice)
		fmt.Println("dammv2.SqrtPrice", pool.Account.SqrtPrice)
		fmt.Println("dammv2.Liquidity", pool.Account.Liquidity)
		j, _ := jsoniter.MarshalToString(pool)
		fmt.Println("dammv2.Json", j)
		fmt.Println("===========================")
	}
}
func TestCreatePoolDAMMv2(t *testing.T) {
	return

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	mintWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("mint address:", mintWallet.PublicKey())

	ctx := context.Background()
	TokenMintto(t, ctx, rpcClient, wsClient, mintWallet, ownerWallet, partner)
	fmt.Println("mintto success")

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	baseAmount := big.NewInt(1_000_000)
	quoteAmount := big.NewInt(1) // SOL

	positionFirstNftWallet := solana.NewWallet()

	// positionSecondNftWallet := solana.NewWallet()

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, solana.WrappedSol)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	initialPoolTokenBaseAmount := helpers.GetInitialPoolTokenAmount(baseAmount, inputTokenInfo.Decimals)
	initialPoolTokenQuoteAmount := helpers.GetInitialPoolTokenAmount(quoteAmount, outputTokenInfo.Decimals)

	// 1 base token = 1 quote token
	initSqrtPrice := helpers.GetSqrtPriceFromPrice(decimal.NewFromFloat(1), inputTokenInfo.Decimals, outputTokenInfo.Decimals)

	tokenBaseProgram := inputTokenInfo.Owner
	tokenQuoteProgram := outputTokenInfo.Owner

	config := dammv2.DeriveConfigAddress(0)

	configState, err := cpAmm.FetchConfigState(ctx, config)
	if err != nil {
		t.Fatal("dammv2.FetchConfigState() fail", err)
	}

	liquidityDelta := cpAmm.GetLiquidityDelta(dammv2.LiquidityDeltaParams{
		MaxAmountTokenA: initialPoolTokenBaseAmount,
		MaxAmountTokenB: initialPoolTokenQuoteAmount,
		SqrtPrice:       initSqrtPrice,
		SqrtMinPrice:    configState.SqrtMinPrice.BigInt(),
		SqrtMaxPrice:    configState.SqrtMaxPrice.BigInt(),
	})

	txBuilder, _, _, _, err := cpAmm.CreatePool(
		ctx,
		dammv2.CreatePoolParams{
			Payer:           owner,
			Creator:         owner,
			PositionNft:     positionFirstNftWallet.PublicKey(),
			TokenAMint:      mintWallet.PublicKey(),
			TokenBMint:      solana.WrappedSol,
			TokenAAmount:    baseAmount,
			TokenBAmount:    quoteAmount,
			Config:          config,
			InitSqrtPrice:   initSqrtPrice,
			LiquidityDelta:  liquidityDelta,
			TokenAProgram:   tokenBaseProgram,
			TokenBProgram:   tokenQuoteProgram,
			ActivationPoint: nil,
			IsLockLiquidity: true,
		},
	)

	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreatePool txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionFirstNftWallet.PublicKey()):
			return &positionFirstNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreatePool SendTransaction() fail", err)
	}
	fmt.Println("create pool success sig:", sig.String())

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail", err)
	}
	if len(pools) == 0 {
		fmt.Println("pool does not exist:", mintWallet.PublicKey())
		return
	}
	for idx, pool := range pools {
		fmt.Println("===========================")
		fmt.Println("print pool info")
		fmt.Println("dammv2 Index", idx)
		fmt.Println("dammv2.PoolAddress", pool.PublicKey)
		fmt.Println("dammv2.TokenAMint", pool.Account.TokenAMint)
		fmt.Println("dammv2.TokenBMint", pool.Account.TokenBMint)
		fmt.Println("dammv2.SqrtMinPrice", pool.Account.SqrtMinPrice)
		fmt.Println("dammv2.SqrtMaxPrice", pool.Account.SqrtMaxPrice)
		fmt.Println("dammv2.SqrtPrice", pool.Account.SqrtPrice)
		fmt.Println("dammv2.Liquidity", pool.Account.Liquidity)
		j, _ := jsoniter.MarshalToString(pool)
		fmt.Println("dammv2.Json", j)
		fmt.Println("===========================")
	}
}

func TestSplitDAMMv2(t *testing.T) {
	return

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	mintWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("mint address:", mintWallet.PublicKey())

	ctx := context.Background()
	TokenMintto(t, ctx, rpcClient, wsClient, mintWallet, ownerWallet, partner)
	fmt.Println("mintto success")

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	baseAmount := big.NewInt(1_000_000)
	quoteAmount := big.NewInt(1) // SOL

	positionFirstNftWallet := solana.NewWallet()

	positionSecondNftWallet := solana.NewWallet()

	inputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, mintWallet.PublicKey())
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	outputTokenInfo, err := helpers.GetTokenInfo(ctx, rpcClient, solana.WrappedSol)
	if err != nil {
		t.Fatal("dammv2.GetTokenInfo() fail", err)
	}

	initialPoolTokenBaseAmount := helpers.GetInitialPoolTokenAmount(baseAmount, inputTokenInfo.Decimals)
	initialPoolTokenQuoteAmount := helpers.GetInitialPoolTokenAmount(quoteAmount, outputTokenInfo.Decimals)

	// 1 base token = 1 quote token
	initSqrtPrice := helpers.GetSqrtPriceFromPrice(decimal.NewFromFloat(1), inputTokenInfo.Decimals, outputTokenInfo.Decimals)

	tokenBaseProgram := inputTokenInfo.Owner
	tokenQuoteProgram := outputTokenInfo.Owner

	liquidityDelta := cpAmm.GetLiquidityDelta(dammv2.LiquidityDeltaParams{
		MaxAmountTokenA: initialPoolTokenBaseAmount,
		MaxAmountTokenB: initialPoolTokenQuoteAmount,
		SqrtPrice:       initSqrtPrice,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
	})

	baseFeeParam, err := helpers.GetFeeTimeSchedulerParams(
		5000, // 50%
		25,   // 0.25%
		shared.BaseFeeModeFeeTimeSchedulerExponential,
		60,   // 60 peridos
		3600, // 60 * 60
	)
	if err != nil {
		t.Fatal("helpers.GetFeeTimeSchedulerParams() fail", err)
	}

	var dynamicFeeParam *damm_v2.DynamicFeeParameters
	dynamicFeeParam, err = helpers.GetDynamicFeeParams(25, shared.MaxPriceChangeBpsDefault)
	if err != nil {
		t.Fatal("helpers.GetDynamicFeeParams() fail", err)
	}

	poolFees := dammv2.PoolFeesParams{
		BaseFee:    baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    []uint8{},
	}

	txBuilder, poolAddress, positionNftFirst, _, err := cpAmm.CreateCustomPool(ctx, dammv2.InitializeCustomizeablePoolParams{
		Payer:           owner,
		Creator:         owner,
		PositionNft:     positionFirstNftWallet.PublicKey(),
		TokenAMint:      mintWallet.PublicKey(),
		TokenBMint:      solana.WrappedSol,
		TokenAAmount:    baseAmount,
		TokenBAmount:    quoteAmount,
		InitSqrtPrice:   initSqrtPrice,
		LiquidityDelta:  liquidityDelta,
		TokenAProgram:   tokenBaseProgram,
		TokenBProgram:   tokenQuoteProgram,
		ActivationPoint: nil,
		IsLockLiquidity: false,
		SqrtMinPrice:    shared.MinSqrtPrice,
		SqrtMaxPrice:    shared.MaxSqrtPrice,
		PoolFees:        poolFees,
		HasAlphaVault:   false,
		ActivationType:  dammv2.ActivationTypeTimestamp,
		CollectFeeMode:  dammv2.CollectFeeModeBothToken,
	})
	tx, err := txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreateCustomPool txBuilder.Build() fail", err)
	}
	sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionFirstNftWallet.PublicKey()):
			return &positionFirstNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreateCustomPool SendTransaction() fail", err)
	}
	fmt.Println("create custom pool success sig:", sig.String())

	txBuilder, positionNftSecond, _, err := cpAmm.CreatePosition(ctx, dammv2.CreatePositionParams{
		Owner:       owner,
		Payer:       owner,
		Pool:        poolAddress,
		PositionNft: positionSecondNftWallet.PublicKey(),
	})
	if err != nil {
		t.Fatal("cpAmm.CreatePosition() fail", err)
	}
	tx, err = txBuilder.SetFeePayer(owner).Build()
	if err != nil {
		t.Fatal("CreatePosition txBuilder.Build() fail", err)
	}
	sig, err = SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(owner):
			return &ownerWallet.PrivateKey
		case key.Equals(positionSecondNftWallet.PublicKey()):
			return &positionSecondNftWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("CreatePosition SendTransaction() fail", err)
	}
	fmt.Println("create position success sig:", sig.String())

	{
		txBuilder, err := cpAmm.SplitPosition(ctx, dammv2.SplitPositionParams{
			FirstPositionOwner:                 owner,
			SecondPositionOwner:                owner,
			Pool:                               poolAddress,
			FirstPosition:                      positionNftFirst,
			FirstPositionNftAccount:            dammv2.DerivePositionNftAccount(positionFirstNftWallet.PublicKey()),
			SecondPosition:                     positionNftSecond,
			SecondPositionNftAccount:           dammv2.DerivePositionNftAccount(positionSecondNftWallet.PublicKey()),
			PermanentLockedLiquidityPercentage: 0,
			UnlockedLiquidityPercentage:        50,
			FeeAPercentage:                     50,
			FeeBPercentage:                     50,
			Reward0Percentage:                  50,
			Reward1Percentage:                  50,
			// InnerVestingLiquidityPercentage    uint8
		})
		if err != nil {
			t.Fatal("cpAmm.SplitPosition() fail", err)
		}

		tx, err := txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("SplitPosition txBuilder.Build() fail", err)
		}
		sig, err := SendTransaction(ctx, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("SplitPosition SendTransaction() fail", err)
		}
		fmt.Println("split position success sig:", sig.String())
	}
}

func TestClaimDAMMv2(t *testing.T) {
	return

	baseMint := solana.MustPublicKeyFromBase58("")

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

	ctx1 := context.Background()

	cpAmm := dammv2.NewCpAmm(rpcClient, rpc.CommitmentFinalized)

	pools, err := cpAmm.FetchPoolStatesByTokenAMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("cpAmm.FetchPoolStatesByTokenAMint() fail", err)
	}

	for _, v := range pools {
		fmt.Println("pool:", v.PublicKey)
		fmt.Println(v.Account)
	}

	pool := pools[0]

	// positions, err := cpAmm.GetPositionsByUser(ctx1, owner)
	// if err != nil {
	// 	t.Fatal("cpAmm.GetPositionsByUser() fail", err)
	// }
	// var userPosition dammv2.UserPosition
	// for _, v := range positions {
	// 	if !v.PositionState.Pool.Equals(pool.PublicKey) {
	// 		continue
	// 	}
	// 	userPosition = v
	// 	break
	// }
	// fmt.Println("userPosition", userPosition)

	{
		// var (
		// 	txBuilder dammv2.TxBuilder
		// 	tx        *solana.Transaction
		// 	sig       solana.Signature
		// )
		// position, positionNftAccount := userPosition.Position, userPosition.PositionNftAccount
		// positionNft := userPosition.PositionState.NftMint

		positionNftWallet := solana.NewWallet()
		positionNft := positionNftWallet.PublicKey()
		txBuilder, position, positionNftAccount, err := cpAmm.CreatePosition(ctx1, dammv2.CreatePositionParams{
			Owner:       owner,
			Payer:       owner,
			Pool:        pool.PublicKey,
			PositionNft: positionNft,
		})
		if err != nil {
			t.Fatal("cpAmm.CreatePosition() fail", err)
		}
		tx, err := txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("CreatePosition txBuilder.Build() fail", err)
		}
		sig, err := SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			case key.Equals(positionNft):
				return &positionNftWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("CreatePosition SendTransaction() fail", err)
		}
		fmt.Println("create position success sig:", sig.String())

		inputTokenInfo, err := helpers.GetTokenInfo(ctx1, rpcClient, pool.Account.TokenAMint)
		if err != nil {
			t.Fatal("dammv2.GetTokenInfo() fail", err)
		}

		outputTokenInfo, err := helpers.GetTokenInfo(ctx1, rpcClient, pool.Account.TokenBMint)
		if err != nil {
			t.Fatal("dammv2.GetTokenInfo() fail", err)
		}

		inAmount := new(big.Int).SetUint64(0.1 * 1e9)
		depositQuote := cpAmm.GetDepositQuote(dammv2.GetDepositQuoteParams{
			InAmount:        inAmount,
			IsTokenA:        true,
			MinSqrtPrice:    pool.Account.SqrtMinPrice.BigInt(),
			MaxSqrtPrice:    pool.Account.SqrtMaxPrice.BigInt(),
			SqrtPrice:       pool.Account.SqrtPrice.BigInt(),
			InputTokenInfo:  inputTokenInfo,
			OutputTokenInfo: outputTokenInfo,
		})

		txBuilder, err = cpAmm.AddLiquidity(ctx1, dammv2.AddLiquidityParams{
			Owner:                 owner,
			Pool:                  pool.PublicKey,
			PoolState:             pool.Account,
			Position:              position,
			PositionNftAccount:    positionNftAccount,
			LiquidityDelta:        depositQuote.LiquidityDelta,
			MaxAmountTokenA:       inAmount,
			MaxAmountTokenB:       inAmount,
			TokenAAmountThreshold: shared.U64Max,
			TokenBAmountThreshold: shared.U64Max,
		})

		if err != nil {
			t.Fatal("cpAmm.AddLiquidity() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("AddLiquidity txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("AddLiquidity SendTransaction() fail", err)
		}
		fmt.Println("add liquidity success Success sig:", sig.String())

		txBuilder, err = cpAmm.RemoveLiquidity(ctx1, dammv2.RemoveLiquidityParams{
			Owner:                 owner,
			Pool:                  pool.PublicKey,
			PoolState:             pool.Account,
			Position:              position,
			PositionNftAccount:    positionNftAccount,
			LiquidityDelta:        depositQuote.LiquidityDelta,
			TokenAAmountThreshold: big.NewInt(0),
			TokenBAmountThreshold: big.NewInt(0),
			// Vestings              []VestingWithAccount
			// CurrentPoint          *big.Int
		})
		if err != nil {
			t.Fatal("cpAmm.RemoveLiquidity() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("RemoveLiquidity txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("RemoveLiquidity SendTransaction() fail", err)
		}
		fmt.Println("remove liquidity success Success sig:", sig.String())

		txBuilder, err = cpAmm.ClosePosition(ctx1, dammv2.ClosePositionParams{
			Owner:              owner,
			Pool:               pool.PublicKey,
			Position:           position,
			PositionNftAccount: positionNftAccount,
			PositionNftMint:    positionNft,
		})

		if err != nil {
			t.Fatal("cpAmm.ClosePosition() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("ClosePosition txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("ClosePosition SendTransaction() fail", err)
		}
		fmt.Println("close position success Success sig:", sig.String())

		balance, err := MintBalance(ctx1, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("MintBalance() fail", err)
		}

		currentPoint := dammv2.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(pool.Account.ActivationType))

		quote, err := cpAmm.GetQuote(dammv2.GetQuoteParams{
			InAmount:        new(big.Int).SetUint64(balance / 3),
			InputTokenMint:  pool.Account.TokenAMint,
			Slippage:        5000,
			PoolState:       pool.Account,
			CurrentPoint:    currentPoint,
			InputTokenInfo:  inputTokenInfo,
			OutputTokenInfo: outputTokenInfo,
			TokenADecimal:   9,
			TokenBDecimal:   9,
			// HasReferral     bool
		})
		if err != nil {
			t.Fatal("cpAmm.GetQuote() fail", err)
		}

		txBuilder, err = cpAmm.Swap(ctx1, dammv2.SwapParams{
			Payer:            owner,
			Pool:             pool.PublicKey,
			PoolState:        pool.Account,
			InputTokenMint:   pool.Account.TokenAMint,
			OutputTokenMint:  pool.Account.TokenBMint,
			AmountIn:         new(big.Int).SetUint64(balance / 3),
			MinimumAmountOut: quote.MinSwapOutAmount,
			// ReferralTokenAccount *solanago.PublicKey
			// Receiver             *solanago.PublicKey
		})
		if err != nil {
			t.Fatal("cpAmm.Swap() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("Swap txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("Swap SendTransaction() fail", err)
		}
		fmt.Println("swap success Success sig:", sig.String())

		currentPoint = dammv2.CurrentPointForActivation(ctx1, rpcClient, rpc.CommitmentFinalized, dammv2.ActivationType(pool.Account.ActivationType))

		quote2, err := cpAmm.GetQuote2(dammv2.GetQuote2Params{
			InputTokenMint:  pool.Account.TokenAMint,
			Slippage:        10000,
			PoolState:       pool.Account,
			CurrentPoint:    currentPoint,
			InputTokenInfo:  inputTokenInfo,
			OutputTokenInfo: outputTokenInfo,
			TokenADecimal:   9,
			TokenBDecimal:   9,
			// HasReferral     bool
			SwapMode: dammv2.SwapModeExactIn,
			AmountIn: new(big.Int).SetUint64(balance - balance/3),
			// AmountOut       *big.Int
		})
		if err != nil {
			t.Fatal("cpAmm.GetQuote() fail", err)
		}

		txBuilder, err = cpAmm.Swap2(ctx1, dammv2.Swap2Params{
			Payer:           owner,
			Pool:            pool.PublicKey,
			PoolState:       pool.Account,
			InputTokenMint:  pool.Account.TokenAMint,
			OutputTokenMint: pool.Account.TokenBMint,
			// ReferralTokenAccount *solanago.PublicKey
			// Receiver             *solanago.PublicKey
			SwapMode:         dammv2.SwapModeExactIn,
			AmountIn:         new(big.Int).SetUint64(balance - balance/3),
			MinimumAmountOut: quote2.MinimumAmountOut,
			// AmountOut            *big.Int
			// MaximumAmountIn      *big.Int
		})

		if err != nil {
			t.Fatal("cpAmm.Swap2() fail", err)
		}
		tx, err = txBuilder.SetFeePayer(owner).Build()
		if err != nil {
			t.Fatal("Swap2 txBuilder.Build() fail", err)
		}
		sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("Swap2 SendTransaction() fail", err)
		}
		fmt.Println("swap2 success Success sig:", sig.String())

		positions, err := cpAmm.GetPositionsByUser(ctx1, partner.PublicKey())
		if err != nil {
			t.Fatal("cpAmm.GetPositionsByUser() fail", err)
		}

		var partnerPosition dammv2.UserPosition
		for _, v := range positions {
			if !v.PositionState.Pool.Equals(pool.PublicKey) {
				continue
			}
			partnerPosition = v
			break
		}

		pools, err := cpAmm.GetMultiplePools(ctx1, []solana.PublicKey{pool.PublicKey})
		if err != nil {
			t.Fatal("cpAmm.GetMultiplePools() fail", err)
		}

		baseFee, quoteFee, _, err := helpers.GetUnClaimLpFee(pools[0], partnerPosition.PositionState)
		if err != nil {
			t.Fatal("helpers.GetUnclaimedLpFee() fail", err)
		}
		fmt.Println("baseFee:", baseFee)
		fmt.Println("quoteFee:", quoteFee)

		if quoteFee.Sign() > 0 {

			txBuilder, err = cpAmm.ClaimPositionFee(ctx1, dammv2.ClaimPositionFeeParams{
				Owner:              partner.PublicKey(),
				Position:           partnerPosition.Position,
				Pool:               pool.PublicKey,
				PositionNftAccount: partnerPosition.PositionNftAccount,
				PoolState:          pools[0],
				// Receiver           *solanago.PublicKey
				// FeePayer           *solanago.PublicKey
				// TempWSolAccount    *solanago.PublicKey
			})
			if err != nil {
				t.Fatal("cpAmm.ClaimPositionFee() fail", err)
			}
			tx, err = txBuilder.SetFeePayer(partner.PublicKey()).Build()
			if err != nil {
				t.Fatal("claim txBuilder.Build() fail", err)
			}
			sig, err = SendTransaction(ctx1, rpcClient, wsClient, tx, func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(partner.PublicKey()):
					return &partner.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("claim SendTransaction() fail", err)
			}
			fmt.Println("claim success Success sig:", sig.String())
		}

	}
}
