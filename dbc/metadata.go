package dbc

import (
	"context"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
)

func dbcCreatePartnerMetadata(
	m *DBC,
	// Params:
	name string,
	website string,
	logo string,

	// Accounts:
	partnerMetadata solana.PublicKey,
	payer solana.PublicKey,
	feeClaimer solana.PublicKey,

) (solana.Instruction, error) {

	metadata := dbc.CreatePartnerMetadataParameters{
		Name:    name,
		Website: website,
		Logo:    logo,
	}

	return dbc.NewCreatePartnerMetadataInstruction(
		// Params:
		metadata,

		// Accounts:
		partnerMetadata,
		payer,
		feeClaimer,
		solana.SystemProgramID,
		eventAuthority,
		dbc.ProgramID,
	)
}

func (m *DBC) CreatePartnerMetadataInstruction(
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
