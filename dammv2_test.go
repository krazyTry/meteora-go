package meteora

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	dammV2 "github.com/krazyTry/meteora-go/damm.v2"
)

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
