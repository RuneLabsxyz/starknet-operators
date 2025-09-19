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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *StarknetRPCReconciler) ReconcileArchiveRestore(ctx context.Context, cluster *v1alpha1.StarknetRPC) (*ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Show what status we are in
	logger.Info("Reconciling archive restore", "conditions", cluster.Status.Conditions)

	// If archive is already made, return early
	if meta.IsStatusConditionTrue(cluster.Status.Conditions, "Restore") {
		logger.V(1).Info("Archive already made, cleaning up")
		// Delete the job if it still exists
		restoreJob := r.GetWantedRestoreJob(cluster)

		deletePropagation := metav1.DeletePropagationBackground

		err := r.Delete(ctx, &restoreJob, &client.DeleteOptions{
			PropagationPolicy: &deletePropagation,
		})
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}

		// Also Delete the PVC if it still exists
		restorePvc := r.GetWantedRestorePvc(cluster)
		err = r.Delete(ctx, &restorePvc)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}

		return &ctrl.Result{}, nil
	}

	// If archive is not needed, return early
	if cluster.Spec.RestoreArchive.Enable != nil && !*cluster.Spec.RestoreArchive.Enable {
		logger.V(1).Info("Archive restore not enabled")
		// Mark the archive restore as skipped
		if err := condition.SetPhases(ctx, r.Client, cluster, markRestoreAsSkipped); err != nil {
			return nil, err
		}
		return &ctrl.Result{}, nil
	}

	// Create PVC (if it not already exists)
	restorePvc := r.GetWantedRestorePvc(cluster)
	if err := r.Create(ctx, &restorePvc); err != nil && !apierrs.IsAlreadyExists(err) {
		return nil, err
	}

	// Fetch the PVC
	if err := r.Get(ctx, types.NamespacedName{Name: restorePvc.Name, Namespace: restorePvc.Namespace}, &restorePvc); err != nil {
		if apierrs.IsNotFound(err) {
			return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
		}
		logger.V(1).Error(err, "Error while fetching PVC")
		return nil, err
	}

	if !isReady(&restorePvc) {
		logger.V(1).Info("Archive PVC is not ready yet", "pvc", restorePvc.Name)

		return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
	}

	// Also validate that the base PVC is ready
	result, err := r.EnsurePvcReady(ctx, cluster)
	if err != nil {
		if err == errs.ErrNextLoop {
			logger.V(1).Info("Base PVC is not ready yet")
			return result, err
		}

		logger.Error(err, "Error while reconciling PVC")
		return nil, err
	}

	// We can now create the job that is going to restore.
	// Create PVC (if it not already exists)
	restoreJob := r.GetWantedRestoreJob(cluster)
	err = r.Create(ctx, &restoreJob)
	if err == nil {
		logger.V(1).Info("Created restore job")
		err := condition.SetPhases(ctx, r.Client, cluster, markRestoreAsProgressing)
		if err != nil {
			return nil, err
		}
		// We just created the job, so early exit
		return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
	} else if !apierrs.IsAlreadyExists(err) {
		return nil, err
	}

	// Read the job
	err = r.Get(ctx, types.NamespacedName{Name: restoreJob.Name, Namespace: restoreJob.Namespace}, &restoreJob)
	if err != nil {
		return nil, err
	}

	if restoreJob.Status.Succeeded > 0 {
		// Mark the job as completed
		err := condition.SetPhases(ctx, r.Client, cluster, markArchiveAsFinished)
		if err != nil {
			return nil, err
		}

		// We just created the job, so early exit
		return &ctrl.Result{RequeueAfter: time.Second}, errs.ErrNextLoop
	} else if restoreJob.Status.Failed > 0 {
		logger.V(1).Info("Restore failed!")
		// Mark the job as completed
		err := condition.SetPhases(ctx, r.Client, cluster, markRestoreAsFailed)
		if err != nil {
			return nil, err
		}
		// We completed the archive! Let's re-run the loop to continue the setup
		return nil, errs.ErrTerminateLoop
	} else {
		logger.V(1).Info("Restore still in progress, waiting for completion")
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

func getEnvVars(cluster *v1alpha1.StarknetRPC) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "PATHFINDER_NETWORK",
			Value: cluster.Spec.Network,
		},
		{
			Name:  "PATHFINDER_FILE_NAME",
			Value: cluster.Spec.RestoreArchive.FileName,
		},
		{
			Name:  "PATHFINDER_CHECKSUM",
			Value: cluster.Spec.RestoreArchive.Checksum,
		},
	}

	if cluster.Spec.RestoreArchive.RsyncConfig != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PATHFINDER_DOWNLOAD_URL",
			Value: *cluster.Spec.RestoreArchive.RsyncConfig,
		})
	}

	return envVars
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
						{
							Name:  "archive-downloader",
							Image: getImage(&cluster.Spec.RestoreArchive),
							Env:   getEnvVars(cluster),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "snapshot-scratch",
									MountPath: "/scratch",
								},
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: r.GetStoragePvcName(cluster).Name,
								},
							},
						},
						{
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
