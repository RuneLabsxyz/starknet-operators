package controller

import (
	"context"
	"fmt"
	"maps"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/reconciler"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ReconcilePodMonitor reconciles the PodMonitor for the StarknetRPC pod
func (r *StarknetRPCReconciler) ReconcilePodMonitor(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	contextLogger := log.FromContext(ctx)

	// Check if PodMonitor is enabled
	if cluster.Spec.PodMonitor == nil || !cluster.Spec.PodMonitor.Enabled {
		// Check if there's an existing PodMonitor that should be deleted
		existingPodMonitor := &monitoringv1.PodMonitor{}
		nameInfo := r.GetPodMonitorName(cluster)
		err := r.Get(ctx, nameInfo, existingPodMonitor)
		if err == nil {
			// PodMonitor exists but shouldn't, delete it
			contextLogger.Info("PodMonitor is disabled, removing existing PodMonitor", "name", existingPodMonitor.Name)
			if err := r.Delete(ctx, existingPodMonitor); err != nil {
				return nil, fmt.Errorf("failed to delete disabled PodMonitor: %w", err)
			}
			r.Recorder.Event(cluster, "Normal", "PodMonitorDeleted",
				fmt.Sprintf("PodMonitor %s deleted as monitoring is disabled", existingPodMonitor.Name))
		}
		contextLogger.V(1).Info("PodMonitor is not enabled, skipping creation")
		return &ctrl.Result{}, nil
	}

	// Check if the pod exists first
	pod := r.GetWantedPod(cluster)
	if err := r.Get(ctx, r.GetPodName(cluster), &pod); err != nil {
		if apierrs.IsNotFound(err) {
			contextLogger.V(1).Info("Pod does not exist yet, skipping PodMonitor creation")
			return &ctrl.Result{}, nil
		}
		return nil, err
	}

	// Create or reconcile the PodMonitor
	podMonitor := r.GetWantedPodMonitor(cluster)

	created, err := reconciler.CreateOrReconcile(ctx, r.Client, &podMonitor,
		PodMonitorSpecReconciler(cluster),
	)
	if err != nil {
		if apierrs.IsNotFound(err) {
			// CRD might not be installed
			contextLogger.Info("PodMonitor CRD not found, skipping monitoring setup. Install Prometheus Operator to enable monitoring.")
			return &ctrl.Result{}, nil
		}
		return nil, err
	}

	if created {
		contextLogger.Info("PodMonitor created", "name", podMonitor.Name)
		r.Recorder.Event(cluster, "Normal", "PodMonitorCreated",
			fmt.Sprintf("PodMonitor %s created for monitoring", podMonitor.Name))
	}

	return &ctrl.Result{}, nil
}

// PodMonitorSpecReconciler ensures the PodMonitor spec is up to date
func PodMonitorSpecReconciler(rpc *v1alpha1.StarknetRPC) reconciler.ObjectReconcilier[*monitoringv1.PodMonitor] {
	contextLogger := log.Log
	return reconciler.ObjectReconcilier[*monitoringv1.PodMonitor]{
		Name: "PodMonitorSpecReconciler",
		IsUpToDate: func(podMonitor *monitoringv1.PodMonitor) bool {
			// Check if the spec matches what we want
			expected := getPodMonitorSpec(rpc)
			expectedLabels := getPodMonitorLabels(rpc)

			// Check if labels match
			for k, v := range expectedLabels {
				if podMonitor.Labels[k] != v {
					contextLogger.V(1).Info("PodMonitor label mismatch", "key", k, "current", podMonitor.Labels[k], "expected", v)
					return false
				}
			}

			// Compare key fields
			if len(podMonitor.Spec.PodMetricsEndpoints) != len(expected.PodMetricsEndpoints) {
				contextLogger.V(1).Info("PodMonitor spec mismatch", "current", len(podMonitor.Spec.PodMetricsEndpoints), "expected", len(expected.PodMetricsEndpoints))
				return false
			}

			if len(podMonitor.Spec.PodMetricsEndpoints) > 0 && len(expected.PodMetricsEndpoints) > 0 {
				current := podMonitor.Spec.PodMetricsEndpoints[0]
				want := expected.PodMetricsEndpoints[0]

				if *current.Port != *want.Port || current.Path != want.Path {
					contextLogger.V(1).Info("PodMonitor spec mismatch (port or path mismatch)", "current", current, "expected", want)
					return false
				}
			}

			// Check selector
			currentSelector := podMonitor.Spec.Selector
			expectedSelector := expected.Selector

			// Compare MatchLabels
			if len(currentSelector.MatchLabels) != len(expectedSelector.MatchLabels) {
				contextLogger.V(1).Info("PodMonitor spec mismatch (labels len mismatch)", "current", currentSelector.MatchLabels, "expected", expectedSelector.MatchLabels)
				return false
			}
			for k, v := range expectedSelector.MatchLabels {
				if currentSelector.MatchLabels[k] != v {
					contextLogger.V(1).Info("PodMonitor spec mismatch (labels mismatch)", "current", currentSelector.MatchLabels[k], "expected", v)
					return false
				}
			}

			// Compare MatchExpressions
			if len(currentSelector.MatchExpressions) != len(expectedSelector.MatchExpressions) {
				contextLogger.V(1).Info("PodMonitor spec mismatch (match expressions len mismatch)", "current", currentSelector.MatchExpressions, "expected", expectedSelector.MatchExpressions)
				return false
			}

			return true
		},
		Update: func(podMonitor *monitoringv1.PodMonitor) error {
			podMonitor.Spec = getPodMonitorSpec(rpc)
			// Update labels
			if podMonitor.Labels == nil {
				podMonitor.Labels = make(map[string]string)
			}
			maps.Copy(podMonitor.Labels, getPodMonitorLabels(rpc))
			return nil
		},
	}
}

// GetPodMonitorName returns the name and namespace for the PodMonitor
func (r *StarknetRPCReconciler) GetPodMonitorName(cluster *v1alpha1.StarknetRPC) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-podmonitor", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

// getPodMonitorSpec returns the desired PodMonitor spec
func getPodMonitorSpec(cluster *v1alpha1.StarknetRPC) monitoringv1.PodMonitorSpec {
	portName := "monitoring"
	return monitoringv1.PodMonitorSpec{
		PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
			{
				Port: &portName, // This matches the port name in the pod spec
				Path: "/metrics",
				// Interval is not set, so it will use the default Prometheus scrape interval
			},
		},
		Selector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"rpc.runelabs.xyz/name": cluster.Name,
			},
		},
		NamespaceSelector: monitoringv1.NamespaceSelector{
			MatchNames: []string{cluster.Namespace},
		},
		// Attach additional metadata to the scraped metrics
		PodTargetLabels: []string{
			"runelabs.xyz/network",
			"rpc.runelabs.xyz/name",
		},
	}
}

// getPodMonitorLabels returns the labels for the PodMonitor resource
func getPodMonitorLabels(cluster *v1alpha1.StarknetRPC) map[string]string {
	labels := map[string]string{
		"rpc.runelabs.xyz/type":        "starknet",
		"rpc.runelabs.xyz/name":        cluster.Name,
		"runelabs.xyz/network":         cluster.Spec.Network,
		"app.kubernetes.io/name":       "starknet-rpc",
		"app.kubernetes.io/instance":   cluster.Name,
		"app.kubernetes.io/managed-by": "starknet-operator",
	}

	// Add custom labels if provided
	if cluster.Spec.PodMonitor != nil && cluster.Spec.PodMonitor.Labels != nil {
		maps.Copy(labels, cluster.Spec.PodMonitor.Labels)
	}

	return labels
}

// GetWantedPodMonitor returns the desired PodMonitor resource
func (r *StarknetRPCReconciler) GetWantedPodMonitor(cluster *v1alpha1.StarknetRPC) monitoringv1.PodMonitor {
	nameInfo := r.GetPodMonitorName(cluster)
	labels := getPodMonitorLabels(cluster)

	podMonitor := monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: make(map[string]string),
			Name:        nameInfo.Name,
			Namespace:   nameInfo.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "pathfinder.runelabs.xyz/v1alpha1",
					Kind:               "StarknetRPC",
					Name:               cluster.Name,
					UID:                cluster.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
		Spec: getPodMonitorSpec(cluster),
	}

	return podMonitor
}
