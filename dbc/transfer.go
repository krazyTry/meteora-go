package dbc

import (
	"context"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func (m *DBC) TransferInstruction(ctx context.Context,
	payer solana.PublicKey,
	sender solana.PublicKey,
	receiver solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	amount *big.Int,
) ([]solana.Instruction, error) {

	baseMint := virtualPool.BaseMint // baseMint

	var instructions []solana.Instruction

	sendTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, sender, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	receiveTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, receiver, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	transferIx := token.NewTransferCheckedInstruction(
		amount.Uint64(),
		uint8(config.TokenDecimal),
		sendTokenAccount,
		baseMint,
		receiveTokenAccount,
		payer,
		[]solana.PublicKey{},
	).Build()

	return append(instructions, transferIx), nil
}

func (m *DBC) Transfer(ctx context.Context,
	payer *solana.Wallet,
	sender *solana.Wallet,
	receiver *solana.Wallet,
	baseMint solana.PublicKey,
	amount *big.Int,
) (string, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.TransferInstruction(ctx,
		payer.PublicKey(),
		sender.PublicKey(),
		receiver.PublicKey(),
		virtualPool,
		config,
		amount,
	)

	if err != nil {
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", err
		}
		return "-", nil
	}

	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}
