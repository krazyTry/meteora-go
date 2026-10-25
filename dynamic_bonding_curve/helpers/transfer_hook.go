package helpers

import (
	"encoding/binary"
	"errors"

	solanago "github.com/solana-foundation/solana-go/v2"
)

// DeriveExtraAccountMetaAddress is the transfer-hook extra-account-metas PDA.
func DeriveExtraAccountMetaAddress(mint, hookProgram solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("extra-account-metas"), mint.Bytes()}, hookProgram)
	return pub
}

// ParseExtraAccountMetas reads literal pubkeys from an extra-account-metas account.
// A non-literal seed config returns an error so the caller can pass the accounts directly.
func ParseExtraAccountMetas(data []byte) ([]*solanago.AccountMeta, error) {
	if metas, err := parseExtraAccountMetasAt(data, 0); err == nil {
		return metas, nil
	}
	return parseExtraAccountMetasAt(data, 8)
}

func parseExtraAccountMetasAt(data []byte, offset int) ([]*solanago.AccountMeta, error) {
	if offset+4 > len(data) {
		return nil, errors.New("extra account metas are truncated")
	}
	count := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	if count > 64 {
		return nil, errors.New("extra account meta count is invalid")
	}
	if offset+int(count)*35 > len(data) {
		return nil, errors.New("extra account metas are truncated")
	}
	out := make([]*solanago.AccountMeta, 0, count)
	for i := uint32(0); i < count; i++ {
		if data[offset] != 0 {
			return nil, errors.New("dynamic transfer hook accounts must be supplied by the caller")
		}
		var pk solanago.PublicKey
		copy(pk[:], data[offset+1:offset+33])
		out = append(out, solanago.NewAccountMeta(pk, data[offset+34] != 0, data[offset+33] != 0))
		offset += 35
	}
	return out, nil
}
