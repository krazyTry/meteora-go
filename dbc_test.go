package meteora

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/krazyTry/meteora-go/u128"

	dammV2 "github.com/krazyTry/meteora-go/damm.v2"
	"github.com/krazyTry/meteora-go/dbc"

	"github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	"github.com/shopspring/decimal"
)

func TestDbc(t *testing.T) {

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

	config := solana.NewWallet()
	// config = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("3nUKWprxN5qtQ1EqboXgaZncHPZTUT8y1EdnhSYfWRYev7BwdmUJhLjYdnkRouivGGYt1gu7jjPv8Qa1x4up9ocL")}
	fmt.Printf("config address:%s(%s)\n", config.PublicKey(), config.PrivateKey)

	poolCreator := solana.NewWallet()
	// poolCreator = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("2sDd4jKUwcfUVxkhfJY9JrKCJn2tYh9pBDwPQEwF41xN13hzMaL1iJ3QUgiPmMFez44LUUSAgXGZ7yCTd9kPxJY4")}
	fmt.Printf("poolCreator address:%s(%s)\n", poolCreator.PublicKey(), poolCreator.PrivateKey)

	poolPartner := solana.NewWallet()
	// poolPartner = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("66Sq63JTSnqhECNYciiL3yUv6LErRED3j69ahs2RpTrqjgdbYu9gHgg3bjoKwYYNbq2wrcpN7w5R73ZbdCb7JtMJ")}
	fmt.Printf("poolPartner address:%s(%s)\n", poolPartner.PublicKey(), poolPartner.PrivateKey)

	leftoverReceiver := solana.NewWallet()
	// leftoverReceiver = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("43twn5bdq43h5QeSaJWRy41HjHm2s1y1pqn4EptD9uMJD6co6mBgkwiF2QB5hNaYETUvZjZweRiL33CvWQrk2UYR")}
	fmt.Printf("leftoverReceiver address:%s(%s)\n", leftoverReceiver.PublicKey(), leftoverReceiver.PrivateKey)

	{
		fmt.Println("transfer a little sol to leftoverReceiver")
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		if _, err = testTransferSOL(ctx1, rpcClient, wsClient, payer, leftoverReceiver.PublicKey(), 0.1*1e9); err != nil {
			t.Fatal("testTransferSOL fail", err)
		}
	}
	fmt.Printf("\n\n")

	if err = dbc.Init(); err != nil {
		t.Fatal("dbc.Init() fail", err)
	}

	meteoraDBC, err := dbc.NewDBC(rpcClient, config, poolCreator, poolPartner, leftoverReceiver)
	if err != nil {
		t.Fatal("NewMeteoraDBC() fail", err)
	}

	// Check if the configuration creation function is ok
	testDBCBuildCurveCheck(t)

	{
		// Initialize the configuration, create it if it does not exist, otherwise return
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		fmt.Println("try to initialize config")
		if err = meteoraDBC.InitConfig(ctx1, wsClient, payer, solana.WrappedSol, testDBCGenConfig()); err != nil {
			t.Fatal("dbc.InitConfig() fail", err)
		}
		fmt.Println("initialization config completed")
		// Check if the configuration matches
		testDBCConfigCheck(t, ctx, meteoraDBC, solana.WrappedSol, config.PublicKey())
	}

	name := "MeteoraGoTest"
	symbol := "METAGOTEST"
	uri := "https://launch.meteora.ag/icons/logo.svg"

	{
		mintWallet := solana.NewWallet()
		baseMint := mintWallet.PublicKey()
		fmt.Printf("new token mint address:%s(%s)\n", baseMint, mintWallet.PrivateKey)

		{
			_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
			if err != nil {
				t.Fatal("testMintBalance() fail")
			}
		}

		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()

		fmt.Println("try to create token mint address:", baseMint)
		amountIn := new(big.Int).SetUint64(uint64(0.1 * 1e9))
		sig, err := meteoraDBC.CreatePoolWithFirstBuy(ctx1, wsClient, ownerWallet, mintWallet, name, symbol, uri, amountIn, 250)
		if err != nil {
			t.Fatal("dbc.CreatePoolWithFirstBuy fail", err)
		}
		fmt.Println("create token and buy 0.1*1e9 Success sig:", sig)

		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			pools, err := meteoraDBC.GetPoolsByCreator(ctx1)
			if err != nil {
				t.Fatal("dbc.GetPoolsByCreator() fail")
			}

			for _, pool := range pools {
				if pool.BaseMint != baseMint {
					t.Fatal("dbc.GetPoolsByCreator() fail")
				}
			}
		}

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			pools, err := meteoraDBC.GetPoolsByConfig(ctx1)
			if err != nil {
				t.Fatal("dbc.GetPoolsByCreator() fail")
			}
			for _, pool := range pools {
				if pool.BaseMint != baseMint {
					t.Fatal("dbc.GetPoolsByCreator() fail")
				}
			}
		}

		var balance uint64
		{
			bal, err := testMintBalance(ctx, rpcClient, owner, baseMint)
			if err != nil {
				t.Fatal("testMintBalance() fail")
			}
			balance = bal
		}

		{
			fmt.Println("try to sell token address:", baseMint)
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			amountIn := new(big.Int).SetUint64(balance)
			quote, poolState, configState, currentPoint, err := meteoraDBC.SellQuote(ctx1, baseMint, amountIn, 250, false)
			if err != nil {
				t.Fatal("testMintBalance() fail")
			}
			sig, err := meteoraDBC.Sell(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.VirtualPool, configState, amountIn, quote.MinimumAmountOut, currentPoint)
			if err != nil {
				t.Fatal("dbc.Sell() fail", err)
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

	// Generate new token information
	mintWallet := solana.NewWallet()
	// mintWallet = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("5242H5ijH7p754wzbPSqxrS9m9xvRECnW1LvNzaDaqNGX8z61ecyxAo5G8vkM1WidkUE5JyjkKPeih7DutcdSCdG")}
	baseMint := mintWallet.PublicKey()
	fmt.Printf("new token mint address:%s(%s)\n", baseMint, mintWallet.PrivateKey)

	{
		fmt.Println("try to create token mint address:", baseMint)
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		sig, err := meteoraDBC.CreatePool(ctx1, wsClient, payer, mintWallet, name, symbol, uri)
		if err != nil {
			t.Fatal("dbc.CreatePool() fail", err)
		}
		fmt.Println("create token success Success sig:", sig)

		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)
	}

	{
		_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("testMintBalance() fail")
		}
	}

	{
		fmt.Println("try to buy token 0.4*1e9 address:", baseMint)
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		amountIn := new(big.Int).SetUint64(uint64(0.4 * 1e9))
		minOutAmount, poolState, configState, currentPoint, err := meteoraDBC.BuyQuote(ctx1, baseMint, amountIn, 250, false)
		if err != nil {
			t.Fatal("dbc.BuyQuote() fail", err)
		}
		fmt.Printf("buy token address:%s expected:%v minimum:%v\n", baseMint, minOutAmount.AmountOut, minOutAmount.MinimumAmountOut)
		sig, err := meteoraDBC.Buy(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.VirtualPool, configState, amountIn, minOutAmount.MinimumAmountOut, currentPoint)
		if err != nil {
			t.Fatal("dbc.Buy() fail", err)
		}
		fmt.Println("buy token completed Success sig:", sig)

		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)
	}

	var balance uint64
	{
		bal, err := testMintBalance(ctx, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("testMintBalance() fail", err)
		}
		balance = bal
	}

	{
		fmt.Println("try to sell half of tokens address:", baseMint)
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()

		amountIn := new(big.Int).SetUint64(balance / 2) // uint64(100000 * 1e9)
		quote, poolState, configState, currentPoint, err := meteoraDBC.SellQuote(ctx1, baseMint, amountIn, 250, false)
		if err != nil {
			t.Fatal("dbc.SellQuote() fail", err)
		}
		sig, err := meteoraDBC.Sell(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.VirtualPool, configState, amountIn, quote.MinimumAmountOut, currentPoint)
		if err != nil {
			t.Fatal("dbc.Sell() fail", err)
		}
		fmt.Println("sell token completed Success sig:", sig)

		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)
	}

	{
		_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("testMintBalance() fail", err)
		}
	}

	{
		fmt.Println("try to buy token 0.2*1e9 address:", baseMint)
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		amountIn := new(big.Int).SetUint64(uint64(0.2 * 1e9))
		minOutAmount, poolState, configState, currentPoint, err := meteoraDBC.BuyQuote(ctx1, baseMint, amountIn, 250, false)
		if err != nil {
			t.Fatal("dbc.BuyQuote() fail", err)
		}
		fmt.Printf("buy token address:%s expected:%v minimum:%v\n", baseMint, minOutAmount.AmountOut, minOutAmount.MinimumAmountOut)
		sig, err := meteoraDBC.Buy(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.VirtualPool, configState, amountIn, minOutAmount.MinimumAmountOut, currentPoint)
		if err != nil {
			t.Fatal("dbc.Buy() fail", err)
		}
		fmt.Println("buy token completed Success sig:", sig)
	}

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
		quote, poolState, configState, currentPoint, err := meteoraDBC.SellQuote(ctx1, baseMint, amountIn, 250, false)
		if err != nil {
			t.Fatal("dbc.SellQuote() fail", err)
		}
		sig, err := meteoraDBC.Sell(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.VirtualPool, configState, amountIn, quote.MinimumAmountOut, currentPoint)
		if err != nil {
			t.Fatal("dbc.Sell() fail", err)
		}
		fmt.Println("sell token completed Success sig:", sig)
	}

	{
		_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
		if err != nil {
			t.Fatal("testMintBalance() fail")
		}
	}

	pool := testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)

	if pool.CreatorQuoteFee > 0 {

		{
			_, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
			if err != nil {
				t.Fatal("testBalance() fail")
			}
		}

		{
			fmt.Println("try to claim CreatorQuoteFee")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDBC.ClaimCreatorTradingFee(ctx1, wsClient, payer, baseMint, false, pool.CreatorQuoteFee)
			if err != nil {
				t.Fatal("dbc.ClaimCreatorTradingFee() fail", err)
			}
			fmt.Println("claim CreatorQuoteFee completed sig:", sig)
		}

		var balance uint64
		{
			lamports, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
			if err != nil {
				t.Fatal("testBalance() fail")
			}
			balance = lamports
		}

		{
			fmt.Println("try to transfer sol to owner")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			if _, err = testTransferSOL(ctx1, rpcClient, wsClient, poolCreator, owner, balance); err != nil {
				t.Fatal("testTransferSOL fail")
			}
			fmt.Println("transfer sol completed")
		}

		{
			_, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
			if err != nil {
				t.Fatal("testBalance() fail")
			}
		}

	}

	if pool.PartnerQuoteFee > 0 {
		{
			_, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("testBalance() fail")
			}
		}

		{
			fmt.Println("try to claim PartnerQuoteFee")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDBC.ClaimPartnerTradingFee(ctx1, wsClient, payer, baseMint, false, pool.PartnerQuoteFee)
			if err != nil {
				t.Fatal("dbc.ClaimPartnerTradingFee() fail", err)
			}
			fmt.Println("claim PartnerQuoteFee completed sig:", sig)
		}

		var balance uint64
		{
			lamports, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("testBalance() fail")
			}
			balance = lamports
		}
		{
			fmt.Println("try to transfer sol to owner")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			if _, err = testTransferSOL(ctx1, rpcClient, wsClient, poolPartner, owner, balance); err != nil {
				t.Fatal("testTransferSOL fail", err)
			}
			fmt.Println("transfer sol completed")
		}
		{
			_, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("testBalance() fail")
			}
		}
	}

	{
		fmt.Println("prepare dbc -> dammv2 mint address:", baseMint)

		{
			fmt.Println("try to buy token 1*1e9 address:", baseMint)
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			amountIn := new(big.Int).SetUint64(uint64(1 * 1e9))
			minOutAmount, poolState, configState, currentPoint, err := meteoraDBC.BuyQuote(ctx1, baseMint, amountIn, 250, false)
			if err != nil {
				t.Fatal("dbc.BuyQuote() fail", err)
			}
			fmt.Printf("buy token address:%s expected:%v minimum:%v\n", baseMint, minOutAmount.AmountOut, minOutAmount.MinimumAmountOut)
			sig, err := meteoraDBC.Buy(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.VirtualPool, configState, amountIn, minOutAmount.MinimumAmountOut, currentPoint)
			if err != nil {
				t.Fatal("dbc.Buy() fail", err)
			}
			fmt.Println("buy token completed Success sig:", sig)
		}

		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)

		fmt.Println("try dbc -> dammv2 mint address:", baseMint)
		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			sig, err := meteoraDBC.MigrationDammV2CreateMetadata(ctx1, wsClient, payer, baseMint)
			if err != nil {
				t.Fatal("dbc.MigrationDammV2CreateMetadata fail", err)
			}
			fmt.Println("MigrationDammV2CreateMetadata Success sig:", sig)
		}
		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			sig, err := meteoraDBC.CreateLocker(ctx1, wsClient, payer, baseMint)
			if err != nil {
				t.Fatal("dbc.CreateLocker fail", err)
			}
			fmt.Println("CreateLocker Success sig:", sig)
		}
		testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			sig, _, _, err := meteoraDBC.MigrationDammV2(ctx1, wsClient, payer, baseMint)
			if err != nil {
				t.Fatal("dbc.MigrationDammV2 fail", err)
			}
			fmt.Println("MigrationDammV2 Success sig:", sig)
		}

		fmt.Println("dbc -> dammv2 Success")
	}

	{
		fmt.Println("dbc closing work")

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDBC.WithdrawCreatorSurplus(ctx1, wsClient, payer, baseMint)
			if err != nil {
				t.Fatal("dbc.WithdrawCreatorSurplus fail", err)
			}
			fmt.Println("WithdrawCreatorSurplus completed sig:", sig)
		}

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDBC.WithdrawPartnerSurplus(ctx1, wsClient, payer, baseMint)
			if err != nil {
				t.Fatal("dbc.WithdrawPartnerSurplus fail", err)
			}
			fmt.Println("WithdrawPartnerSurplus completed sig:", sig)
		}

		{
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()
			sig, err := meteoraDBC.WithdrawLeftover(ctx1, wsClient, payer, baseMint)
			if err != nil {
				t.Fatal("dbc.WithdrawLeftover fail", err)
			}
			fmt.Println("WithdrawLeftover completed sig:", sig)
		}

		pool := testDBCPoolCheck(t, ctx, meteoraDBC, baseMint)

		if pool.CreatorQuoteFee > 0 {

			{
				_, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
			}

			{
				fmt.Println("try to claim CreatorQuoteFee")
				ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
				defer cancel1()
				sig, err := meteoraDBC.ClaimCreatorTradingFee(ctx1, wsClient, payer, baseMint, false, pool.CreatorQuoteFee)
				if err != nil {
					t.Fatal("dbc.ClaimCreatorTradingFee() fail", err)
				}
				fmt.Println("claim CreatorQuoteFee completed sig:", sig)
			}

			var balance uint64
			{
				lamports, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
				balance = lamports
			}

			{
				fmt.Println("try to transfer sol to owner")
				ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
				defer cancel1()
				if _, err = testTransferSOL(ctx1, rpcClient, wsClient, poolCreator, owner, balance); err != nil {
					t.Fatal("testTransferSOL fail", err)
				}
				fmt.Println("transfer sol completed")
			}

			{
				_, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
			}

		}

		if pool.PartnerQuoteFee > 0 {
			{
				_, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
			}

			{
				fmt.Println("try to claim PartnerQuoteFee")
				ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
				defer cancel1()
				sig, err := meteoraDBC.ClaimPartnerTradingFee(ctx1, wsClient, payer, baseMint, false, pool.PartnerQuoteFee)
				if err != nil {
					t.Fatal("dbc.ClaimPartnerTradingFee() fail", err)
				}
				fmt.Println("claim PartnerQuoteFee completed sig:", sig)
			}

			var balance uint64
			{
				lamports, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
				balance = lamports
			}
			{
				fmt.Println("try to transfer sol to owner")
				ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
				defer cancel1()
				if _, err = testTransferSOL(ctx1, rpcClient, wsClient, poolPartner, owner, balance); err != nil {
					t.Fatal("testTransferSOL fail", err)
				}
				fmt.Println("transfer sol completed")
			}
			{
				_, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
			}
		}
		fmt.Println("claim fee completed")

		fmt.Println("dbc closing work completed 66%")
	}

	if err = dammV2.Init(); err != nil {
		t.Fatal("dammV2.Init() fail", err)
	}

	meteoraDammV2, err := dammV2.NewDammV2(rpcClient, poolCreator)
	if err != nil {
		t.Fatal("NewDammV2() fail", err)
	}

	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

	{
		{
			var balance uint64
			{
				bal, err := testMintBalance(ctx, rpcClient, leftoverReceiver.PublicKey(), baseMint)
				if err != nil {
					t.Fatal("testMintBalance() fail", err)
				}
				balance = bal
			}

			{
				fmt.Println("try to sell all surplus address:", baseMint)
				ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
				defer cancel1()
				amountIn := new(big.Int).SetUint64(balance) // uint64(100000 * 1e9)

				quote, poolState, err := meteoraDammV2.SellQuote(ctx1, baseMint, amountIn, 250)
				if err != nil {
					t.Fatal("cpAmm.BuyQuote fail", err)
				}
				fmt.Println("poolState.Address", poolState.Address)

				fmt.Println("quote.SwapInAmount", quote.SwapInAmount)
				fmt.Println("quote.SwapOutAmount", quote.SwapOutAmount)
				fmt.Println("quote.MinSwapOutAmount", quote.MinSwapOutAmount)

				sig, err := meteoraDammV2.Sell(ctx1, wsClient, leftoverReceiver, nil, poolState.Address, poolState.Pool, amountIn, quote.MinSwapOutAmount)
				if err != nil {
					t.Fatal("meteoraDammV2.Sell fail", err)
				}
				fmt.Println("sell surplus completed Success sig:", sig)
			}

			{
				_, err := testMintBalance(ctx, rpcClient, owner, baseMint)
				if err != nil {
					t.Fatal("testMintBalance() fail")
				}
			}
		}

		{
			var balance uint64
			{
				lamports, err := testBalance(ctx, rpcClient, leftoverReceiver.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
				balance = lamports
			}
			{
				fmt.Println("try to transfer sol to owner")
				ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
				defer cancel1()
				if _, err = testTransferSOL(ctx1, rpcClient, wsClient, leftoverReceiver, owner, balance); err != nil {
					t.Fatal("testTransferSOL fail", err)
				}
				fmt.Println("transfer sol completed")
			}
			{
				_, err := testBalance(ctx, rpcClient, leftoverReceiver.PublicKey())
				if err != nil {
					t.Fatal("testBalance() fail")
				}
			}
		}
		fmt.Println("dbc closing work completed 100%")
	}

	{
		fmt.Println("try to buy token 0.2*1e9 address:", baseMint)
		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
		defer cancel1()
		amountIn := new(big.Int).SetUint64(uint64(0.2 * 1e9))
		minOutAmount, poolState, err := meteoraDammV2.BuyQuote(ctx1, baseMint, amountIn, 250)
		if err != nil {
			t.Fatal("cpAmm.BuyQuote() fail", err)
		}
		fmt.Println("poolState", poolState.Address)
		fmt.Printf("buy token address:%s expected:%v minimum:%v\n", baseMint, minOutAmount.SwapOutAmount, minOutAmount.MinSwapOutAmount)
		sig, err := meteoraDammV2.Buy(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.Pool, amountIn, minOutAmount.MinSwapOutAmount)
		if err != nil {
			t.Fatal("cpAmm.Buy() fail", err)
		}
		fmt.Println("buy token completed Success sig:", sig)
	}

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
				t.Fatal("cpAmm.BuyQuote fail", err)
			}

			sig, err := meteoraDammV2.Sell(ctx1, wsClient, ownerWallet, nil, poolState.Address, poolState.Pool, amountIn, quote.MinSwapOutAmount)
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

	{
		cpamm := testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)

		{
			fmt.Println("try to claim fee creator")
			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			userPositions, err := meteoraDammV2.GetUserPositionByUserAndPoolPDA(ctx1, cpamm.Address, poolCreator.PublicKey())
			if err != nil {
				t.Fatal("GetUserPositionByBaseMint() fail", err)
			}

			if len(userPositions) != 1 {
				for _, v := range userPositions {
					fmt.Println("v.PositionNftAccount:", v.PositionNftAccount)
				}
			}

			baseFee, quoteFee := meteoraDammV2.GetUnclaimedFee(cpamm.Pool, userPositions[0].PositionState)
			fmt.Println("baseFee:", baseFee)
			fmt.Println("quoteFee:", quoteFee)

			if quoteFee > 0 {

				{
					_, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
					if err != nil {
						t.Fatal("testBalance() fail")
					}
				}

				{
					fmt.Println("claim fee creator")
					ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
					defer cancel1()
					sig, err := meteoraDammV2.ClaimPositionFee(ctx1, wsClient, payer, poolCreator, baseMint)
					if err != nil {
						t.Fatal("ClaimPositionFee() fail", err)
					}
					fmt.Println("claim fee completed sig:", sig)
				}

				var balance uint64
				{
					lamports, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
					if err != nil {
						t.Fatal("testBalance() fail")
					}
					balance = lamports
				}
				{
					fmt.Println("try to transfer sol to owner")
					ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
					defer cancel1()
					if _, err = testTransferSOL(ctx1, rpcClient, wsClient, poolCreator, owner, balance); err != nil {
						t.Fatal("testTransferSOL fail", err)
					}
					fmt.Println("transfer sol completed")
				}
				{
					_, err := testBalance(ctx, rpcClient, poolCreator.PublicKey())
					if err != nil {
						t.Fatal("testBalance() fail")
					}
				}

			}
		}

		{
			fmt.Println("try to claim fee partner")

			ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
			defer cancel1()

			userPositions, err := meteoraDammV2.GetUserPositionByUserAndPoolPDA(ctx1, cpamm.Address, poolPartner.PublicKey())
			if err != nil {
				t.Fatal("GetUserPositionByBaseMint() fail", err)
			}

			if len(userPositions) != 1 {
				for _, v := range userPositions {
					fmt.Println("v.PositionNftAccount:", v.PositionNftAccount)
				}
			}

			baseFee, quoteFee := meteoraDammV2.GetUnclaimedFee(cpamm.Pool, userPositions[0].PositionState)
			fmt.Println("baseFee:", baseFee)
			fmt.Println("quoteFee:", quoteFee)

			if quoteFee > 0 {
				{
					_, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
					if err != nil {
						t.Fatal("testBalance() fail")
					}
				}
				{
					fmt.Println("claim fee partner")
					ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
					defer cancel1()
					sig, err := meteoraDammV2.ClaimPositionFee(ctx1, wsClient, payer, poolPartner, baseMint)
					if err != nil {
						t.Fatal("ClaimPositionFee() fail", err)
					}

					fmt.Println("claim fee completed sig:", sig)
				}

				var balance uint64
				{
					lamports, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
					if err != nil {
						t.Fatal("testBalance() fail")
					}
					balance = lamports
				}

				{
					fmt.Println("try to transfer sol to owner")
					ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
					defer cancel1()
					if _, err = testTransferSOL(ctx1, rpcClient, wsClient, poolPartner, owner, balance); err != nil {
						t.Fatal("testTransferSOL fail", err)
					}
					fmt.Println("transfer sol completed")
				}
				{
					_, err := testBalance(ctx, rpcClient, poolPartner.PublicKey())
					if err != nil {
						t.Fatal("testBalance() fail")
					}
				}
			}
		}
	}
}

func testDBCPoolCheck(t *testing.T, ctx context.Context, dbc *dbc.DBC, baseMint solana.PublicKey) *dynamic_bonding_curve.VirtualPool {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	pool, err := dbc.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("dbc.GetPoolByBaseMint() fail")
	}

	if pool == nil {
		fmt.Println("pool not found:", baseMint)
		return nil
	}
	fmt.Println("===========================")
	fmt.Println("print pool info")
	fmt.Println("pool.BaseMint:", pool.BaseMint)
	fmt.Println("pool.MigrationProgress:", pool.MigrationProgress)
	fmt.Println("pool.IsMigrated:", pool.IsMigrated)

	fmt.Println("pool.BaseReserve:", pool.BaseReserve)
	fmt.Println("pool.QuoteReserve:", pool.QuoteReserve)
	fmt.Println("pool.PartnerBaseFee:", pool.PartnerBaseFee)
	fmt.Println("pool.PartnerQuoteFee:", pool.PartnerQuoteFee)
	fmt.Println("pool.CreatorBaseFee:", pool.CreatorBaseFee)
	fmt.Println("pool.CreatorQuoteFee:", pool.CreatorQuoteFee)
	fmt.Println("pool.IsWithdrawLeftover:", pool.IsWithdrawLeftover)
	fmt.Println("pool.MigrationFeeWithdrawStatus:", pool.MigrationFeeWithdrawStatus)
	fmt.Println("pool.CreatorBaseFee:", pool.CreatorBaseFee)
	fmt.Println("pool.CreatorQuoteFee:", pool.CreatorQuoteFee)
	fmt.Println("===========================")

	return pool.VirtualPool
}

func testDBCConfigCheck(t *testing.T, ctx context.Context, dbc *dbc.DBC, quoteMint, address solana.PublicKey) {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	cfg, err := dbc.GetConfig(ctx1, address)
	if err != nil {
		t.Fatal("dbc.GetConfig() fail")
	}

	cfg1 := testDBCGenConfig()

	if cfg.CollectFeeMode != cfg1.CollectFeeMode {
		t.Fatal("cfg.CollectFeeMode != cfg1.CollectFeeMode")
	}
	if cfg.MigrationOption != cfg1.MigrationOption {
		t.Fatal("cfg.MigrationOption != cfg1.MigrationOption")
	}
	if cfg.TokenDecimal != cfg1.TokenDecimal {
		t.Fatal("cfg.TokenDecimal != cfg1.TokenDecimal")
	}
	if cfg.TokenType != cfg1.TokenType {
		t.Fatal("cfg.TokenType != cfg1.TokenType")
	}
	if cfg.QuoteMint != quoteMint {
		t.Fatal("cfg.QuoteMint != quoteMint")
	}
	fmt.Println("===========================")
	fmt.Println("print config info")
	fmt.Println("config.CreatorTradingFeePercentage:", cfg.CreatorTradingFeePercentage)
	fmt.Println("config.MigrationFeePercentage:", cfg.MigrationFeePercentage)
	fmt.Println("config.CreatorMigrationFeePercentage:", cfg.CreatorMigrationFeePercentage)

	fmt.Println("config.PartnerLockedLpPercentage:", cfg.PartnerLockedLpPercentage)
	fmt.Println("config.PartnerLpPercentage:", cfg.PartnerLpPercentage)
	fmt.Println("config.CreatorLockedLpPercentage:", cfg.CreatorLockedLpPercentage)
	fmt.Println("config.CreatorLpPercentage:", cfg.CreatorLpPercentage)
	fmt.Println("===========================")
}

func testDBCBuildCurveCheck(t *testing.T) {
	if _, err := testDBCBuildCurve(); err != nil {
		t.Fatal("testDBCBuildCurve() fail", err)
	}

	if _, err := testDBCBuildCurveWithMarketCap(); err != nil {
		t.Fatal("testDBCBuildCurveWithMarketCap() fail", err)
	}

	if _, err := testDBCBuildCurveWithTwoSegments(); err != nil {
		t.Fatal("testDBCBuildCurveWithTwoSegments() fail", err)
	}

	if _, err := testDBCBuildCurveWithLiquidityWeights(); err != nil {
		t.Fatal("testDBCBuildCurveWithLiquidityWeights() fail", err)
	}

}

func testDBCGenConfig() *dynamic_bonding_curve.ConfigParameters {
	return &dynamic_bonding_curve.ConfigParameters{
		PoolFees: dynamic_bonding_curve.PoolFeeParameters{
			BaseFee: dynamic_bonding_curve.BaseFeeParameters{
				CliffFeeNumerator: 5000 * 100_000, // 50% = 5000*0.01%,
				FirstFactor:       0,
				SecondFactor:      0,
				ThirdFactor:       0,
				BaseFeeMode:       0,
			},
			DynamicFee: &dynamic_bonding_curve.DynamicFeeParameters{
				BinStep:                  1,
				BinStepU128:              u128.GenUint128FromString("1844674407370955"),
				FilterPeriod:             10,
				DecayPeriod:              120,
				ReductionFactor:          1_000,
				MaxVolatilityAccumulator: 100_000,
				VariableFeeControl:       100_000,
			},
		},
		CollectFeeMode:            dynamic_bonding_curve.CollectFeeModeQuoteToken,
		MigrationOption:           dynamic_bonding_curve.MigrationOptionMETDAMMV2,
		ActivationType:            dynamic_bonding_curve.ActivationTypeTimestamp,
		TokenType:                 dynamic_bonding_curve.TokenTypeSPL,
		TokenDecimal:              dynamic_bonding_curve.TokenDecimalNine,
		PartnerLpPercentage:       80,
		PartnerLockedLpPercentage: 0,
		CreatorLpPercentage:       20,
		CreatorLockedLpPercentage: 0,
		MigrationQuoteThreshold:   0.5 * 1e9, // 85 * 1e9, >= 750 USD
		SqrtStartPrice:            u128.GenUint128FromString("58333726687135158"),
		LockedVesting: dynamic_bonding_curve.LockedVesting{
			AmountPerPeriod:                0,
			CliffDurationFromMigrationTime: 0,
			Frequency:                      0,
			NumberOfPeriod:                 0,
			CliffUnlockAmount:              0,
		},
		MigrationFeeOption: dynamic_bonding_curve.MigrationFeeFixedBps200, // 0: Fixed 25bps, 1: Fixed 30bps, 2: Fixed 100bps, 3: Fixed 200bps, 4: Fixed 400bps, 5: Fixed 600bps
		TokenSupply: &dynamic_bonding_curve.TokenSupplyParams{
			PreMigrationTokenSupply:  1000000000000000000,
			PostMigrationTokenSupply: 1000000000000000000,
		},
		CreatorTradingFeePercentage: 0,
		TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityImmutable,
		MigrationFee: dynamic_bonding_curve.MigrationFee{
			FeePercentage:        2,
			CreatorFeePercentage: 0,
		},
		// MigratedPoolFee: &dbc.MigratedPoolFee{},
		Padding: [7]uint64{},
		// use case
		Curve: []dynamic_bonding_curve.LiquidityDistributionParameters{
			{
				SqrtPrice: u128.GenUint128FromString("233334906748540631"),
				Liquidity: u128.GenUint128FromString("622226417996106429201027821619672729"),
			},
			{
				SqrtPrice: u128.GenUint128FromString("79226673521066979257578248091"),
				Liquidity: u128.GenUint128FromString("1"),
			},
		},
	}
}

func testDBCBuildCurve() (*dynamic_bonding_curve.ConfigParameters, error) {
	return dbc.BuildCurve(dynamic_bonding_curve.BuildCurveParam{
		BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
			TotalTokenSupply:  1000000000,
			MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
			TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalSix,
			TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
			LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
				TotalLockedVestingAmount:       0,
				NumberOfVestingPeriod:          0,
				CliffUnlockAmount:              0,
				TotalVestingDuration:           0,
				CliffDurationFromMigrationTime: 0,
			},
			BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
				BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerExponential,
				FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
					StartingFeeBps: 100,
					EndingFeeBps:   100,
					NumberOfPeriod: 0,
					TotalDuration:  0,
				},
			},
			DynamicFeeEnabled:           true,
			ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
			CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
			MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
			TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
			PartnerLpPercentage:         0,
			CreatorLpPercentage:         0,
			PartnerLockedLpPercentage:   100,
			CreatorLockedLpPercentage:   0,
			CreatorTradingFeePercentage: 0,
			Leftover:                    10000,
			TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityCreatorUpdateAuthority,
			MigrationFee: dynamic_bonding_curve.MigrationFee{
				FeePercentage:        0,
				CreatorFeePercentage: 0,
			},
		},
		PercentageSupplyOnMigration: 2.983257229832572,
		MigrationQuoteThreshold:     95.07640791476408,
		// PercentageSupplyOnMigration: 10,
		// MigrationQuoteThreshold:     300,
	})

}

func testDBCBuildCurveWithMarketCap() (*dynamic_bonding_curve.ConfigParameters, error) {
	return dbc.BuildCurveWithMarketCap(dynamic_bonding_curve.BuildCurveWithMarketCapParam{
		BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
			TotalTokenSupply:  1000000000,
			MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
			TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalSix,
			TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
			LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
				TotalLockedVestingAmount:       0,
				NumberOfVestingPeriod:          0,
				CliffUnlockAmount:              0,
				TotalVestingDuration:           0,
				CliffDurationFromMigrationTime: 0,
			},
			BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
				BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerLinear,
				FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
					StartingFeeBps: 100,
					EndingFeeBps:   100,
					NumberOfPeriod: 0,
					TotalDuration:  0,
				},
			},
			DynamicFeeEnabled:           true,
			ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
			CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
			MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
			TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
			PartnerLpPercentage:         0,
			CreatorLpPercentage:         0,
			PartnerLockedLpPercentage:   100,
			CreatorLockedLpPercentage:   0,
			CreatorTradingFeePercentage: 0,
			Leftover:                    10000,
			TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityImmutable,
			MigrationFee: dynamic_bonding_curve.MigrationFee{
				FeePercentage:        10,
				CreatorFeePercentage: 50,
			},
		},
		// InitialMarketCap:   100,
		// MigrationMarketCap: 3000,
		InitialMarketCap:   23.5,
		MigrationMarketCap: 405.882352941,
	})
}

func testDBCBuildCurveWithTwoSegments() (*dynamic_bonding_curve.ConfigParameters, error) {
	return dbc.BuildCurveWithTwoSegments(dynamic_bonding_curve.BuildCurveWithTwoSegmentsParam{
		BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
			TotalTokenSupply:  1000000000,
			MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
			TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalNine,
			TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
			LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
				TotalLockedVestingAmount:       0,
				NumberOfVestingPeriod:          0,
				CliffUnlockAmount:              0,
				TotalVestingDuration:           0,
				CliffDurationFromMigrationTime: 0,
			},
			BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
				BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerExponential,
				FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
					StartingFeeBps: 5000,
					EndingFeeBps:   100,
					NumberOfPeriod: 120,
					TotalDuration:  120,
				},
			},
			DynamicFeeEnabled:           true,
			ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
			CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
			MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
			TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
			PartnerLpPercentage:         0,
			CreatorLpPercentage:         0,
			PartnerLockedLpPercentage:   100,
			CreatorLockedLpPercentage:   0,
			CreatorTradingFeePercentage: 0,
			Leftover:                    350000000,
			TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityCreatorUpdateAuthority,
			MigrationFee: dynamic_bonding_curve.MigrationFee{
				FeePercentage:        10,
				CreatorFeePercentage: 50,
			},
		},

		InitialMarketCap:            20000,
		MigrationMarketCap:          1000000,
		PercentageSupplyOnMigration: 20,
	})
}

func testDBCBuildCurveWithLiquidityWeights() (*dynamic_bonding_curve.ConfigParameters, error) {
	liquidityWeights := make([]float64, 16)

	base := decimal.NewFromFloat(1.2)
	for i := 0; i < 16; i++ {
		liquidityWeights[i] = base.Pow(decimal.NewFromInt(int64(i))).InexactFloat64()
	}

	return dbc.BuildCurveWithLiquidityWeights(dynamic_bonding_curve.BuildCurveWithLiquidityWeightsParam{
		BuildCurveBaseParam: dynamic_bonding_curve.BuildCurveBaseParam{
			TotalTokenSupply:  1000000000,
			MigrationOption:   dynamic_bonding_curve.MigrationOptionMETDAMMV2,
			TokenBaseDecimal:  dynamic_bonding_curve.TokenDecimalSix,
			TokenQuoteDecimal: dynamic_bonding_curve.TokenDecimalNine,
			LockedVestingParam: dynamic_bonding_curve.LockedVestingParams{
				TotalLockedVestingAmount:       0,
				NumberOfVestingPeriod:          0,
				CliffUnlockAmount:              0,
				TotalVestingDuration:           0,
				CliffDurationFromMigrationTime: 0,
			},
			BaseFeeParams: dynamic_bonding_curve.BaseFeeParams{
				BaseFeeMode: dynamic_bonding_curve.BaseFeeModeFeeSchedulerLinear,
				FeeSchedulerParam: &dynamic_bonding_curve.FeeSchedulerParams{
					StartingFeeBps: 100,
					EndingFeeBps:   100,
					NumberOfPeriod: 0,
					TotalDuration:  0,
				},
			},
			DynamicFeeEnabled:           true,
			ActivationType:              dynamic_bonding_curve.ActivationTypeSlot,
			CollectFeeMode:              dynamic_bonding_curve.CollectFeeModeQuoteToken,
			MigrationFeeOption:          dynamic_bonding_curve.MigrationFeeFixedBps100,
			TokenType:                   dynamic_bonding_curve.TokenTypeSPL,
			PartnerLpPercentage:         0,
			CreatorLpPercentage:         0,
			PartnerLockedLpPercentage:   100,
			CreatorLockedLpPercentage:   0,
			CreatorTradingFeePercentage: 0,
			Leftover:                    10000,
			TokenUpdateAuthority:        dynamic_bonding_curve.TokenUpdateAuthorityImmutable,
			MigrationFee: dynamic_bonding_curve.MigrationFee{
				FeePercentage:        10,
				CreatorFeePercentage: 50,
			},
		},

		InitialMarketCap:   30,
		MigrationMarketCap: 300,
		LiquidityWeights:   liquidityWeights,
	})
}
