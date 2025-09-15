package dbc

import (
	"context"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
)

// CreatePartnerMetadataInstruction generates the instruction needed for modifying Partner's Metadata.
// It creates a new partner metadata account. This partner metadata will be tagged to a wallet address that holds the config keys.
//
// Example:
//
// instructions,_ := CreatePartnerMetadataInstruction(
//
//	ctx,
//	payer,
//	m.feeClaimer.PublicKey(), // parnter
//	name,
//	website,
//	logo,
//
// )
func CreatePartnerMetadataInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	poolPartner solana.PublicKey,
	name string,
	website string,
	logo string,
) ([]solana.Instruction, error) {

	partnerMetadata, err := dbc.DerivePartnerMetadataPDA(poolPartner)
	if err != nil {
		return nil, err
	}

	metadata := dbc.CreatePartnerMetadataParameters{
		Name:    name,
		Website: website,
		Logo:    logo,
	}

	createIx, err := dbc.NewCreatePartnerMetadataInstruction(
		// Params:
		metadata,

		// Accounts:
		partnerMetadata,
		payer,
		poolPartner,
		solana.SystemProgramID,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}

	return []solana.Instruction{createIx}, nil
}

// CreateVirtualPoolMetadataInstruction generates the instruction needed for modifying the Metadata of a dbc pool.
// It creates a new pool metadata account.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
// instructions,_ := CreateVirtualPoolMetadataInstruction(
//
//	ctx,
//	payer,
//	m.poolCreator.PublicKey(), // creator
//	poolState.Address, // dbc pool address
//	name,
//	website,
//	logo,
//
// )
func (m *DBC) CreateVirtualPoolMetadataInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	poolAddress solana.PublicKey,
	name string,
	website string,
	logo string,
) ([]solana.Instruction, error) {
	virtualPoolMetadata, err := dbc.DeriveDbcPoolMetadataPDA(poolAddress)
	if err != nil {
		return nil, err
	}
	metadata := dbc.CreateVirtualPoolMetadataParameters{
		Name:    name,
		Website: website,
		Logo:    logo,
	}

	createIx, err := dbc.NewCreateVirtualPoolMetadataInstruction(
		// Params:
		metadata,

		// Accounts:
		poolAddress,
		virtualPoolMetadata,
		poolCreator,
		payer,
		solana.SystemProgramID,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{createIx}, nil
}
