package dynamic_bonding_curve

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	jsoniter "github.com/json-iterator/go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dynamicbondingcurve "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
)

func TestEvent(t *testing.T) {

	rpcClient := rpc.New(rpc.MainNetBeta_RPC)
	defer rpcClient.Close()
	wsClient, err := ws.Connect(context.Background(), rpc.MainNetBeta_WS)
	if err != nil {
		log.Fatal(err)
	}

	defer wsClient.Close()

	sub, err := wsClient.LogsSubscribeMentions(
		helpers.DynamicBondingCurveProgramID,
		rpc.CommitmentProcessed,
	)

	if err != nil {
		log.Fatal(err)
	}
	defer sub.Unsubscribe()

	fmt.Printf("start listen program %s Swap event...\n", helpers.DynamicBondingCurveProgramID)

	for {
		data, err := sub.Recv(context.Background())
		if err != nil {
			log.Printf("recv error: %v", err)
			continue
		}
		if data.Value.Err != nil {
			continue
		}
		fmt.Println(data.Value.Signature)

		maxSupportedTransactionVersion := uint64(0)
		out, err := rpcClient.GetTransaction(context.Background(), data.Value.Signature, &rpc.GetTransactionOpts{
			Encoding: solana.EncodingBase64,
			// Commitment: rpc.CommitmentFinalized,
			MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
		})
		if err != nil {
			continue
		}

		twm := out.Transaction

		tx, err := twm.GetTransaction()
		if err != nil {
			continue
		}

		meta := out.Meta

		bShow := false
		for _, inner := range meta.InnerInstructions {
			for _, inst := range inner.Instructions {

				programID, err := tx.Message.ResolveProgramIDIndex(inst.ProgramIDIndex)
				if err != nil {
					continue
				}
				if !helpers.DynamicBondingCurveProgramID.Equals(programID) {
					continue
				}
				bShow = true
				event, err := dynamicbondingcurve.ParseAnyEvent(inst.Data[8:])
				if err != nil {
					continue
				}
				switch inst := event.(type) {
				case *dynamicbondingcurve.EvtSwap:
					jsonStr, _ := jsoniter.MarshalToString(inst)
					fmt.Printf("EvtSwap:%v\n", jsonStr)
				case *dynamicbondingcurve.EvtSwap2:
					jsonStr, _ := jsoniter.MarshalToString(inst)
					fmt.Printf("EvtSwap2:%v\n", jsonStr)
				}
			}
		}
		if bShow {
			fmt.Println(tx.Signatures)
		}
	}
}
