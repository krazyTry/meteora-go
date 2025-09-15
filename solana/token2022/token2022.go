package token2022

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// TransferFee represents the transfer fee configuration for a specific epoch
type TransferFee struct {
	Epoch       uint64 // Epoch when this fee configuration is active
	MaximumFee  uint64 // Maximum fee amount in token units
	BasisPoints uint16 // Fee rate in basis points (1/10000)
}

// TransferFeeConfig represents the complete transfer fee configuration for a token
type TransferFeeConfig struct {
	TransferFeeConfigAuthority *solana.PublicKey // Authority that can modify transfer fee configuration
	WithdrawWithheldAuthority  *solana.PublicKey // Authority that can withdraw withheld fees
	WithheldAmount             uint64            // Amount of fees currently withheld
	OlderTransferFee           TransferFee       // Previous epoch's transfer fee configuration
	NewerTransferFee           TransferFee       // Current/next epoch's transfer fee configuration
}

// parseCOptionPubkey
func parseCOptionPubkey(data []byte) (*solana.PublicKey, int, error) {
	if len(data) < 1 {
		return nil, 0, errors.New("data too short for COption tag")
	}

	switch data[0] {
	case 0: // None
		return nil, 1, nil
	case 1: // Some(pubkey)
		if len(data) < 33 {
			return nil, 0, errors.New("data too short for Pubkey")
		}
		key := solana.PublicKeyFromBytes(data[1:33])
		return &key, 33, nil
	default:
		return nil, 0, errors.New("invalid COption tag")
	}
}

// GetTransferFeeConfig
func GetTransferFeeConfig(ctx context.Context, rpcClient *rpc.Client, baseMint solana.PublicKey) (*TransferFeeConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	out, err := rpcClient.GetAccountInfoWithOpts(ctx, baseMint, &rpc.GetAccountInfoOpts{Commitment: rpc.CommitmentFinalized})
	if err != nil {
		return nil, err
	}
	return getTransferFeeConfig(out.GetBinary())
}

func getTransferFeeConfig(data []byte) (*TransferFeeConfig, error) {
	// Token2022 TransferFee discriminator
	idx := bytes.Index(data, []byte{0xad, 0x65, 0x2b, 0x54, 0x0e, 0x4d, 0x0d, 0x27})
	if idx < 0 {
		return nil, nil
	}

	buf := data[idx+8:]

	cfg := &TransferFeeConfig{}

	// 1. TransferFeeConfigAuthority
	auth, n, err := parseCOptionPubkey(buf)
	if err != nil {
		return nil, err
	}
	cfg.TransferFeeConfigAuthority = auth
	buf = buf[n:]

	// 2. WithdrawWithheldAuthority
	withdrawAuth, n, err := parseCOptionPubkey(buf)
	if err != nil {
		return nil, err
	}
	cfg.WithdrawWithheldAuthority = withdrawAuth
	buf = buf[n:]

	// 3. WithheldAmount
	if len(buf) < 8 {
		return nil, errors.New("data too short for WithheldAmount")
	}
	cfg.WithheldAmount = binary.LittleEndian.Uint64(buf[:8])
	buf = buf[8:]

	// 4. OlderTransferFee
	if len(buf) < 18 {
		return nil, errors.New("data too short for OlderTransferFee")
	}
	cfg.OlderTransferFee = TransferFee{
		Epoch:       binary.LittleEndian.Uint64(buf[:8]),
		MaximumFee:  binary.LittleEndian.Uint64(buf[8:16]),
		BasisPoints: binary.LittleEndian.Uint16(buf[16:18]),
	}
	buf = buf[18:]

	// 5. NewerTransferFee
	if len(buf) < 18 {
		return nil, errors.New("data too short for NewerTransferFee")
	}
	cfg.NewerTransferFee = TransferFee{
		Epoch:       binary.LittleEndian.Uint64(buf[:8]),
		MaximumFee:  binary.LittleEndian.Uint64(buf[8:16]),
		BasisPoints: binary.LittleEndian.Uint16(buf[16:18]),
	}

	return cfg, nil
}

// GetEpochFee
func GetEpochFee(cfg *TransferFeeConfig, currentEpoch uint64) TransferFee {
	if cfg == nil {
		return TransferFee{Epoch: 0, MaximumFee: 0, BasisPoints: 0} // SPL Token returns 0 fee
	}
	if currentEpoch >= cfg.NewerTransferFee.Epoch {
		return cfg.NewerTransferFee
	}
	return cfg.OlderTransferFee
}

// CalculateFee
func CalculateFee(tf TransferFee, amount *big.Int) *big.Int {
	if tf.BasisPoints == 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(amount, big.NewInt(int64(tf.BasisPoints)))
	fee.Div(fee, big.NewInt(10000))
	if fee.Uint64() > tf.MaximumFee {
		return new(big.Int).SetUint64(tf.MaximumFee)
	}
	return fee
}
