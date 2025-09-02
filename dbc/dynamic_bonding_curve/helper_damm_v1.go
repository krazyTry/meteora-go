package dynamic_bonding_curve

import "github.com/gagliardetto/solana-go"

func GetDammV1Config(migrationFeeOption MigrationFeeOption) solana.PublicKey {
	switch migrationFeeOption {
	case MigrationFeeFixedBps25:
		return solana.MustPublicKeyFromBase58("8f848CEy8eY6PhJ3VcemtBDzPPSD4Vq7aJczLZ3o8MmX")
	case MigrationFeeFixedBps30:
		return solana.MustPublicKeyFromBase58("HBxB8Lf14Yj8pqeJ8C4qDb5ryHL7xwpuykz31BLNYr7S")
	case MigrationFeeFixedBps100:
		return solana.MustPublicKeyFromBase58("7v5vBdUQHTNeqk1HnduiXcgbvCyVEZ612HLmYkQoAkik")
	case MigrationFeeFixedBps200:
		return solana.MustPublicKeyFromBase58("EkvP7d5yKxovj884d2DwmBQbrHUWRLGK6bympzrkXGja")
	case MigrationFeeFixedBps400:
		return solana.MustPublicKeyFromBase58("9EZYAJrcqNWNQzP2trzZesP7XKMHA1jEomHzbRsdX8R2")
	case MigrationFeeFixedBps600:
		return solana.MustPublicKeyFromBase58("8cdKo87jZU2R12KY1BUjjRPwyjgdNjLGqSGQyrDshhud")
	}
	return solana.PublicKey{}
}
