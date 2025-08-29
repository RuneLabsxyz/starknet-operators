package starknetrpc

import (
	"fmt"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StarknetRPCAvailableStatus string

const (
	// Pending status indicates that the RPC pod is waiting for the restore operation to complete.
	StarknetRPCAvailableStatusPending StarknetRPCAvailableStatus = "Pending"
	// Creating status indicates that the pod is being created and is initializing.
	StarknetRPCAvailableStatusCreating StarknetRPCAvailableStatus = "Creating"
	// CatchingUp status indicates that the pod is ready to respond to requests, but is still catching up to the latest block.
	StarknetRPCAvailableStatusCatchingUp StarknetRPCAvailableStatus = "CatchingUp"
	// Ready status indicates that the pod is ready to respond to requests, and is fully operational.
	StarknetRPCAvailableStatusReady StarknetRPCAvailableStatus = "Ready"
	// Failed status indicates that the pod failed to start, or another error occurred.
	StarknetRPCAvailableStatusFailed StarknetRPCAvailableStatus = "Failed"
	// Unknown status indicates that the RPC's status is unknown.
	//
	// This could happen if the pod is not responding to the requests for latest block height, or simply not responding at all,
	// before the transition to the Failed state (to prevent intermittent issues, or recreating in the event of an overload,
	// which could lead to a cascading failure).
	StarknetRPCAvailableStatusUnknown StarknetRPCAvailableStatus = "Unknown"
)

func (s StarknetRPCAvailableStatus) String() string {
	switch s {
	case StarknetRPCAvailableStatusPending:
		return "Pending"
	case StarknetRPCAvailableStatusCreating:
		return "Creating"
	case StarknetRPCAvailableStatusCatchingUp:
		return "CatchingUp"
	case StarknetRPCAvailableStatusReady:
		return "Ready"
	case StarknetRPCAvailableStatusFailed:
		return "Failed"
	case StarknetRPCAvailableStatusUnknown:
		return "Unknown"
	default:
		return fmt.Sprintf("Unknown(%s)", string(s))
	}
}

func (s StarknetRPCAvailableStatus) Message() string {
	switch s {
	case StarknetRPCAvailableStatusPending:
		return "The RPC is waiting for the initial state to be ready"
	case StarknetRPCAvailableStatusCreating:
		return "RPC Pod is being scheduled"
	case StarknetRPCAvailableStatusCatchingUp:
		return "The node is catching up with the latest block"
	case StarknetRPCAvailableStatusReady:
		return "The node is ready and is fully synced"
	case StarknetRPCAvailableStatusFailed:
		return "The node failed to start, or another error occurred"
	case StarknetRPCAvailableStatusUnknown:
		return "Impossible to determine the status of the node"
	default:
		return fmt.Sprintf("Unknown(%s)", string(s))
	}
}

func (s StarknetRPCAvailableStatus) Status() metav1.ConditionStatus {
	switch s {
	case StarknetRPCAvailableStatusPending:
		return metav1.ConditionUnknown
	case StarknetRPCAvailableStatusCreating:
		return metav1.ConditionUnknown
	case StarknetRPCAvailableStatusCatchingUp:
		return metav1.ConditionUnknown
	case StarknetRPCAvailableStatusReady:
		return metav1.ConditionTrue
	case StarknetRPCAvailableStatusFailed:
		return metav1.ConditionFalse
	case StarknetRPCAvailableStatusUnknown:
		return metav1.ConditionUnknown
	default:
		return metav1.ConditionUnknown
	}
}

func (s StarknetRPCAvailableStatus) AsCondition() metav1.Condition {
	return metav1.Condition{
		Type:    string(StarknetRPCAvailableCondition),
		Status:  s.Status(),
		Reason:  string(s),
		Message: s.Message(),
	}
}

func (s StarknetRPCAvailableStatus) Apply() condition.StateTransition {
	return func(rpc *v1alpha1.StarknetRPC) {
		meta.SetStatusCondition(&rpc.Status.Conditions, s.AsCondition())
	}
}
