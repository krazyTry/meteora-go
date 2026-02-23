package meteora

// import (
// 	"fmt"
// 	"testing"

// 	"github.com/gagliardetto/solana-go"
// 	dbc "github.com/krazyTry/meteora-go/dynamic_bonding_curve"
// )

// func TestXxx(t *testing.T) {

// 	k1 := solana.NewWallet()
// 	fmt.Println("k1", k1.PublicKey(), k1.PrivateKey)
// 	k2 := solana.NewWallet()
// 	fmt.Println("k2", k2.PublicKey(), k2.PrivateKey)
// 	k3 := solana.NewWallet()
// 	fmt.Println("k3", k3.PublicKey(), k3.PrivateKey)
// 	k4 := solana.NewWallet()
// 	fmt.Println("k4", k4.PublicKey(), k4.PrivateKey)

// 	// init

// 	{
// 		config := solana.MustPublicKeyFromBase58("CTkJQSeYRX5WaYUJydHoFtzzgvXkhyr1oevA9G6ZHhVe")
// 		cfg, err := dbc.GetConfig(ctx, rpcClient, config)
// 		if err != nil {
// 			t.Fatal("dbc.GetConfig() fail")
// 		}

// 		fmt.Println("===========================")
// 		fmt.Println("print config info")
// 		fmt.Println("config.MigrationQuoteThreshold:", cfg.MigrationQuoteThreshold)
// 		fmt.Println("===========================")

// 		baseMint := solana.MustPublicKeyFromBase58("81Z4GLY2nHv1W78BeJs6eh4rJ2eskX2FziHwUVRNyear")
// 		pool, err := dbc.GetPoolByBaseMint(ctx, rpcClient, baseMint)
// 		if err != nil {
// 			t.Fatal("GetPoolByBaseMint() fail", err)
// 		}
// 		fmt.Println("===========================")
// 		fmt.Println("print pool info")
// 		fmt.Println("pool.BaseMint:", pool.BaseMint)
// 		fmt.Println("pool.Config:", pool.Config)
// 		fmt.Println("pool.MigrationProgress:", pool.MigrationProgress)
// 		fmt.Println("pool.IsMigrated:", pool.IsMigrated)

// 		fmt.Println("pool.BaseReserve:", pool.BaseReserve)
// 		fmt.Println("pool.QuoteReserve:", pool.QuoteReserve)
// 		fmt.Println("pool.PartnerBaseFee:", pool.PartnerBaseFee)
// 		fmt.Println("pool.PartnerQuoteFee:", pool.PartnerQuoteFee)
// 		fmt.Println("pool.CreatorBaseFee:", pool.CreatorBaseFee)
// 		fmt.Println("pool.CreatorQuoteFee:", pool.CreatorQuoteFee)
// 		fmt.Println("pool.IsWithdrawLeftover:", pool.IsWithdrawLeftover)
// 		fmt.Println("pool.MigrationFeeWithdrawStatus:", pool.MigrationFeeWithdrawStatus)
// 		fmt.Println("pool.CreatorBaseFee:", pool.CreatorBaseFee)
// 		fmt.Println("pool.CreatorQuoteFee:", pool.CreatorQuoteFee)
// 		fmt.Println("===========================")

// 		// // Bm5kGfQhXXst8zSFS8DieYuXCxXDwkJbsXSFHT1aPreQ
// 		// "partner_private_key": "2bCoL7upg5jrRodu8hUA1csJLXWHNfk4XJgtu4PsuDmedRTh1HGsgy26JYgZeMjehavnMzg7eMVMSwqu5PDdySDa",
// 		// // 9ZRGCiJWfR83WX8ARks3NCPV4HKs22UhFfPZtQzG6Zpr
// 		// "leftover_receiver_private_key": "3HG74Upp1LjUUeiifiiELMeF1uiczdwnzgMcRMF6uQLiqGSM3dzmbJE42PX2bG9pUHyQiFgE9SJ1UccZvxeqRxAA",

// 		// poolPartner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("2bCoL7upg5jrRodu8hUA1csJLXWHNfk4XJgtu4PsuDmedRTh1HGsgy26JYgZeMjehavnMzg7eMVMSwqu5PDdySDa")}
// 		// fmt.Printf("poolPartner address:%s(%s)\n", poolPartner.PublicKey(), poolPartner.PrivateKey)

// 		// leftoverReceiver := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("3HG74Upp1LjUUeiifiiELMeF1uiczdwnzgMcRMF6uQLiqGSM3dzmbJE42PX2bG9pUHyQiFgE9SJ1UccZvxeqRxAA")}
// 		// fmt.Printf("leftoverReceiver address:%s(%s)\n", leftoverReceiver.PublicKey(), leftoverReceiver.PrivateKey)

// 		// meteoraDBC := dbc.NewDBC(rpcClient,
// 		// 	dbc.WithConfigPublicKey(config),
// 		// 	dbc.WithPartner(poolPartner),
// 		// 	dbc.WithLeftoverReceiver(leftoverReceiver),
// 		// )
// 		// fmt.Println("try to claim PartnerQuoteFee")
// 		// ctx1, cancel1 := context.WithTimeout(ctx, time.Second*30)
// 		// defer cancel1()
// 		// sig, err := meteoraDBC.ClaimPartnerTradingFee(ctx1, wsClient, poolPartner, baseMint, true, pool.PartnerBaseFee)
// 		// if err != nil {
// 		// 	t.Fatal("dbc.ClaimPartnerTradingFee() fail", err)
// 		// }
// 		// fmt.Println("claim PartnerQuoteFee completed sig:", sig)
// 	}

// 	return

// }
