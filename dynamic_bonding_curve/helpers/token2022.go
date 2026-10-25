package helpers

import (
	"context"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/programs/token"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

const (
	mintBaseSize             = 82
	extTransferFeeConfig     = 1
	extTransferHook          = 14
	transferFeeConfigPodSize = 120
)

type token2022TransferFee struct {
	Epoch                  uint64
	MaximumFee             uint64
	TransferFeeBasisPoints uint16
}

type token2022TransferFeeConfig struct {
	ConfigAuthority   *solanago.PublicKey
	WithheldAuthority *solanago.PublicKey
	Older             token2022TransferFee
	Newer             token2022TransferFee
}

func (c token2022TransferFeeConfig) forEpoch(epoch uint64) token2022TransferFee {
	if epoch < c.Newer.Epoch {
		return c.Older
	}
	return c.Newer
}

// GetSwapQuoteTransferFees reads the current-epoch transfer fees a swap quote needs.
// A Token-2022 quote mint with no fee config is returned as a zero fee so the quote can proceed.
func GetSwapQuoteTransferFees(ctx context.Context, client *rpc.Client, commitment rpc.CommitmentType, quoteMint solanago.PublicKey, quoteTokenFlag shared.TokenType, baseMint *solanago.PublicKey) (*shared.QuoteTransferFees, error) {
	if quoteTokenFlag != shared.TokenTypeToken2022 && baseMint == nil {
		return &shared.QuoteTransferFees{}, nil
	}
	epochInfo, err := client.GetEpochInfo(ctx, commitment)
	if err != nil {
		return nil, err
	}
	out := &shared.QuoteTransferFees{}
	if quoteTokenFlag == shared.TokenTypeToken2022 {
		fee, err := epochTransferFee(ctx, client, commitment, quoteMint, epochInfo.Epoch)
		if err != nil {
			return nil, err
		}
		out.Quote = fee
	}
	if baseMint != nil {
		fee, err := epochTransferFee(ctx, client, commitment, *baseMint, epochInfo.Epoch)
		if err != nil {
			return nil, err
		}
		out.Base = fee
	}
	return out, nil
}

func epochTransferFee(ctx context.Context, client *rpc.Client, commitment rpc.CommitmentType, mint solanago.PublicKey, epoch uint64) (*shared.EpochTransferFee, error) {
	acc, err := client.GetAccountInfoWithOpts(ctx, mint, &rpc.GetAccountInfoOpts{Commitment: commitment})
	if err != nil {
		return nil, err
	}
	if acc == nil || acc.Value == nil {
		return nil, errors.New("mint not found")
	}
	if acc.Value.Owner.Equals(token.ProgramID) {
		return &shared.EpochTransferFee{MaximumFee: big.NewInt(0)}, nil
	}
	cfg, ok, err := parseTransferFeeConfig(acc.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	if !ok {
		return &shared.EpochTransferFee{MaximumFee: big.NewInt(0)}, nil
	}
	fee := cfg.forEpoch(epoch)
	return &shared.EpochTransferFee{
		TransferFeeBasisPoints: fee.TransferFeeBasisPoints,
		MaximumFee:             new(big.Int).SetUint64(fee.MaximumFee),
	}, nil
}

func parseTransferFeeConfig(data []byte) (token2022TransferFeeConfig, bool, error) {
	raw, ok, err := extensionValue(data, extTransferFeeConfig)
	if err != nil || !ok {
		return token2022TransferFeeConfig{}, ok, err
	}
	if len(raw) < transferFeeConfigPodSize {
		return token2022TransferFeeConfig{}, false, errors.New("transfer fee config is truncated")
	}
	cfg := token2022TransferFeeConfig{
		ConfigAuthority:   optionalNonZeroPubkey(raw[0:32]),
		WithheldAuthority: optionalNonZeroPubkey(raw[32:64]),
	}
	var n int
	cfg.Older, n, err = readPodTransferFee(raw[72:])
	if err != nil {
		return token2022TransferFeeConfig{}, false, err
	}
	cfg.Newer, _, err = readPodTransferFee(raw[72+n:])
	if err != nil {
		return token2022TransferFeeConfig{}, false, err
	}
	return cfg, true, nil
}

func readPodTransferFee(b []byte) (token2022TransferFee, int, error) {
	if len(b) < 18 {
		return token2022TransferFee{}, 0, errors.New("transfer fee is truncated")
	}
	return token2022TransferFee{
		Epoch:                  binary.LittleEndian.Uint64(b[0:8]),
		MaximumFee:             binary.LittleEndian.Uint64(b[8:16]),
		TransferFeeBasisPoints: binary.LittleEndian.Uint16(b[16:18]),
	}, 24, nil
}

func optionalNonZeroPubkey(b []byte) *solanago.PublicKey {
	var pk solanago.PublicKey
	copy(pk[:], b)
	if pk.IsZero() {
		return nil
	}
	return &pk
}

func extensionValue(data []byte, want uint16) ([]byte, bool, error) {
	if len(data) < mintBaseSize {
		return nil, false, errors.New("mint account is too short")
	}
	if len(data) == mintBaseSize {
		return nil, false, nil
	}
	// Token-2022 stores one account-type byte between the base mint and the TLV extensions.
	off := mintBaseSize + 1
	for off+4 <= len(data) {
		typ := binary.LittleEndian.Uint16(data[off : off+2])
		length := binary.LittleEndian.Uint16(data[off+2 : off+4])
		off += 4
		if typ == 0 && length == 0 {
			break
		}
		if off+int(length) > len(data) {
			return nil, false, errors.New("mint extension is truncated")
		}
		val := data[off : off+int(length)]
		off += int(length)
		if typ == want {
			return val, true, nil
		}
	}
	return nil, false, nil
}

// NativeMint2022 is the Token-2022 wrapped SOL mint, which cannot be a quote mint.
var NativeMint2022 = solanago.MustPublicKeyFromBase58("9pan9bMn5HatX4EJdBwg9VgCa7Uz5HL8N1m5D3NdXejP")

const (
	extMetadataPointer = 18
	extTokenMetadata   = 19
)

// HasTransferFeeOrConfigAuthority reports a non-zero transfer fee or a live config authority.
func HasTransferFeeOrConfigAuthority(data []byte, currentEpoch uint64) (bool, error) {
	cfg, ok, err := parseTransferFeeConfig(data)
	if err != nil || !ok {
		return false, err
	}
	if cfg.ConfigAuthority != nil {
		return true, nil
	}
	if currentEpoch < cfg.Newer.Epoch {
		return !zeroPodFee(cfg.Older) || !zeroPodFee(cfg.Newer), nil
	}
	return !zeroPodFee(cfg.forEpoch(currentEpoch)), nil
}

// EpochTransferFeeFromMint is the mint's fee for currentEpoch. It is nil when the mint has no transfer-fee config.
func EpochTransferFeeFromMint(data []byte, currentEpoch uint64) (*shared.EpochTransferFee, error) {
	cfg, ok, err := parseTransferFeeConfig(data)
	if err != nil || !ok {
		return nil, err
	}
	fee := cfg.forEpoch(currentEpoch)
	return &shared.EpochTransferFee{TransferFeeBasisPoints: fee.TransferFeeBasisPoints, MaximumFee: new(big.Int).SetUint64(fee.MaximumFee)}, nil
}

// IsSupportedQuoteMint reports whether a quote mint can be used without a token badge.
func IsSupportedQuoteMint(mint solanago.PublicKey, data []byte, currentEpoch uint64) (bool, error) {
	if mint.Equals(NativeMint2022) {
		return false, errors.New("Token-2022 native mint is not supported as quote mint")
	}
	types, err := extensionTypes(data)
	if err != nil {
		return false, err
	}
	for _, typ := range types {
		if typ != extTransferFeeConfig && typ != extMetadataPointer && typ != extTokenMetadata {
			return false, nil
		}
	}
	hasFee, err := HasTransferFeeOrConfigAuthority(data, currentEpoch)
	if err != nil {
		return false, err
	}
	return !hasFee, nil
}

func zeroPodFee(fee token2022TransferFee) bool {
	return fee.TransferFeeBasisPoints == 0 || fee.MaximumFee == 0
}

func extensionTypes(data []byte) ([]uint16, error) {
	if len(data) <= mintBaseSize {
		return nil, nil
	}
	off := mintBaseSize + 1
	var out []uint16
	for off+4 <= len(data) {
		typ := binary.LittleEndian.Uint16(data[off : off+2])
		length := binary.LittleEndian.Uint16(data[off+2 : off+4])
		off += 4
		if typ == 0 && length == 0 {
			break
		}
		if off+int(length) > len(data) {
			return nil, errors.New("mint extension is truncated")
		}
		out = append(out, typ)
		off += int(length)
	}
	return out, nil
}

// TransferHookProgram reads the Token-2022 transfer-hook program id. The second result is false when the mint has no hook.
func TransferHookProgram(data []byte) (solanago.PublicKey, bool, error) {
	raw, ok, err := extensionValue(data, extTransferHook)
	if err != nil || !ok {
		return solanago.PublicKey{}, false, err
	}
	if len(raw) < 64 {
		return solanago.PublicKey{}, false, errors.New("transfer hook extension is truncated")
	}
	var program solanago.PublicKey
	copy(program[:], raw[32:64])
	if program.IsZero() {
		return solanago.PublicKey{}, false, nil
	}
	return program, true, nil
}
