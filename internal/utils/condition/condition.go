package condition

import (
	"context"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StateTransition func(*v1alpha1.StarknetRPC)

func SetPhases(ctx context.Context, client client.Client, node *v1alpha1.StarknetRPC, transitions ...StateTransition) error {
	if node.Status.Conditions == nil {
		node.Status.Conditions = []metav1.Condition{}
	}
	// Apply all transitions
	for _, transition := range transitions {
		transition(node)
	}

	// Update the node status
	if err := client.Status().Update(ctx, node); err != nil {
		return err
	}

	return nil
}
