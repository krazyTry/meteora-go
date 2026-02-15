package helpers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
)

const maxFeeBasisPoints = 10_000

type TransferFeeIncludedAmount struct {
	Amount      *big.Int
	TransferFee *big.Int
}

type TransferFeeExcludedAmount struct {
	Amount      *big.Int
	TransferFee *big.Int
}

func calculatePreFeeAmount(transferFeeBasisPoints uint16, maximumFee *big.Int, postFeeAmount *big.Int) *big.Int {
	if postFeeAmount.Sign() == 0 {
		return big.NewInt(0)
	}
	if transferFeeBasisPoints == 0 {
		return new(big.Int).Set(postFeeAmount)
	}
	if transferFeeBasisPoints == maxFeeBasisPoints {
		return new(big.Int).Add(postFeeAmount, maximumFee)
	}
	oneInBps := big.NewInt(maxFeeBasisPoints)
	numerator := new(big.Int).Mul(postFeeAmount, oneInBps)
	denominator := new(big.Int).Sub(oneInBps, big.NewInt(int64(transferFeeBasisPoints)))
	rawPreFee := new(big.Int).Add(numerator, denominator)
	rawPreFee.Sub(rawPreFee, big.NewInt(1))
	rawPreFee.Div(rawPreFee, denominator)

	if new(big.Int).Sub(rawPreFee, postFeeAmount).Cmp(maximumFee) >= 0 {
		return new(big.Int).Add(postFeeAmount, maximumFee)
	}
	return rawPreFee
}

func calculateInverseFee(transferFeeBasisPoints uint16, maximumFee *big.Int, postFeeAmount *big.Int) *big.Int {
	preFeeAmount := calculatePreFeeAmount(transferFeeBasisPoints, maximumFee, postFeeAmount)
	return calculateFee(transferFeeBasisPoints, maximumFee, preFeeAmount)
}

func calculateFee(transferFeeBasisPoints uint16, maximumFee *big.Int, amount *big.Int) *big.Int {
	if transferFeeBasisPoints == 0 || amount.Sign() == 0 {
		return big.NewInt(0)
	}
	if transferFeeBasisPoints == maxFeeBasisPoints {
		return new(big.Int).Set(maximumFee)
	}
	fee := new(big.Int).Mul(amount, big.NewInt(int64(transferFeeBasisPoints)))
	fee.Div(fee, big.NewInt(maxFeeBasisPoints))
	if fee.Cmp(maximumFee) > 0 {
		return new(big.Int).Set(maximumFee)
	}
	return fee
}

func CalculateTransferFeeIncludedAmount(transferFeeExcludedAmount *big.Int, tokenInfo *TokenInfo) TransferFeeIncludedAmount {
	if transferFeeExcludedAmount.Sign() == 0 {
		return TransferFeeIncludedAmount{Amount: big.NewInt(0), TransferFee: big.NewInt(0)}
	}
	if tokenInfo == nil || !tokenInfo.HasTransferFee {
		return TransferFeeIncludedAmount{Amount: new(big.Int).Set(transferFeeExcludedAmount), TransferFee: big.NewInt(0)}
	}
	maxFee := tokenInfo.MaximumFee
	if maxFee == nil {
		maxFee = big.NewInt(0)
	}
	transferFee := calculateInverseFee(tokenInfo.BasisPoints, maxFee, transferFeeExcludedAmount)
	return TransferFeeIncludedAmount{Amount: new(big.Int).Add(transferFeeExcludedAmount, transferFee), TransferFee: transferFee}
}

func CalculateTransferFeeExcludedAmount(transferFeeIncludedAmount *big.Int, tokenInfo *TokenInfo) TransferFeeExcludedAmount {
	if tokenInfo == nil || !tokenInfo.HasTransferFee {
		return TransferFeeExcludedAmount{Amount: new(big.Int).Set(transferFeeIncludedAmount), TransferFee: big.NewInt(0)}
	}
	maxFee := tokenInfo.MaximumFee
	if maxFee == nil {
		maxFee = big.NewInt(0)
	}
	fee := calculateFee(tokenInfo.BasisPoints, maxFee, transferFeeIncludedAmount)
	return TransferFeeExcludedAmount{Amount: new(big.Int).Sub(new(big.Int).Set(transferFeeIncludedAmount), fee), TransferFee: fee}
}

// HasTransferHookExtension is a placeholder for Token2022 transfer hook detection.
// Caller can pre-populate TokenInfo.HasTransferHook to enforce policy.
func HasTransferHookExtension(tokenInfo *TokenInfo) bool {
	if tokenInfo == nil {
		return false
	}
	return tokenInfo.HasTransferHook
}

const (
	// Token mint base size (Token-2020 compatible header)
	MintBaseSize = 82 // :contentReference[oaicite:2]{index=2}

	// Token-2022 extension types (from SPL Token JS docs) :contentReference[oaicite:3]{index=3}
	ExtUninitialized     uint16 = 0
	ExtTransferFeeConfig uint16 = 1
	ExtTransferHook      uint16 = 14
)

// Extensions holds raw TLV slices + decoded structs you care about.
type Extensions struct {
	Raw map[uint16][]byte

	TransferFeeConfig *TransferFeeConfig
	HasTransferHook   bool
}

type TransferFee struct {
	Epoch  uint64
	MaxFee uint64
	FeeBps uint16
}

type TransferFeeConfig struct {
	// Authorities are stored as COption<Pubkey> on-chain; nil means "None".
	TransferFeeConfigAuthority *solanago.PublicKey
	WithdrawWithheldAuthority  *solanago.PublicKey

	WithheldAmount uint64

	Older TransferFee
	Newer TransferFee
}

// FeeForEpoch picks older/newer based on current epoch.
// SPL Token JS docs describe older used if currentEpoch < newer.epoch, else newer. :contentReference[oaicite:4]{index=4}
func (c *TransferFeeConfig) FeeForEpoch(currentEpoch uint64) TransferFee {
	if currentEpoch < c.Newer.Epoch {
		return c.Older
	}
	return c.Newer
}

// parseToken2022Extensions parses TLV extensions from a Token-2022 *Mint* account data.
//
// data: account data bytes (base64 decoded)
// returns:
// - Extensions.Raw: map[extType]extData
// - Extensions.TransferFeeConfig decoded if present
func parseToken2022Extensions(data []byte) (*Extensions, error) {
	if len(data) < MintBaseSize {
		return nil, fmt.Errorf("data too short for mint base: got=%d want>=%d", len(data), MintBaseSize)
	}

	exts := &Extensions{
		Raw: make(map[uint16][]byte),
	}

	off := MintBaseSize
	for {
		// Need at least 4 bytes for TLV header: u16 type + u16 length
		if off+4 > len(data) {
			break
		}

		typ := binary.LittleEndian.Uint16(data[off : off+2])
		l := binary.LittleEndian.Uint16(data[off+2 : off+4])
		off += 4

		// Convention: trailing zero padding often appears; stop on (0,0).
		if typ == ExtUninitialized && l == 0 {
			break
		}

		if off+int(l) > len(data) {
			return nil, fmt.Errorf("invalid TLV length: type=%d len=%d off=%d total=%d", typ, l, off, len(data))
		}

		val := data[off : off+int(l)]
		off += int(l)

		// Store raw
		exts.Raw[typ] = val

		// Decode the ones we care about
		switch typ {
		case ExtTransferFeeConfig:
			cfg, err := parseTransferFeeConfig(val)
			if err != nil {
				return nil, fmt.Errorf("parse TransferFeeConfig failed: %w", err)
			}
			exts.TransferFeeConfig = cfg
		case ExtTransferHook:
			exts.HasTransferHook = true
		}

		// Optional: if remaining bytes are all zeros, you can break early.
		// (Leave it simple; loop will stop naturally when it can't read next header.)
	}

	return exts, nil
}

// --- internal decoders ---

func parseTransferFeeConfig(b []byte) (*TransferFeeConfig, error) {
	// Layout is fixed-size in practice, but can evolve; parse minimally and ignore any extra bytes.
	// Expected fields (conceptually):
	// - COption<Pubkey> transfer_fee_config_authority
	// - COption<Pubkey> withdraw_withheld_authority
	// - u64 withheld_amount
	// - TransferFee older
	// - TransferFee newer
	//
	// We decode:
	// - COption<Pubkey> is 4-byte tag (0 or 1) + 32 bytes when Some
	// - TransferFee: epoch u64 + maximum_fee u64 + transfer_fee_basis_points u16 (+ possible padding)
	off := 0

	auth1, n, err := readCOptionPubkey(b, off)
	if err != nil {
		return nil, err
	}
	off += n

	auth2, n, err := readCOptionPubkey(b, off)
	if err != nil {
		return nil, err
	}
	off += n

	if off+8 > len(b) {
		return nil, errors.New("transfer fee config: truncated withheld_amount")
	}
	withheld := binary.LittleEndian.Uint64(b[off : off+8])
	off += 8

	older, n, err := readTransferFee(b, off)
	if err != nil {
		return nil, err
	}
	off += n

	newer, n, err := readTransferFee(b, off)
	if err != nil {
		return nil, err
	}
	off += n

	return &TransferFeeConfig{
		TransferFeeConfigAuthority: auth1,
		WithdrawWithheldAuthority:  auth2,
		WithheldAmount:             withheld,
		Older:                      older,
		Newer:                      newer,
	}, nil
}

func readCOptionPubkey(b []byte, off int) (*solanago.PublicKey, int, error) {
	if off+4 > len(b) {
		return nil, 0, errors.New("COption<Pubkey>: truncated tag")
	}
	tag := binary.LittleEndian.Uint32(b[off : off+4])
	off += 4

	switch tag {
	case 0:
		// None
		return nil, 4, nil
	case 1:
		if off+32 > len(b) {
			return nil, 0, errors.New("COption<Pubkey>: truncated pubkey")
		}
		pk := solanago.PublicKeyFromBytes(b[off : off+32])
		return &pk, 4 + 32, nil
	default:
		return nil, 0, fmt.Errorf("COption<Pubkey>: invalid tag=%d", tag)
	}
}

func readTransferFee(b []byte, off int) (TransferFee, int, error) {
	// Minimum bytes: 8 + 8 + 2 = 18
	if off+18 > len(b) {
		return TransferFee{}, 0, errors.New("TransferFee: truncated")
	}
	epoch := binary.LittleEndian.Uint64(b[off : off+8])
	maxFee := binary.LittleEndian.Uint64(b[off+8 : off+16])
	bps := binary.LittleEndian.Uint16(b[off+16 : off+18])

	// Some implementations pad to 24 bytes (align); TLV length tells the true size.
	// Here we advance by 24 if enough bytes remain AND the surrounding structure expects padding.
	// Safer strategy: if there are at least 24 bytes left before next field, consume 24; otherwise consume 18.
	// But since we're parsing inside a known struct, we can prefer 24 when available.
	advance := 18
	if off+24 <= len(b) {
		advance = 24
	}

	return TransferFee{
		Epoch:  epoch,
		MaxFee: maxFee,
		FeeBps: bps,
	}, advance, nil
}
