package starknetrpc

import (
	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StarknetRPCRestoreStatus string

const (
	// Pending status indicates that the restore operation is preparing.
	StarknetRPCRestoreStatusPending StarknetRPCRestoreStatus = "Pending"
	// Restoring status indicates that the restore operation is in progress.
	StarknetRPCRestoreStatusRestoring StarknetRPCRestoreStatus = "Restoring"
	// Success status indicates that the restore operation has completed successfully.
	StarknetRPCRestoreStatusSuccess StarknetRPCRestoreStatus = "Success"
	// Failed status indicates that the restore operation has failed.
	StarknetRPCRestoreStatusFailed StarknetRPCRestoreStatus = "Failed"
	// Skipped status indicates that the restore operation was skipped.
	StarknetRPCRestoreStatusSkipped StarknetRPCRestoreStatus = "Skipped"
)

func (s StarknetRPCRestoreStatus) Message() string {
	switch s {
	case StarknetRPCRestoreStatusPending:
		return "Restore operation is being setup"
	case StarknetRPCRestoreStatusRestoring:
		return "Restore operation is in progress"
	case StarknetRPCRestoreStatusSuccess:
		return "Restore operation has completed successfully"
	case StarknetRPCRestoreStatusFailed:
		return "Restore operation has failed"
	case StarknetRPCRestoreStatusSkipped:
		return "Restore operation was skipped by the configuration"
	default:
		return "Unknown status"
	}
}

func (s StarknetRPCRestoreStatus) Status() metav1.ConditionStatus {
	switch s {
	case StarknetRPCRestoreStatusPending:
		return metav1.ConditionFalse
	case StarknetRPCRestoreStatusRestoring:
		return metav1.ConditionFalse
	case StarknetRPCRestoreStatusFailed:
		return metav1.ConditionFalse
	case StarknetRPCRestoreStatusSuccess:
		return metav1.ConditionTrue
	case StarknetRPCRestoreStatusSkipped:
		return metav1.ConditionTrue
	default:
		return metav1.ConditionUnknown
	}
}

func (s StarknetRPCRestoreStatus) AsCondition() metav1.Condition {
	return metav1.Condition{
		Type:    string(StarknetRPCRestoreCondition),
		Reason:  string(s),
		Status:  s.Status(),
		Message: s.Message(),
	}
}

func (s StarknetRPCRestoreStatus) Apply() condition.StateTransition {
	return func(rpc *v1alpha1.StarknetRPC) {
		meta.SetStatusCondition(&rpc.Status.Conditions, s.AsCondition())
	}
}
