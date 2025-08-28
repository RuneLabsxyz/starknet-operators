package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	"github.com/runelabs-xyz/starknet-operators/internal/utils/condition"
	errs "github.com/runelabs-xyz/starknet-operators/internal/utils/reconciler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *StarknetRPCReconciler) ReconcileArchiveRestore(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	contextLogger := log.FromContext(ctx)

	// If archive is already made, return early
	if meta.IsStatusConditionTrue(cluster.Status.Conditions, "Archive") {
		// Delete the job if it still exists
		restoreJob := r.GetWantedRestoreJob(cluster)
		err := r.Client.Delete(ctx, &restoreJob)
		if err != nil && !apierrs.IsNotFound(err) {
			return nil, err
		}

		return &ctrl.Result{}, nil
	}

	// If archive is not needed, return early
	if !cluster.Spec.RestoreArchive.Enable {
		// Mark the archive restore as skipped
		condition.SetPhases(ctx, r.Client, cluster, markRestoreAsSkipped)
		return &ctrl.Result{}, nil
	}

	// Create PVC (if it not already exists)
	restorePvc := r.GetWantedRestorePvc(cluster)
	if err := r.Create(ctx, &restorePvc); err != nil && !apierrs.IsAlreadyExists(err) {
		return nil, err
	}

	if !isReady(&restorePvc) {
		contextLogger.V(4).Info("Archive PVC is not ready yet", "pvc", restorePvc.Name)

		return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
	}

	// Also validate that the base PVC is ready
	result, err := r.EnsurePvcReady(ctx, cluster)
	if err != nil {
		if err == errs.ErrNextLoop {
			return result, err
		}
	}

	// We can now create the job that is going to restore.
	// Create PVC (if it not already exists)
	restoreJob := r.GetWantedRestoreJob(cluster)
	err = r.Create(ctx, &restoreJob)
	if err == nil {
		condition.SetPhases(ctx, r.Client, cluster, markRestoreAsProgressing)
		// We just created the job, so early exit
		return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
	} else if !apierrs.IsAlreadyExists(err) {
		return nil, err
	}

	if restoreJob.Status.Succeeded > 0 {
		// Mark the job as completed
		condition.SetPhases(ctx, r.Client, cluster, markArchiveAsFinished)

		// We completed the archive! Let's re-run the loop to continue the setup
		return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
	} else if restoreJob.Status.Failed > 0 {
		// Mark the job as completed
		condition.SetPhases(ctx, r.Client, cluster, markRestoreAsFailed)
		// We completed the archive! Let's re-run the loop to continue the setup
		return nil, errs.ErrTerminateLoop
	} else {
		// Re-schedule after 30 seconds
		// The process takes multiple minutes, so we don't want to schedule as frequently
		return &ctrl.Result{RequeueAfter: time.Duration(30) * time.Second}, errs.ErrNextLoop
	}

}

func markArchiveAsFinished(cluster *v1alpha1.StarknetRPC) {
	// Modify the state
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Restore",
		Status:  metav1.ConditionTrue,
		Reason:  "ArchiveJobFinished",
		Message: "The archive restore job has successfully run",
	})
}

func markRestoreAsFailed(cluster *v1alpha1.StarknetRPC) {
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Restore",
		Status:  metav1.ConditionFalse,
		Reason:  "ArchiveJobFailed",
		Message: "The archive restore job has failed",
	})
}

func markRestoreAsSkipped(cluster *v1alpha1.StarknetRPC) {
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Restore",
		Status:  metav1.ConditionTrue,
		Reason:  "ArchiveRestoreSkipped",
		Message: "The configuration was configured to skip the archive restore process",
	})
}

func markRestoreAsProgressing(cluster *v1alpha1.StarknetRPC) {
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Restore",
		Status:  metav1.ConditionFalse,
		Reason:  "ArchiveJobProgressing",
		Message: "The archive restore job is currently running",
	})
}

func (r *StarknetRPCReconciler) GetRestoreJobName(cluster *v1alpha1.StarknetRPC) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-archive-restore-job", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

func getImage(snapshot *v1alpha1.ArchiveSnapshot) string {
	if snapshot.RestoreImage == nil {
		return "ghcr.io/runelabsxyz/pathfinder-snapshotter:latest"
	} else {
		return *snapshot.RestoreImage
	}
}

func (r *StarknetRPCReconciler) GetWantedRestoreJob(cluster *v1alpha1.StarknetRPC) batchv1.Job {
	nameInfo := r.GetRestoreJobName(cluster)
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        nameInfo.Name,
			Namespace:   nameInfo.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					// TODO: Handle failures ourselves
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "archive-downloader",
							Image: getImage(&cluster.Spec.RestoreArchive),
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "PATHFINDER_NETWORK",
									Value: cluster.Spec.Network,
								},
								corev1.EnvVar{
									Name:  "PATHFINDER_FILE_NAME",
									Value: cluster.Spec.RestoreArchive.FileName,
								},
								corev1.EnvVar{
									Name:  "PATHFINDER_CHECKSUM",
									Value: cluster.Spec.RestoreArchive.Checksum,
								},
								corev1.EnvVar{
									Name:  "PATHFINDER_DOWNLOAD_URL",
									Value: *cluster.Spec.RestoreArchive.RsyncConfig,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									Name:      "snapshot-scratch",
									MountPath: "/scratch",
								},
								corev1.VolumeMount{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: r.GetStoragePvcName(cluster).Name,
								},
							},
						},
						corev1.Volume{
							Name: "snapshot-scratch",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: r.GetRestorePvcName(cluster).Name,
								},
							},
						},
					},
				},
			},
		},
	}

	controllerutil.SetControllerReference(cluster, &job, r.Scheme)

	return job
}

func (r *StarknetRPCReconciler) GetRestorePvcName(cluster *v1alpha1.StarknetRPC) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-archive-restore", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

func (r *StarknetRPCReconciler) GetWantedRestorePvc(cluster *v1alpha1.StarknetRPC) corev1.PersistentVolumeClaim {
	nameInfo := r.GetRestorePvcName(cluster)
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        nameInfo.Name,
			Namespace:   nameInfo.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &cluster.Spec.Storage.StorageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: cluster.Spec.Storage.Size,
				},
			},
		},
	}

	controllerutil.SetControllerReference(cluster, &pvc, r.Scheme)

	return pvc
}
