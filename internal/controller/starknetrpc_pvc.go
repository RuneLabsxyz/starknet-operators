package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition/starknetrpc"
	reconcilier "github.com/runelabs-xyz/starknet-operators/internal/utils/reconciler"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *StarknetRPCReconciler) ReconcilePvc(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	contextLogger := log.FromContext(ctx)
	// Create PVC (if it not already exists)
	pvc := r.GetWantedPvc(cluster)
	err := r.Create(ctx, &pvc)
	if err == nil {
		contextLogger.V(1).Info("PVC created", "name", pvc.Name)

		// We need to reset the archive status!
		err := condition.SetPhases(ctx, r.Client, cluster,
			starknetrpc.StarknetRPCRestoreStatusPending.Apply(),
		)
		if err != nil {
			return nil, err
		}

		return &ctrl.Result{}, nil
	} else if !apierrs.IsAlreadyExists(err) {
		return nil, err
	}

	return &ctrl.Result{}, nil
}

func (r *StarknetRPCReconciler) EnsurePvcReady(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	contextLogger := log.FromContext(ctx)

	// Ensure PVC is ready
	var pvc corev1.PersistentVolumeClaim

	if err := r.Get(ctx, r.GetStoragePvcName(cluster), &pvc); err != nil {
		contextLogger.V(1).Info("Failed to get PVC", "error", err)
		return nil, err
	}

	if !isReady(&pvc) {
		contextLogger.V(10).Info("PVC is not ready yet", "pvc", pvc.Name)

		return &ctrl.Result{RequeueAfter: time.Second}, reconcilier.ErrNextLoop
	}

	return &ctrl.Result{}, nil
}

func isReady(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.Status.Phase == corev1.ClaimBound
}

func (r *StarknetRPCReconciler) GetStoragePvcName(cluster *v1alpha1.StarknetRPC) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-storage", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

func (r *StarknetRPCReconciler) GetWantedPvc(cluster *v1alpha1.StarknetRPC) corev1.PersistentVolumeClaim {
	nameInfo := r.GetStoragePvcName(cluster)
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        nameInfo.Name,
			Namespace:   nameInfo.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         cluster.APIVersion,
					Kind:               cluster.Kind,
					Name:               cluster.Name,
					UID:                cluster.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &cluster.Spec.Storage.Class,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: cluster.Spec.Storage.Size,
				},
			},
		},
	}

	return pvc
}
