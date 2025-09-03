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

	// {
	// 	fmt.Println("transfer a little sol to leftoverReceiver")
	// 	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	// 	defer cancel1()
	// 	if _, err = testTransferSOL(ctx1, rpcClient, wsClient, payer, leftoverReceiver.PublicKey(), 0.1*1e9); err != nil {
	// 		t.Fatal("testTransferSOL fail", err)
	// 	}
	// }

	fmt.Printf("\n\n")

	meteoraDammV2, err := dammV2.NewDammV2(
		wsClient,
		rpcClient,
		poolCreator,
	)
	if err != nil {
		t.Fatal("NewDammV2() fail", err)
	}

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
		testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)
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

	// {
	// 	poolCreatorAuthority := solana.NewWallet()
	// 	fmt.Printf("poolCreatorAuthority address:%s(%s)\n", poolCreatorAuthority.PublicKey(), poolCreatorAuthority.PrivateKey)

	// 	mintWallet := solana.NewWallet()
	// 	mintWallet = &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("5SBchR7A6ysnjDdMeaLoxPNpn1wFL1xCCzJAdCBAr3cLesrPru3HprVwTRrtyGp9DuUpQksUdaAtWSanFWVK8QsS")}
	// 	baseMint := mintWallet.PublicKey()
	// 	fmt.Printf("new token mint address:%s(%s)\n", baseMint, mintWallet.PrivateKey)

	// 	// testCreateTokenMint(t, ctx, rpcClient, wsClient, mintWallet, payer, poolCreator)

	// 	{
	// 		fmt.Println("准备创建 CreateCustomizablePoolWithDynamicConfig")
	// 		ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	// 		defer cancel1()
	// 		baseAmount := big.NewInt(1_000_000)
	// 		quoteAmount := big.NewInt(1) // SOL
	// 		sig, _, _, err := meteoraDammV2.CreateCustomizablePoolWithDynamicConfig(
	// 			ctx1,
	// 			payer,
	// 			1,
	// 			poolCreatorAuthority,
	// 			1, // 1 base token = 1 quote token
	// 			baseMint,
	// 			solana.WrappedSol,
	// 			baseAmount,
	// 			quoteAmount,
	// 			false,
	// 			cp_amm.ActivationTypeTimestamp,
	// 			cp_amm.CollectFeeModeBothToken,
	// 			nil,
	// 			true,
	// 			5000, // 50%
	// 			25,   // 0.25%
	// 			cp_amm.FeeSchedulerModeExponential,
	// 			60,   // 60 peridos
	// 			3600, // 60 * 60
	// 			true,
	// 		)
	// 		if err != nil {
	// 			t.Fatal("meteoraDammV2.CreateCustomizablePoolWithDynamicConfig fail", err)
	// 		}
	// 		fmt.Println("创建完成 CreateCustomizablePoolWithDynamicConfig sig:", sig)
	// 	}
	// 	testCpAmmPoolCheck(t, ctx, meteoraDammV2, baseMint)
	// }

}

func testCpAmmPoolCheck(t *testing.T, ctx context.Context, cpamm *dammV2.DammV2, baseMint solana.PublicKey) *dammV2.Pool {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	pool, err := cpamm.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("cpamm.GetPoolByBaseMint() fail")
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

func testCreateTokenMint(t *testing.T, ctx context.Context, rpcClient *rpc.Client, wsClient *ws.Client, mint, payer, creator *solana.Wallet) {
	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
	defer cancel1()

	mintAmount := uint64(10_000_000 * 1e9)

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

	// mintTx := token.NewMintToInstruction(
	// 	1_000_000*1e9,
	// 	mint.PublicKey(),
	// 	ata,
	// 	payer.PublicKey(),
	// 	nil,
	// ).Build()

	sig, err := solanago.SendTransaction(
		ctx1,
		rpcClient,
		wsClient,
		[]solana.Instruction{createIx, initializeIx, ix, mintIx},
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(mint.PublicKey()):
				return &mint.PrivateKey
			case key.Equals(creator.PublicKey()):
				return &creator.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		t.Fatal("solanago.SendTransaction fail", err)
	}
	fmt.Println("testCreateTokenMint 成功", sig.String())
}
