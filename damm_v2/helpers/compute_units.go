package helpers

import (
	"context"
	"fmt"
	"strings"

	solanago "github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/damm_v2/shared"
)

const defaultSimulationUnits uint32 = 1_400_000

// GetSimulationComputeUnits simulates a transaction and returns the compute units consumed.
// It mirrors the TS helper that prepends a high compute limit instruction to fetch the real usage.
func GetSimulationComputeUnits(
	ctx context.Context,
	client *rpc.Client,
	instructions []solanago.Instruction,
	payer solanago.PublicKey,
	commitment rpc.CommitmentType,
) (*uint64, error) {
	if len(instructions) == 0 {
		return nil, fmt.Errorf("no instructions to simulate")
	}

	limitIx := computebudget.NewSetComputeUnitLimitInstructionBuilder().
		SetUnits(defaultSimulationUnits).
		Build()

	testInstructions := make([]solanago.Instruction, 0, len(instructions)+1)
	testInstructions = append(testInstructions, limitIx)
	testInstructions = append(testInstructions, instructions...)

	tx, err := solanago.NewTransaction(
		testInstructions,
		solanago.Hash{},
		solanago.TransactionPayer(payer),
	)
	if err != nil {
		return nil, err
	}

	opts := &rpc.SimulateTransactionOpts{
		SigVerify:              false,
		ReplaceRecentBlockhash: true,
	}
	if commitment != "" {
		opts.Commitment = commitment
	}

	resp, err := client.SimulateTransactionWithOpts(ctx, tx, opts)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Value == nil {
		return nil, nil
	}
	if resp.Value.Err != nil {
		logs := "No logs available"
		if len(resp.Value.Logs) > 0 {
			logs = strings.Join(resp.Value.Logs, "\n  • ")
		}
		return nil, fmt.Errorf("transaction simulation failed:\n  •%s%v", logs, resp.Value.Err)
	}
	return resp.Value.UnitsConsumed, nil
}

// GetEstimatedComputeUnitUsageWithBuffer returns the simulated compute units plus a buffer.
func GetEstimatedComputeUnitUsageWithBuffer(
	ctx context.Context,
	client *rpc.Client,
	instructions []solanago.Instruction,
	payer solanago.PublicKey,
	buffer *float64,
) (uint64, error) {
	buf := 0.1
	if buffer != nil {
		buf = *buffer
	}
	if buf < 0 {
		buf = 0
	}
	if buf > 1 {
		buf = 1
	}

	estimated, err := GetSimulationComputeUnits(ctx, client, instructions, payer, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}
	if estimated == nil {
		return 0, nil
	}

	extra := float64(*estimated) * buf
	if extra > float64(shared.MaxCuBuffer) {
		extra = float64(shared.MaxCuBuffer)
	} else if extra < float64(shared.MinCuBuffer) {
		extra = float64(shared.MinCuBuffer)
	}

	return uint64(float64(*estimated) + extra), nil
}

// GetEstimatedComputeUnitIxWithBuffer builds a SetComputeUnitLimit instruction using simulation.
// If simulation fails, a fallback instruction with defaultSimulationUnits is returned along with the error.
func GetEstimatedComputeUnitIxWithBuffer(
	ctx context.Context,
	client *rpc.Client,
	instructions []solanago.Instruction,
	payer solanago.PublicKey,
	buffer *float64,
) (solanago.Instruction, error) {
	units, err := GetEstimatedComputeUnitUsageWithBuffer(ctx, client, instructions, payer, buffer)
	if err != nil || units == 0 {
		return computebudget.NewSetComputeUnitLimitInstructionBuilder().
			SetUnits(defaultSimulationUnits).
			Build(), err
	}

	if units > uint64(^uint32(0)) {
		units = uint64(^uint32(0))
	}
	return computebudget.NewSetComputeUnitLimitInstructionBuilder().
		SetUnits(uint32(units)).
		Build(), nil
}
