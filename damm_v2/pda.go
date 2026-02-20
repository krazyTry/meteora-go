package dammv2

import (
	"bytes"
	"encoding/binary"

	solanago "github.com/gagliardetto/solana-go"
)

// getFirstKey returns the lexicographically larger key bytes.
func getFirstKey(key1, key2 solanago.PublicKey) []byte {
	buf1 := key1.Bytes()
	buf2 := key2.Bytes()
	if bytes.Compare(buf1, buf2) == 1 {
		return buf1
	}
	return buf2
}

// getSecondKey returns the lexicographically smaller key bytes.
func getSecondKey(key1, key2 solanago.PublicKey) []byte {
	buf1 := key1.Bytes()
	buf2 := key2.Bytes()
	if bytes.Compare(buf1, buf2) == 1 {
		return buf2
	}
	return buf1
}

func DerivePoolAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("pool_authority")}, CpAmmProgramID)
	return pub
}

func DeriveEventAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("__event_authority")}, CpAmmProgramID)
	return pub
}

func DeriveConfigAddress(index uint64) solanago.PublicKey {
	indexBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(indexBytes, index)
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("config"), indexBytes}, CpAmmProgramID)
	return pub
}

func DerivePoolAddress(config, tokenAMint, tokenBMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{
		[]byte("pool"),
		config.Bytes(),
		getFirstKey(tokenAMint, tokenBMint),
		getSecondKey(tokenAMint, tokenBMint),
	}, CpAmmProgramID)
	return pub
}

func DerivePositionAddress(positionNft solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("position"), positionNft.Bytes()}, CpAmmProgramID)
	return pub
}

func DeriveTokenVaultAddress(tokenMint, pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("token_vault"), tokenMint.Bytes(), pool.Bytes()}, CpAmmProgramID)
	return pub
}

func DeriveRewardVaultAddress(pool solanago.PublicKey, rewardIndex uint8) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("reward_vault"), pool.Bytes(), []byte{rewardIndex}}, CpAmmProgramID)
	return pub
}

func DeriveCustomizablePoolAddress(tokenAMint, tokenBMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{
		[]byte("cpool"),
		getFirstKey(tokenAMint, tokenBMint),
		getSecondKey(tokenAMint, tokenBMint),
	}, CpAmmProgramID)
	return pub
}

func DeriveTokenBadgeAddress(tokenMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("token_badge"), tokenMint.Bytes()}, CpAmmProgramID)
	return pub
}

func DeriveClaimFeeOperatorAddress(operator solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("cf_operator"), operator.Bytes()}, CpAmmProgramID)
	return pub
}

func DerivePositionNftAccount(positionNftMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("position_nft_account"), positionNftMint.Bytes()}, CpAmmProgramID)
	return pub
}

func DeriveOperatorAddress(whitelistedAddress solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("operator"), whitelistedAddress.Bytes()}, CpAmmProgramID)
	return pub
}
