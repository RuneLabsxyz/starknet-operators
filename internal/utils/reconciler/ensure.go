package reconciler

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ObjectReconcilier[T client.Object] struct {
	Name       string
	IsUpToDate func(T) bool
	Update     func(T) error
}

// CreateOrReconcile creates (if the object does not exist) or reconciles (if the object exists) the given object.
//
// Returns true if the object was created, false otherwise.
func CreateOrReconcile[T client.Object](ctx context.Context, c client.Client, object T, reconcilers ...ObjectReconcilier[T]) (bool, error) {
	logger := log.FromContext(ctx, "object", client.ObjectKeyFromObject(object))
	// First, create the object if it does not exist
	err := c.Create(ctx, object)
	if client.IgnoreAlreadyExists(err) != nil {
		logger.Error(err, "Failed to create object")
		return true, err
	} else if !errors.IsAlreadyExists(err) {
		logger.Info("Created object!")
		// You will need a reconciliation to get the latest value anyways
		return true, nil
	}

	// Get the latest version of the object
	if err := c.Get(ctx, client.ObjectKeyFromObject(object), object); err != nil {
		return false, err
	}

	// Then, reconcile the object if it exists
	for _, reconciler := range reconcilers {
		if !reconciler.IsUpToDate(object) {
			if err := reconciler.Update(object); err != nil {
				return false, err
			}
		}
	}

	return false, nil
}
