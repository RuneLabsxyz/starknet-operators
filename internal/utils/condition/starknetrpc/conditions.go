package starknetrpc

import (
	"context"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StarknetRPCConditionType string

const (
	StarknetRPCRestoreCondition   StarknetRPCConditionType = "Restore"
	StarknetRPCAvailableCondition StarknetRPCConditionType = "Available"
)

func Initialize(ctx context.Context, client client.Client, rpc *v1alpha1.StarknetRPC) error {
	// Only initialize conditions if they are not already set
	if len(rpc.Status.Conditions) == 0 {
		return condition.SetPhases(ctx, client, rpc,
			StarknetRPCRestoreStatusPending.Apply(),
			StarknetRPCAvailableStatusPending.Apply(),
		)
	}
	return nil
}
