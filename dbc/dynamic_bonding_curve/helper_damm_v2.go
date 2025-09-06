package dynamic_bonding_curve

import (
	"bytes"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"

	"github.com/gagliardetto/solana-go"
)

// Derives the DAMM V2 pool address
func DeriveDammV2PoolPDA(config, tokenAMint, tokenBMint solana.PublicKey) (solana.PublicKey, error) {
	// Get the first and second keys based on byte comparison
	var firstKey, secondKey solana.PublicKey
	if bytes.Compare(tokenAMint.Bytes(), tokenBMint.Bytes()) > 0 {
		firstKey = tokenAMint
		secondKey = tokenBMint
	} else {
		firstKey = tokenBMint
		secondKey = tokenAMint
	}

	seeds := [][]byte{[]byte("pool"), config.Bytes(), firstKey.Bytes(), secondKey.Bytes()}

	pda, _, err := solana.FindProgramAddress(seeds, cp_amm.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DeriveDammV2PoolAuthority() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("pool_authority")}
	poolAuthority, _, err := solana.FindProgramAddress(seeds, cp_amm.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return poolAuthority, nil
}

func DeriveDammV2EventAuthority() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("__event_authority")}

	eventAuthority, _, err := solana.FindProgramAddress(seeds, cp_amm.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return eventAuthority, nil
}

// Derives the dbc token vault address
func DeriveDammV2TokenVaultPDA(pool, mint solana.PublicKey) (solana.PublicKey, error) {
	seed := [][]byte{[]byte("token_vault"), mint.Bytes(), pool.Bytes()}
	pda, _, err := solana.FindProgramAddress(seed, cp_amm.ProgramID)
	if err != nil {
		// log.Fatalf("find vault PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DerivePosition(positionNft solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("position"), positionNft.Bytes()}
	position, _, err := solana.FindProgramAddress(seeds, cp_amm.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return position, nil
}

func DerivePositionNftAccount(positionNftMint solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("position_nft_account"), positionNftMint.Bytes()}

	positionNftAccount, _, err := solana.FindProgramAddress(seeds, cp_amm.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return positionNftAccount, nil
}

func GetDammV2Config(migrationFeeOption MigrationFeeOption) solana.PublicKey {
	switch migrationFeeOption {
	case MigrationFeeFixedBps25:
		return solana.MustPublicKeyFromBase58("7F6dnUcRuyM2TwR8myT1dYypFXpPSxqwKNSFNkxyNESd")
	case MigrationFeeFixedBps30:
		return solana.MustPublicKeyFromBase58("2nHK1kju6XjphBLbNxpM5XRGFj7p9U8vvNzyZiha1z6k")
	case MigrationFeeFixedBps100:
		return solana.MustPublicKeyFromBase58("Hv8Lmzmnju6m7kcokVKvwqz7QPmdX9XfKjJsXz8RXcjp")
	case MigrationFeeFixedBps200:
		return solana.MustPublicKeyFromBase58("2c4cYd4reUYVRAB9kUUkrq55VPyy2FNQ3FDL4o12JXmq")
	case MigrationFeeFixedBps400:
		return solana.MustPublicKeyFromBase58("AkmQWebAwFvWk55wBoCr5D62C6VVDTzi84NJuD9H7cFD")
	case MigrationFeeFixedBps600:
		return solana.MustPublicKeyFromBase58("DbCRBj8McvPYHJG1ukj8RE15h2dCNUdTAESG49XpQ44u")
	}
	return solana.PublicKey{}
}
