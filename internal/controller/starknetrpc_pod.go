package controller

import (
	"context"
	"fmt"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/proxy"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *StarknetRPCReconciler) ReconcilePod(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	// Create PVC (if it not already exists)
	pod := r.GetWantedPod(cluster)
	err := r.Create(ctx, &pod)
	if err == nil {
		// Mark the pod as created & pending
		condition.SetPhases(ctx, r.Client, cluster, markRpcAsPending)
	}

	if err != nil && !apierrs.IsAlreadyExists(err) {
		return nil, err
	}

	// Fetch the pod
	fetchedPod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, fetchedPod); err != nil {
		return nil, err
	}

	// Try to make a request to the pod
	if ok, err := proxy.IsReady(ctx, r.Interface, cluster, fetchedPod); err == nil && ok {
		condition.SetPhases(ctx, r.Client, cluster, markRpcAsCatchingUp)
	}

	// For now, stop here.
	// TODO: Check for the sync status

	return &ctrl.Result{}, nil
}

func markRpcAsPending(cluster *v1alpha1.StarknetRPC) {
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Available",
		Status:  metav1.ConditionFalse,
		Reason:  "Pending",
		Message: "Starting RPC node",
	})
}

func markRpcAsCatchingUp(cluster *v1alpha1.StarknetRPC) {
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Available",
		Status:  metav1.ConditionFalse,
		Reason:  "CatchingUp",
		Message: "Syncing with the blockchain",
	})
}

func markRpcAsReady(cluster *v1alpha1.StarknetRPC) {
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Available",
		Status:  metav1.ConditionTrue,
		Reason:  "CatchingUp",
		Message: "Syncing with the blockchain",
	})
}

func (r *StarknetRPCReconciler) GetPodName(cluster *v1alpha1.StarknetRPC) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-rpc", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

func getPodImage(rpc *v1alpha1.StarknetRPC) string {
	if rpc.Spec.Image != nil {
		return *rpc.Spec.Image
	} else {
		return "eqlabs/pathfinder:v0.20.0"
	}
}
func (r *StarknetRPCReconciler) GetWantedPod(cluster *v1alpha1.StarknetRPC) corev1.Pod {
	var userId int64 = 1000

	nameInfo := r.GetPodName(cluster)
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        nameInfo.Name,
			Namespace:   nameInfo.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "rpc-pathfinder",
					Image:           getPodImage(cluster),
					ImagePullPolicy: corev1.PullIfNotPresent,
					// Useful Env variables
					Env: []corev1.EnvVar{
						{
							Name:  "RUST_LOG",
							Value: "info",
						},
						{
							Name: "PATHFINDER_DATA_DIR",
							// Default emplacement, and makes it easy to get
							Value: "/usr/share/pathfinder/data",
						},
						{
							Name: "PATHFINDER_MONITOR_ADDRESS",
							// Arbitrary port, not sure if it is needed to be configurable
							Value: "0.0.0.0:9000",
						},
						{
							Name: "PATHFINDER_ETHEREUM_API_URL",
							// Arbitrary port, not sure if it is needed to be configurable
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &cluster.Spec.Layer1RpcSecret,
							},
						},
					},
					Resources: cluster.Spec.Resources,
					Ports: []corev1.ContainerPort{
						{
							Name:          "rpc",
							ContainerPort: 9545,
						},
						{
							Name:          "monitoring",
							ContainerPort: 9000,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "pathfinder-data",
							MountPath: "/usr/share/pathfinder/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "pathfinder-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: r.GetStoragePvcName(cluster).Name,
						},
					},
				},
			},
			// Default security context. Cannot be modified for now
			// TODO: Support custom security context for custom images
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  &userId,
				RunAsGroup: &userId,
				FSGroup:    &userId,
			},
		},
	}

	controllerutil.SetControllerReference(cluster, &pod, r.Scheme)

	return pod
}
