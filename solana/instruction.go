package solana

import (
	"context"
	bin "encoding/binary"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// PrepareTokenATA checks if ATA exists, creates it if it doesn't exist
func PrepareTokenATA(
	ctx context.Context,
	rpcClient *rpc.Client,
	owner solana.PublicKey,
	tokenMint solana.PublicKey,
	payer solana.PublicKey,
	instructions *[]solana.Instruction,
) (solana.PublicKey, error) {
	tokenATA, _, err := solana.FindAssociatedTokenAddress(
		owner,
		tokenMint,
	)

	if err != nil {
		return solana.PublicKey{}, err
	}

	exists, err := GetAccountInfo(ctx, rpcClient, tokenATA)
	if err != nil && err != rpc.ErrNotFound {
		return solana.PublicKey{}, err
	}

	if exists == nil {
		ix := associatedtokenaccount.NewCreateInstruction(
			payer, owner, tokenMint,
		).Build()
		*instructions = append(*instructions, ix)
	}
	return tokenATA, nil
}

var (
	ataInstructionTypeID          = binary.NoTypeIDDefaultID
	transferInstructionTypeID     = binary.TypeIDFromUint32(system.Instruction_Transfer, bin.LittleEndian)
	syncNativeInstructionTypeID   = binary.TypeIDFromUint8(token.Instruction_SyncNative)
	closeAccountInstructionTypeID = binary.TypeIDFromUint8(token.Instruction_CloseAccount)
)

// SplitInstructions splits instructions into three phases: start, middle, end.
// The start and end phases will attempt to deduplicate specific types to avoid multiple identical instructions.
func SplitInstructions(oldInstructions []solana.Instruction) ([]solana.Instruction, []solana.Instruction, []solana.Instruction) {
	var (
		startInstruction  []solana.Instruction
		middleInstruction []solana.Instruction
		endInstruction    []solana.Instruction
	)
loop:
	for _, v := range oldInstructions {
		switch inst := v.(type) {
		case *associatedtokenaccount.Instruction:
			switch inst.BaseVariant.TypeID {
			case binary.NoTypeIDDefaultID:
				vs := v.Accounts()
				bJump := false
				for _, vv := range startInstruction {
					vvs := vv.Accounts()
					if vs[0].PublicKey != vvs[0].PublicKey || vs[1].PublicKey != vvs[1].PublicKey ||
						vs[2].PublicKey != vvs[2].PublicKey || vs[3].PublicKey != vvs[3].PublicKey {
						continue
					}
					bJump = true
					break
				}
				if !bJump {
					startInstruction = append(startInstruction, v)
				}
				continue loop
			}
		case *system.Instruction:
			switch inst.BaseVariant.TypeID {
			case binary.TypeIDFromUint32(system.Instruction_Transfer, bin.LittleEndian): // wrapSOLIx ?
			}
		case *token.Instruction:
			switch inst.BaseVariant.TypeID {
			case binary.TypeIDFromUint8(token.Instruction_SyncNative): // syncNativeIx ?

			case binary.TypeIDFromUint8(token.Instruction_CloseAccount):
				vs := v.Accounts()
				bJump := false
				for _, vv := range endInstruction {
					vvs := vv.Accounts()
					if vs[0].PublicKey != vvs[0].PublicKey || vs[1].PublicKey != vvs[1].PublicKey || vs[2].PublicKey != vvs[2].PublicKey {
						continue
					}
					bJump = true
					break
				}
				if !bJump {
					endInstruction = append(endInstruction, v)
				}
				continue loop
			}
		default:
		}
		middleInstruction = append(middleInstruction, v)
	}
	return startInstruction, middleInstruction, endInstruction
}

// MergeInstructions merges instructions
func MergeInstructions(oldInstructions []solana.Instruction) []solana.Instruction {
	var (
		newInstructions []solana.Instruction
	)

	startInstruction, middleInstruction, endInstruction := SplitInstructions(oldInstructions)

	newInstructions = append(newInstructions, startInstruction...)
	newInstructions = append(newInstructions, middleInstruction...)
	newInstructions = append(newInstructions, endInstruction...)

	return newInstructions
}

// MergeInstructions2 merges instructions
func MergeInstructions2(oldInstructions []solana.Instruction) []solana.Instruction {
	var (
		ataCreateInstructions    []*associatedtokenaccount.Create
		transferInstructions     []*system.Transfer
		closeAccountInstructions []*token.CloseAccount
		syncNativeInstructions   []*token.SyncNative

		newInstructions []solana.Instruction
	)

	for _, v := range oldInstructions {
		switch inst := v.(type) {
		case *associatedtokenaccount.Instruction:
			if inst.TypeID != ataInstructionTypeID {
				newInstructions = append(newInstructions, v)
				break
			}

			ataCreate, ok := inst.Impl.(associatedtokenaccount.Create)
			if !ok {
				newInstructions = append(newInstructions, v)
				break
			}

			// deduplicate
			bSave := false
			for _, instruction := range ataCreateInstructions {
				if ataCreate.Mint != instruction.Mint ||
					ataCreate.Payer != instruction.Payer ||
					ataCreate.Wallet != instruction.Wallet {
					continue
				}

				bSave = true
				break
			}

			if !bSave {
				ataCreateInstructions = append(ataCreateInstructions, &ataCreate)
				newInstructions = append(newInstructions, v)
			}
		case *system.Instruction:
			if inst.TypeID != transferInstructionTypeID {
				newInstructions = append(newInstructions, v)
				break
			}

			transfer, ok := inst.Impl.(system.Transfer)
			if !ok {
				newInstructions = append(newInstructions, v)
				break
			}

			// deduplicate
			bSave := false
			for _, instruction := range transferInstructions {
				if transfer.GetFundingAccount().PublicKey != instruction.GetFundingAccount().PublicKey ||
					transfer.GetRecipientAccount().PublicKey != instruction.GetRecipientAccount().PublicKey {
					continue
				}

				bSave = true
				// add lamports to first
				*instruction.Lamports += *transfer.Lamports
				break
			}
			if !bSave {
				transferInstructions = append(transferInstructions, &transfer)
				newInstructions = append(newInstructions, v)
			}
		case *token.Instruction:
			switch inst.TypeID {
			case syncNativeInstructionTypeID:
				syncNative, ok := inst.Impl.(token.SyncNative)
				if !ok {
					newInstructions = append(newInstructions, v)
					break
				}
				// deduplicate
				bSave := false
				for _, instruction := range syncNativeInstructions {
					if syncNative.GetTokenAccount().PublicKey != instruction.GetTokenAccount().PublicKey {
						continue
					}

					bSave = true
					break
				}
				if !bSave {
					syncNativeInstructions = append(syncNativeInstructions, &syncNative)
					newInstructions = append(newInstructions, v)
				}
			case closeAccountInstructionTypeID:
				closeAccount, ok := inst.Impl.(token.CloseAccount)
				if !ok {
					newInstructions = append(newInstructions, v)
					break
				}

				// deduplicate
				bSave := false
				for _, instruction := range closeAccountInstructions {
					if closeAccount.GetAccount().PublicKey != instruction.GetAccount().PublicKey ||
						closeAccount.GetDestinationAccount().PublicKey != instruction.GetDestinationAccount().PublicKey ||
						closeAccount.GetOwnerAccount().PublicKey != instruction.GetOwnerAccount().PublicKey {
						continue
					}

					bSave = true
					break
				}

				if !bSave {
					closeAccountInstructions = append(closeAccountInstructions, &closeAccount)
					newInstructions = append(newInstructions, v)
				}
			default:
				newInstructions = append(newInstructions, v)
			}
		default:
			newInstructions = append(newInstructions, v)
		}
	}

	return newInstructions
}
