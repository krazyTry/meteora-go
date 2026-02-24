package dynamic_bonding_curve

import (
	"context"
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
)

func TestMigrateToDammV2(t *testing.T) {

	dbcService := dynamic_bonding_curve.NewDynamicBondingCurve(rpcClient, rpc.CommitmentFinalized)

	configAddress := solana.MustPublicKeyFromBase58("")

	partner := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("partner address:", partner.PublicKey())

	leftover := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	fmt.Println("leftover address:", leftover.PublicKey())

	configState, err := dbcService.GetPoolConfig(context.Background(), configAddress)
	if err != nil {
		t.Fatal("GetConfig() fail", err)
	}

	ownerWallet := &solana.Wallet{PrivateKey: solana.MustPrivateKeyFromBase58("")}
	owner := ownerWallet.PublicKey()
	fmt.Println("owner address:", owner)

	baseMint := solana.MustPublicKeyFromBase58("")

	ctx1 := context.Background()

	poolState, err := dbcService.GetPoolByBaseMint(ctx1, baseMint)
	if err != nil {
		t.Fatal("GetPoolByPoolAddress() fail", err)
	}
	fmt.Println("token info Pool:", poolState)

	if poolState.Account.QuoteReserve == configState.MigrationQuoteThreshold {

		params := dynamic_bonding_curve.CreateDammV2MigrationMetadataParams{
			VirtualPool: poolState.Pubkey,
			Payer:       ownerWallet.PublicKey(),
			Config:      configAddress,
		}
		ix, err := dbcService.CreateDammV2MigrationMetadata(ctx1, params)
		if err != nil {
			t.Fatal("CreateDammV2MigrationMetadata() fail", err)
		}
		instructions := []solana.Instruction{ix}
		sig, err := SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(ownerWallet.PublicKey()):
				return &ownerWallet.PrivateKey
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("swap SendTransaction() fail", err)
		}
		fmt.Println("create migration metadata success Success sig:", sig.String())

		if dynamic_bonding_curve.MigrationProgress(poolState.Account.MigrationProgress) < dynamic_bonding_curve.MigrationProgressPostBondingCurve {
			lockerParams := dynamic_bonding_curve.CreateLockerParams{
				VirtualPool: poolState.Pubkey,
				Payer:       ownerWallet.PublicKey(),
			}
			pre, lockerIx, post, err := dbcService.CreateLocker(ctx1, lockerParams)
			if err != nil {
				t.Fatal("CreateLocker() fail", err)
			}

			if len(pre) > 0 {
				// 没上锁过
				instructions = []solana.Instruction{}
				instructions = append(pre, lockerIx)
				instructions = append(instructions, post...)

				sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
					switch {
					case key.Equals(ownerWallet.PublicKey()):
						return &ownerWallet.PrivateKey
					default:
						return nil
					}
				})
				if err != nil {
					t.Fatal("swap SendTransaction() fail", err)
				}
				fmt.Println("create locker success Success sig:", sig.String())
			}
		}

		resp, err := dbcService.MigrateToDammV2(ctx1, dynamic_bonding_curve.MigrateToDammV2Params{
			VirtualPool: poolState.Pubkey,
			Payer:       ownerWallet.PublicKey(),
			DammConfig:  helpers.GetDammV2Config(dynamic_bonding_curve.MigrationFeeOption(configState.MigrationFeeOption)),
		})
		if err != nil {
			t.Fatal("MigrateToDammV2() fail", err)
		}

		sig, err = SendTransaction(ctx1, rpcClient, wsClient, resp.Transaction, func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(ownerWallet.PublicKey()):
				return &ownerWallet.PrivateKey
			case key.Equals(resp.FirstPositionNFT.PublicKey()):
				return &resp.FirstPositionNFT
			case key.Equals(resp.SecondPositionNFT.PublicKey()):
				return &resp.SecondPositionNFT
			default:
				return nil
			}
		})
		if err != nil {
			t.Fatal("MigrateToDammV2 SendTransaction() fail", err)
		}
		fmt.Println("migrate to damm v2 success Success sig:", sig.String())

		if poolState.Account.IsWithdrawLeftover == 0 {
			leftoverParams := dynamic_bonding_curve.WithdrawLeftoverParams{
				VirtualPool: poolState.Pubkey,
				Payer:       leftover.PublicKey(),
			}
			pre, withdrawIx, post, err := dbcService.WithdrawLeftover(ctx1, leftoverParams)
			if err != nil {
				t.Fatal("WithdrawLeftover() fail", err)
			}

			instructions = []solana.Instruction{}
			instructions = append(pre, withdrawIx)
			instructions = append(instructions, post...)

			sig, err = SendInstruction(ctx1, rpcClient, wsClient, instructions, ownerWallet.PublicKey(), func(key solana.PublicKey) *solana.PrivateKey {
				switch {
				case key.Equals(ownerWallet.PublicKey()):
					return &ownerWallet.PrivateKey
				case key.Equals(leftover.PublicKey()):
					return &leftover.PrivateKey
				default:
					return nil
				}
			})
			if err != nil {
				t.Fatal("withdraw SendInstruction() fail", err)
			}

			fmt.Println("withdraw token success Success sig:", sig.String())
		}

		if dynamic_bonding_curve.MigrationFeeWithdrawStatus(poolState.Account.MigrationFeeWithdrawStatus).IsPartnerWithdraw() == 0 {

		}
	}

}
