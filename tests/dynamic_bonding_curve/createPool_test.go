package dynamic_bonding_curve

import (
	"context"
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve"
)

func TestCreatePool(t *testing.T) {

	dbcService := dynamic_bonding_curve.NewDynamicBondingCurve(rpcClient, rpc.CommitmentFinalized)

	configAddress := solana.MustPublicKeyFromBase58("")

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

	name := "MeteoraGoTest"
	symbol := "METAGOTEST"
	uri := "https://launch.meteora.ag/icons/logo.svg"

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	mintWallet := solana.NewWallet()
	baseMint := mintWallet.PublicKey()

	fmt.Println("try to create token mint address:", baseMint, mintWallet)
	ctx1 := context.Background()

	createParams := dynamic_bonding_curve.CreatePoolParams{
		Name:        name,
		Symbol:      symbol,
		URI:         uri,
		Payer:       owner,
		PoolCreator: owner,
		Config:      configAddress,
		BaseMint:    baseMint,
	}

	createIx, err := dbcService.CreatePool(ctx1, createParams)
	if err != nil {
		t.Fatal("dbc.CreatePool() fail", err)
	}

	instructions := []solana.Instruction{createIx}
	sig, err := SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(mintWallet.PublicKey()):
			return &mintWallet.PrivateKey
		case key.Equals(ownerWallet.PublicKey()):
			return &ownerWallet.PrivateKey
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatal("create SendTransaction() fail", err)
	}
	fmt.Println("create token success Success sig:", sig.String())

	poolState, err := dbcService.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("GetPoolByPoolAddress() fail", err)
	}
	fmt.Println("create token success Pool:", poolState)
}
