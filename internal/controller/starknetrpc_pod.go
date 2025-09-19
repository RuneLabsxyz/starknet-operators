package controller

import (
	"context"
	"fmt"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition/starknetrpc"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/proxy"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *StarknetRPCReconciler) ReconcilePod(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	// Create PVC (if it not already exists)
	pod := r.GetWantedPod(cluster)

	created, err := reconciler.CreateOrReconcile(ctx, r.Client, &pod,
		ImageReconciler(cluster),
	)
	if err != nil {
		return nil, err
	} else if created {
		// Mark the pod as created & pending
		err := condition.SetPhases(ctx, r.Client, cluster,
			starknetrpc.StarknetRPCAvailableStatusCreating.Apply(),
		)
		if err != nil {
			return nil, err
		}
	}

	if shouldPodGetRecreated(&pod) {
		// It should immediately reconcile
		if err := r.Delete(ctx, &pod); err != nil {
			return nil, err
		}

		return &ctrl.Result{Requeue: true}, reconciler.ErrNextLoop

	}
	// Try to make a request to the pod
	if ok, err := proxy.IsReady(ctx, r.Interface, cluster, &pod); err == nil && ok {
		err := condition.SetPhases(ctx, r.Client, cluster,
			starknetrpc.StarknetRPCAvailableStatusCatchingUp.Apply(),
		)
		if err != nil {
			return nil, err
		}
	}

	// For now, stop here.
	// TODO: Check for the sync status

	return &ctrl.Result{}, nil
}

func ImageReconciler(rpc *v1alpha1.StarknetRPC) reconciler.ObjectReconcilier[*corev1.Pod] {
	return reconciler.ObjectReconcilier[*corev1.Pod]{
		Name: "ImageReconciler",
		IsUpToDate: func(pod *corev1.Pod) bool {
			return pod.Spec.Containers[0].Image == getPodImage(rpc)
		},
		Update: func(pod *corev1.Pod) error {
			pod.Spec.Containers[0].Image = getPodImage(rpc)
			return nil
		},
	}
}

func (r *StarknetRPCReconciler) GetPodName(cluster *v1alpha1.StarknetRPC) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-rpc", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

func shouldPodGetRecreated(pod *corev1.Pod) bool {
	// This is bad! Pods will never be restarted when evicted
	if pod.Status.Reason == "Evicted" {
		return true
	}

	for _, cs := range pod.Status.ContainerStatuses {
		// Calculate total restarts
		restartCount := cs.RestartCount

		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == "CrashLoopBackOff" && restartCount > 5 {
				// If restart count exceeds a certain threshold, consider the pod unhealthy
				return true
			}
		}
	}

	return false
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

	var labels = map[string]string{
		"rpc.runelabs.xyz/type": "starknet",
		"rpc.runelabs.xyz/name": cluster.Name,
		"runelabs.xyz/network":  cluster.Spec.Network,
	}

	nameInfo := r.GetPodName(cluster)
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
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
						{
							Name:  "PATHFINDER_WEBSOCKET_ENABLED",
							Value: "true",
						},
						{
							Name:  "PATHFINDER_HEAD_POLL_INTERVAL_SECONDS",
							Value: "2",
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
			Tolerations: cluster.Spec.Tolerations,
			// Default security context. Cannot be modified for now
			// TODO: Support custom security context for custom images
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  &userId,
				RunAsGroup: &userId,
				FSGroup:    &userId,
			},
		},
	}

	return pod
}
