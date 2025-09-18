/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rpccondition "github.com/runelabs-xyz/starknet-operators/internal/utils/condition/starknetrpc"

	pathfinderv1alpha1 "github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	errs "github.com/runelabs-xyz/starknet-operators/internal/utils/reconciler"
)

//+kubebuilder:rbac:groups=pathfinder.runelabs.xyz,resources=starknetrpcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pathfinder.runelabs.xyz,resources=starknetrpcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pathfinder.runelabs.xyz,resources=starknetrpcs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/proxy,verbs=get;create
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// StarknetRPCReconciler reconciles a StarknetRPC object
type StarknetRPCReconciler struct {
	kubernetes.Interface
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StarknetRPC object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *StarknetRPCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the StarknetRPC instance
	rpc := &pathfinderv1alpha1.StarknetRPC{}
	if err := r.Get(ctx, req.NamespacedName, rpc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("Reconciling StarknetRPC", "name", rpc.Name)

	// Ensure that the conditions are initialized
	err := rpccondition.Initialize(ctx, r.Client, rpc)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 2. We need to setup the main PVC
	result, err := r.ReconcilePvc(ctx, rpc)
	if err != nil {
		if err == errs.ErrNextLoop {
			logger.V(1).Info("ReconcilePvc re-scheduled", "error", err)
			return *result, nil
		}
		logger.Error(err, "Error while reconciling PVC")
		return ctrl.Result{}, err
	}

	// Wait for the archival to complete (if enabled)
	result, err = r.ReconcileArchiveRestore(ctx, rpc)
	if err != nil {
		if err == errs.ErrNextLoop {
			return *result, nil
		}
		logger.Error(err, "Error while reconciling archive restore")
		return ctrl.Result{}, err
	}

	// Wait if the restore is not made yet
	if !meta.IsStatusConditionTrue(rpc.Status.Conditions, "Restore") {
		logger.V(1).Info("Archive is not ready, re-scheduled", "error", err)
		return ctrl.Result{RequeueAfter: time.Duration(30) * time.Second}, nil
	}

	result, err = r.ReconcilePod(ctx, rpc)
	if err != nil {
		if err == errs.ErrNextLoop {
			return *result, nil
		}
		logger.Error(err, "Error while reconciling pod")
		return ctrl.Result{}, err
	}

	// If the archive was restored correctly, we can finally create the pod.
	// TODO: Monitor the sync status
	// TODO: If the status is ready, change the syncstatus condition to true
	// TODO: Re-create the pod if it is missing, and reset the status conditions

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StarknetRPCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pathfinderv1alpha1.StarknetRPC{}).
		Owns(&corev1.Pod{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("starknetrpc").
		Complete(r)
}
