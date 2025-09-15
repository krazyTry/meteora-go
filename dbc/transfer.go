package dbc

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// Transfer transfers tokens from a DBC pool.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// sig, _ := meteoraDBC.Transfer(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	from, // sender account
//	to, // receiver account
//	baseMint,// token address
//	amount,// transfer amount
//
// )
func (m *DBC) Transfer(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	sender *solana.Wallet,
	receiver solana.PublicKey,
	baseMint solana.PublicKey,
	amount *big.Int,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := solanago.TransferInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		sender.PublicKey(),
		receiver,
		baseMint,
		uint8(configState.TokenDecimal),
		amount,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(sender.PublicKey()):
				return &sender.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}
