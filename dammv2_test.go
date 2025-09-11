package meteora

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	dammV2 "github.com/krazyTry/meteora-go/damm.v2"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func TestDammV2(t *testing.T) {

	// init
	rpcClient, wsClient, pctx, cancel, err := testInit()
	if err != nil {
		t.Fatal("testInit() fail", err)
	}
	ctx := *pctx
	defer (*cancel)()

	// Create a payment account for the token sol > 2
	payer := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("3wXb4MZeb8uueTiEuCN3EF9rQ6Ro6WfUG28AQ7a41kBwLyXjbrfKdWuHup85Ce6rVTwryVW5mJ57e1qnJMUhmxmh")}

	// account for buying and selling sol > 2
	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("2jYSi3Kpgf2KPhQrAGpUiP3csginxjLp7omVAMhvBpnHbxUDffnZNi4mM5ErH1pHMPzxTUimnnfZaoBgcCiEZ1DR")}
	owner := ownerWallet.PublicKey()

	{
		fmt.Println("wallet address:", payer.PublicKey())
		_, err := testBalance(ctx, rpcClient, payer.PublicKey())
		if err != nil {
			t.Fatal("testBalance() fail", err)
		}
	}

	{
		fmt.Println("owner address:", owner)
		_, err := testBalance(ctx, rpcClient, owner)
		if err != nil {
			t.Fatal("testBalance() fail", err)
		}
	}

	config := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("27Ub4t71yxh4yeHcRDGeKBKPKHQkqcmPxeDRnSTihVmApmSciG8i4y4Pa7NgfMqJ3gWuDnCJQRp1ygb6uQb99x6V")}
	fmt.Printf("config address:%s(%s)\n", config.PublicKey(), config.PrivateKey)

	poolCreator := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("DcnbqgS69gTSCaAV5cXe191vMMDSpB3rxPDW9Ha12rxktMkzzNpJGMkCJk2t84s1EpWhjSToHTjBD53VnXRPSRB")}
	fmt.Printf("poolCreator address:%s(%s)\n", poolCreator.PublicKey(), poolCreator.PrivateKey)

	poolPartner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("34kbWHxXmaCsvwyyXvhZKWXyK72RnusUTCkSnEdF9DNddGPhezrgeBuUxYee8xvijSbDcMGzer7MPEmRJc9rifmB")}
	fmt.Printf("poolPartner address:%s(%s)\n", poolPartner.PublicKey(), poolPartner.PrivateKey)

	leftoverReceiver := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("hxQbRSBAyaSyfNnV4vGBaPvGP8G25WFLoQFGhEtCsN9Cvcfdhk12ybHLuWCkngGdzttCBAdwvZX183XCSLb9fqd")}
	fmt.Printf("leftoverReceiver address:%s(%s)\n", leftoverReceiver.PublicKey(), leftoverReceiver.PrivateKey)

	{
		fmt.Println("transfer a little sol to poolPartner")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		if _, err = testTransferSOL(ctx1, rpcClient, wsClient, payer, poolPartner.PublicKey(), 0.1*1e9); err != nil {
			t.Fatal("testTransferSOL fail", err)
		}
	}

	fmt.Printf("\n\n")

	if err = dammV2.Init(); err != nil {
		t.Fatal("dammV2.Init() fail", err)
	}

	meteoraDammV2, err := dammV2.NewDammV2(
		wsClient,
		rpcClient,
		poolCreator,
	)
	if err != nil {
		t.Fatal("NewDammV2() fail", err)
	}

	// {
	// 	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	// 	defer cancel1()

	// 	baseMint := solana.MustPublicKeyFromBase58("EAeMX1T6Jzm7jMm7bVsfu7kLyZSxpEhdmtRUyMXJfaNC")

	// 	fmt.Println(meteoraDammV2.GetPoolByBaseMint(ctx1, baseMint))
	// 	return
	// }
	// {
	// 	baseMint := solana.MustPublicKeyFromBase58("EAeMX1T6Jzm7jMm7bVsfu7kLyZSxpEhdmtRUyMXJfaNC")
	// 	z, _ := decimal.NewFromString("79226673521066979257578248091")
	// 	y, _ := decimal.NewFromString("18446744073709551616")

	// 	liquidityDelta := cp_amm.GetLiquidityDelta(
	// 		decimal.NewFromInt(1000000),
	// 		decimal.NewFromInt(1),
	// 		z,                              // cp_amm.MAX_SQRT_PRICE,
	// 		decimal.NewFromInt(4295048016), // cp_amm.MIN_SQRT_PRICE,
	// 		y,
	// 	)
	// 	fmt.Println(liquidityDelta)

	// 	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	// 	defer cancel1()

	// 	amountIn := new(big.Int).SetUint64(uint64(0.2 * 1e9))
	// 	minOutAmount, _, err := meteoraDammV2.BuyQuote(ctx1, baseMint, amountIn, 250)
	// 	if err != nil {
	// 		t.Fatal("BuyQuote fail", err)
	// 	}
	// 	fmt.Println(minOutAmount)

	// 	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	// 	defer cancel1()
	// 	amountIn := new(big.Int).SetUint64(1_000_000 * 1e9)
	// 	quote, _, err := meteoraDammV2.GetDepositQuote(ctx1, baseMint, false, amountIn)
	// 	if err != nil {
	// 		t.Fatal("meteoraDammV2.GetDepositQuote fail", err)
	// 	}
	// 	fmt.Println(quote)

	// 	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(decimal.NewFromFloat(1.0), 9, 9)
	// 	if err != nil {
	// 		t.Fatal("GetSqrtPriceFromPrice", err)
	// 	}
	// 	fmt.Println(initSqrtPrice)

	// 	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	// 	defer cancel1()
	// 	baseMint := solana.MustPublicKeyFromBase58("9EpFqwBgu9JkxZYEaG1suWzVeA4YR14bkci2gJHU896y")
	// 	liquidityDelta, _ := new(big.Int).SetString("18446744078004599633000037588", 10)
	// 	quote, _, err := meteoraDammV2.GetWithdrawQuote(ctx1, baseMint, liquidityDelta)
	// 	if err != nil {
	// 		t.Fatal("meteoraDammV2.GetWithdrawQuote fail", err)
	// 	}
	// 	fmt.Println(quote)
	// }
	// return

	{
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		cfg, err := meteoraDammV2.GetConfig(ctx1, solana.MustPublicKeyFromBase58("82p7sVzQWZfCrmStPhsG8BYKwheQkUiXSs2wiqdhwNxr"))
		if err != nil {
			t.Fatal("GetConfig fail", err)
		}
		fmt.Println("===========================")
		fmt.Println("print config info")
		fmt.Println("cfg.Index", cfg.Index)
		fmt.Println("cfg.VaultConfigKey", cfg.VaultConfigKey)
		fmt.Println("cfg.PoolFees", cfg.PoolFees)
		fmt.Println("cfg.PoolFees.BaseFeeConfig", cfg.PoolFees.BaseFee)
		fmt.Println("cfg.PoolFees.DynamicFeeConfig", cfg.PoolFees.DynamicFee)
		fmt.Println("cfg.SqrtMinPrice", cfg.SqrtMinPrice)
		fmt.Println("cfg.SqrtMaxPrice", cfg.SqrtMaxPrice)
		fmt.Println("cfg.ConfigType", cfg.ConfigType)
		fmt.Println("cfg.PoolCreatorAuthority", cfg.PoolCreatorAuthority)
		fmt.Println("===========================")
	}

	{
		mintWallet := solana.NewWallet()
		baseMint := mintWallet.PublicKey()
		fmt.Printf("new token mint address:%s(%s)\n", baseMint, mintWallet.PrivateKey)

		testCreateTokenMint(t, ctx, rpcClient, wsClient, mintWallet, payer, poolCreator)

		{
			fmt.Println("ready CreatePool")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			baseAmount := big.NewInt(1_000_000)
			quoteAmount := big.NewInt(1) // SOL
			sig, _, _, err := meteoraDammV2.CreatePool(
				ctx1,
				payer,
				0,
				1, // 1 base token = 1 quote token
				baseMint,
				solana.WrappedSol,
				baseAmount,
				quoteAmount,
				nil,
				true,
			)
			if err != nil {
				t.Fatal("meteoraDammV2.CreatePool fail", err)
			}
			fmt.Println("success CreatePool sig:", sig)
		}
		testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)
	}

	{
		poolCreatorAuthority := solana.NewWallet()
		fmt.Printf("poolCreatorAuthority address:%s(%s)\n", poolCreatorAuthority.PublicKey(), poolCreatorAuthority.PrivateKey)

		mintWallet := solana.NewWallet()
		mintWallet = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("5SBchR7A6ysnjDdMeaLoxPNpn1wFL1xCCzJAdCBAr3cLesrPru3HprVwTRrtyGp9DuUpQksUdaAtWSanFWVK8QsS")}
		baseMint := mintWallet.PublicKey()
		fmt.Printf("new token mint address:%s(%s)\n", baseMint, mintWallet.PrivateKey)

		// testCreateTokenMint(t, ctx, rpcClient, wsClient, mintWallet, payer, poolCreator)

		{
			fmt.Println("准备创建 CreateCustomizablePoolWithDynamicConfig")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			baseAmount := big.NewInt(1_000_000)
			quoteAmount := big.NewInt(1) // SOL
			sig, _, _, err := meteoraDammV2.CreateCustomizablePoolWithDynamicConfig(
				ctx1,
				payer,
				1,
				poolCreatorAuthority,
				1, // 1 base token = 1 quote token
				baseMint,
				solana.WrappedSol,
				baseAmount,
				quoteAmount,
				false,
				cp_amm.ActivationTypeTimestamp,
				cp_amm.CollectFeeModeBothToken,
				nil,
				true,
				5000, // 50%
				25,   // 0.25%
				cp_amm.FeeSchedulerModeExponential,
				60,   // 60 peridos
				3600, // 60 * 60
				true,
			)
			if err != nil {
				t.Fatal("meteoraDammV2.CreateCustomizablePoolWithDynamicConfig fail", err)
			}
			fmt.Println("创建完成 CreateCustomizablePoolWithDynamicConfig sig:", sig)
		}
		testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)
	}

	mintWallet := solana.NewWallet()
	// mintWallet = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("2M7C27qQShC6kTWUogfBAXW9SGyZaM7LBLvVDTQNJSe1r4sKJu8EnDzi3FDo7FTeX9n1dTtgZWLKo4CW7XjCj3C7")}
	baseMint := mintWallet.PublicKey()
	fmt.Printf("new token mint address:%s(%s)\n", baseMint, mintWallet.PrivateKey)

	{
		testCreateTokenMint(t, ctx, rpcClient, wsClient, mintWallet, payer, poolPartner)

		{
			fmt.Println("ready CreateCustomizablePool")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			baseAmount := big.NewInt(1_000_000)
			quoteAmount := big.NewInt(1) // SOL
			sig, _, _, err := meteoraDammV2.CreateCustomizablePool(
				ctx1,
				payer,
				1, // 1 base token = 1 quote token
				baseMint,
				solana.WrappedSol,
				baseAmount,
				quoteAmount,
				false,
				cp_amm.ActivationTypeTimestamp,
				cp_amm.CollectFeeModeBothToken,
				nil,
				true,
				5000, // 50%
				25,   // 0.25%
				cp_amm.FeeSchedulerModeExponential,
				60,   // 60 peridos
				3600, // 60 * 60
				true,
			)
			if err != nil {
				t.Fatal("meteoraDammV2.CreateCustomizablePool fail", err)
			}
			fmt.Println("success CreateCustomizablePool sig:", sig)
		}
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		var p solana.PublicKey
		{
			fmt.Println("ready CreatePosition")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, position, err := meteoraDammV2.CreatePosition(ctx1, payer, poolPartner, baseMint)
			if err != nil {
				t.Fatal("meteoraDammV2.CreatePosition fail", err)
			}
			fmt.Println("success CreatePosition sig:", sig)

			p = position.PublicKey()
		}

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			list, err := meteoraDammV2.GetUserPositionsByUser(ctx1, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("meteoraDammV2.GetUserPositionsByUser fail", err)
			}
			for _, v := range list {
				if p.Equals(v.PositionState.NftMint) {
					fmt.Println("position mint:", v.PositionState.NftMint)
				}
			}
		}
	}

	{
		fmt.Println("ready AddPositionLiquidity")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		amountIn := new(big.Int).SetUint64(10_000_000)
		quote, virtualPool, err := meteoraDammV2.GetDepositQuote(ctx1, baseMint, true, amountIn)
		if err != nil {
			t.Fatal("meteoraDammV2.GetDepositQuote fail", err)
		}
		sig, err := meteoraDammV2.AddPositionLiquidity(ctx1, payer, poolPartner, virtualPool, true, amountIn, quote.LiquidityDelta, quote.OutputAmount)
		if err != nil {
			t.Fatal("meteoraDammV2.AddPositionLiquidity fail", err)
		}
		fmt.Println("success AddPositionLiquidity sig:", sig)
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		fmt.Println("try to buy token 0.2*1e9 address:", baseMint)
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		amountIn := new(big.Int).SetUint64(uint64(0.2 * 1e9))
		quote, poolState, err := meteoraDammV2.BuyQuote(ctx1, baseMint, amountIn, 250)
		if err != nil {
			t.Fatal("cpAmm.BuyQuote() fail", err)
		}
		fmt.Println("minOutAmount", quote)
		// return
		fmt.Printf("buy token address:%s expected:%v minimum:%v\n", baseMint, quote.SwapOutAmount, quote.MinSwapOutAmount)
		sig, err := meteoraDammV2.Buy(ctx1, ownerWallet, nil, poolState.Address, poolState.Pool, amountIn, quote.MinSwapOutAmount)
		if err != nil {
			t.Fatal("cpAmm.Buy() fail", err)
		}
		fmt.Println("buy token completed Success sig:", sig)
		{
			_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
			if err != nil {
				t.Fatal("testMintBalance() fail")
			}
		}
	}
	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		var balance uint64
		{
			bal, err := testMintBalance(ctx, rpcClient, owner, baseMint)
			if err != nil {
				t.Fatal("testMintBalance() fail")
			}
			balance = bal
		}

		{
			fmt.Println("try to sell all tokens address:", baseMint)
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			amountIn := new(big.Int).SetUint64(balance) // uint64(100000 * 1e9)

			quote, poolState, err := meteoraDammV2.SellQuote(ctx1, baseMint, amountIn, 250)
			if err != nil {
				t.Fatal("cpAmm.SellQuote fail", err)
			}

			sig, err := meteoraDammV2.Sell(ctx1, ownerWallet, nil, poolState.Address, poolState.Pool, amountIn, quote.MinSwapOutAmount)
			if err != nil {
				t.Fatal("cpAmm.Sell fail", err)
			}
			fmt.Println("sell token completed Success sig:", sig)
		}

		{
			_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
			if err != nil {
				t.Fatal("testMintBalance() fail")
			}
		}
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		fmt.Println("ready RemovePositionLiquidity")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()

		liquidityDelta, position, err := meteoraDammV2.GetPositionLiquidity(ctx1, baseMint, poolPartner.PublicKey())
		if err != nil {
			t.Fatal("meteoraDammV2.GetPositionLiquidity fail", err)
		}

		var xx []*dammV2.Vesting
		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			vestings, err := meteoraDammV2.GetVestingsByPosition(ctx1, position.Position)
			if err != nil {
				t.Fatal("meteoraDammV2.GetVestingsByPosition fail", err)
			}

			xx = vestings
		}

		liquidityDelta = new(big.Int).Div(liquidityDelta, big.NewInt(2))

		quote, virtualPool, err := meteoraDammV2.GetWithdrawQuote(ctx1, baseMint, liquidityDelta)
		if err != nil {
			t.Fatal("meteoraDammV2.GetWithdrawQuote fail", err)
		}
		sig, err := meteoraDammV2.RemovePositionLiquidity(ctx1, payer, poolPartner, virtualPool, liquidityDelta, quote.OutBaseAmount, quote.OutQuoteAmount, xx)
		if err != nil {
			t.Fatal("meteoraDammV2.RemovePositionLiquidity fail", err)
		}
		fmt.Println("success RemovePositionLiquidity sig:", sig)
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		fmt.Println("ready RemoveAllLiquidity")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()

		liquidityDelta, position, err := meteoraDammV2.GetPositionLiquidity(ctx1, baseMint, poolPartner.PublicKey())
		if err != nil {
			t.Fatal("meteoraDammV2.GetPositionLiquidity fail", err)
		}

		fmt.Println("liquidityDelta", liquidityDelta)

		var xx []*dammV2.Vesting
		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			vestings, err := meteoraDammV2.GetVestingsByPosition(ctx1, position.Position)
			if err != nil {
				t.Fatal("meteoraDammV2.GetVestingsByPosition fail", err)
			}
			xx = vestings
		}

		sig, err := meteoraDammV2.RemoveAllLiquidity(ctx1, payer, poolPartner, baseMint, xx)
		if err != nil {
			t.Fatal("meteoraDammV2.RemoveAllLiquidity fail", err)
		}
		fmt.Println("success RemoveAllLiquidity sig:", sig)
	}

	{
		pp := testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)
		var baseFee, quoteFee uint64
		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			liquidityDelta, position, err := meteoraDammV2.GetPositionLiquidity(ctx1, baseMint, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("meteoraDammV2.GetPositionLiquidity fail", err)
			}
			fmt.Println("liquidityDelta", liquidityDelta)

			baseFee, quoteFee = meteoraDammV2.GetUnclaimedFee(pp.Pool, position.PositionState)
			fmt.Println(meteoraDammV2.GetUnclaimedRewards(pp.Pool, position.PositionState))
		}

		if baseFee > 0 || quoteFee > 0 {
			fmt.Println("claim fee")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDammV2.ClaimPositionFee(ctx1, payer, poolPartner, baseMint)
			if err != nil {
				t.Fatal("ClaimPositionFee() fail", err)
			}

			fmt.Println("claim fee completed sig:", sig)
		}
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		fmt.Println("ready ClosePosition")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		sig, err := meteoraDammV2.ClosePosition(ctx1, payer, poolPartner, baseMint)
		if err != nil {
			t.Fatal("meteoraDammV2.ClosePosition fail", err)
		}
		fmt.Println("success ClosePosition sig:", sig)
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		fmt.Println("ready CreatePosition")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		sig, _, err := meteoraDammV2.CreatePosition(ctx1, payer, poolPartner, baseMint)
		if err != nil {
			t.Fatal("meteoraDammV2.CreatePosition fail", err)
		}
		fmt.Println("success CreatePosition sig:", sig)
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		fmt.Println("ready AddPositionLiquidity")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		amountIn := new(big.Int).SetUint64(100_000)
		quote, virtualPool, err := meteoraDammV2.GetDepositQuote(ctx1, baseMint, true, amountIn)
		if err != nil {
			t.Fatal("meteoraDammV2.GetDepositQuote fail", err)
		}
		fmt.Println("quote", quote)
		sig, err := meteoraDammV2.AddPositionLiquidity(ctx1, payer, poolPartner, virtualPool, true, amountIn, quote.LiquidityDelta, quote.OutputAmount)
		if err != nil {
			t.Fatal("meteoraDammV2.AddPositionLiquidity fail", err)
		}
		fmt.Println("success AddPositionLiquidity sig:", sig)
	}

	{
		var positionNft *solana.Wallet
		{
			fmt.Println("ready CreatePosition")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, position, err := meteoraDammV2.CreatePosition(ctx1, payer, poolCreator, baseMint)
			if err != nil {
				t.Fatal("meteoraDammV2.CreatePosition fail", err)
			}
			positionNft = position
			fmt.Println("success CreatePosition sig:", sig)
		}

		{
			fmt.Println("ready SplitPosition")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDammV2.SplitPosition(
				ctx1,
				payer,
				poolPartner,
				poolCreator,
				positionNft,
				baseMint,
				50,
				0,
				50,
				50,
				50,
				50,
			)
			if err != nil {
				t.Fatal("meteoraDammV2.SplitPosition fail", err)
			}
			fmt.Println("success SplitPosition sig:", sig)
		}
	}

	{
		var liquidityDelta *big.Int
		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			liquidityDelta, _, err = meteoraDammV2.GetPositionLiquidity(ctx1, baseMint, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("meteoraDammV2.GetPositionLiquidity fail", err)
			}
			fmt.Println("liquidityDelta", liquidityDelta)
		}

		{
			fmt.Println("ready LockPosition")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			numberOfPeriod := uint16(10)

			liquidityToLock := new(big.Int).Div(liquidityDelta, big.NewInt(2))

			cliffUnlockLiquidity := new(big.Int).Div(liquidityToLock, big.NewInt(2))
			liquidityPerPeriod := new(big.Int).Div(new(big.Int).Sub(liquidityToLock, cliffUnlockLiquidity), new(big.Int).SetUint64(uint64(numberOfPeriod)))
			loss := new(big.Int).Sub(liquidityToLock, new(big.Int).Add(cliffUnlockLiquidity, new(big.Int).Mul(liquidityPerPeriod, new(big.Int).SetUint64(uint64(numberOfPeriod)))))

			cliffUnlockLiquidity = new(big.Int).Add(cliffUnlockLiquidity, loss)

			vesting := solana.NewWallet()

			sig, err := meteoraDammV2.LockPosition(ctx1, payer, poolPartner, baseMint, nil, 1, cliffUnlockLiquidity, liquidityPerPeriod, numberOfPeriod, vesting)
			if err != nil {
				t.Fatal("meteoraDammV2.LockPosition fail", err)
			}
			fmt.Println("success LockPosition sig:", sig)
		}
		{
			fmt.Println("ready PermanentLockPosition")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			liquidityToLock := new(big.Int).Div(liquidityDelta, big.NewInt(2))
			sig, err := meteoraDammV2.PermanentLockPosition(ctx1, poolPartner, baseMint, liquidityToLock)
			if err != nil {
				t.Fatal("meteoraDammV2.PermanentLockPosition fail", err)
			}
			fmt.Println("success PermanentLockPosition sig:", sig)
		}
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)
}

func testCpAmmPoolCheck(t *testing.T, ctx context.Context, cpamm *dammV2.DammV2, baseMint solana.PublicKey) *dammV2.Pool {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	pool, err := cpamm.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail", err)
	}

	if pool == nil {
		fmt.Println("pool does not exist:", baseMint)
		return nil
	}
	fmt.Println("===========================")
	fmt.Println("print pool info")
	fmt.Println("dammv2.PoolAddress", pool.Address)
	fmt.Println("dammv2.TokenAMint", pool.TokenAMint)
	fmt.Println("dammv2.TokenBMint", pool.TokenBMint)
	fmt.Println("===========================")

	return pool
}

func testCreateTokenMint(t *testing.T, ctx context.Context, rpcClient *rpc.Client, wsClient *ws.Client, mint, payer, partner *solana.Wallet) {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	mintAmount := uint64(100_000_000 * 1e9)

	lamports, err := rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		uint64(token.MINT_SIZE),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		t.Fatal("rpcClient.GetMinimumBalanceForRentExemption fail", err)
	}

	createIx := system.NewCreateAccountInstruction(
		lamports,
		token.MINT_SIZE,
		solana.TokenProgramID,
		payer.PublicKey(),
		mint.PublicKey(),
	).Build()

	initializeIx := token.NewInitializeMint2InstructionBuilder().
		SetDecimals(9).
		SetMintAuthority(payer.PublicKey()).
		SetMintAccount(mint.PublicKey()).Build()

	ata, _, _ := solana.FindAssociatedTokenAddress(payer.PublicKey(), mint.PublicKey())

	ix := associatedtokenaccount.NewCreateInstruction(
		payer.PublicKey(), payer.PublicKey(), mint.PublicKey(),
	).Build()

	mintIx := token.NewMintToInstruction(
		mintAmount, // 数量 (1000 token, decimals=9)
		mint.PublicKey(),
		ata,
		payer.PublicKey(),
		nil,
	).Build()

	ata1, _, _ := solana.FindAssociatedTokenAddress(partner.PublicKey(), mint.PublicKey())

	ix1 := associatedtokenaccount.NewCreateInstruction(
		payer.PublicKey(), partner.PublicKey(), mint.PublicKey(),
	).Build()

	mintTx1 := token.NewMintToInstruction(
		10_000_000*1e9,
		mint.PublicKey(),
		ata1,
		payer.PublicKey(),
		nil,
	).Build()

	sig, err := solanago.SendTransaction(
		ctx1,
		rpcClient,
		wsClient,
		[]solana.Instruction{createIx, initializeIx, ix, ix1, mintIx, mintTx1},
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(mint.PublicKey()):
				return &mint.PrivateKey
			case key.Equals(partner.PublicKey()):
				return &partner.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		t.Fatal("solanago.SendTransaction fail", err)
	}
	fmt.Println("testCreateTokenMint success", sig.String())
}
