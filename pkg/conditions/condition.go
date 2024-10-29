package conditions

import (
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Conditioner interface {
	SetCondition(condition metav1.Condition)
}

type Patcher func(Conditioner)

type Ready struct{}

// NewReady creates a new Ready instance.
func NewReady() *Ready {
	return &Ready{}
}

// PatcherFailed returns a Patcher that sets the Ready condition to failed with the provided message.
func (p *Ready) PatcherFailed(msg string) Patcher {
	return func(c Conditioner) {
		SetReadyFailedWithMessage(c, msg)
	}
}

// PatcherWithError returns a Patcher that sets the Ready condition based on the presence of an error.
func (p *Ready) PatcherWithError(err error) Patcher {
	return func(c Conditioner) {
		if err == nil {
			SetReadyCreated(c)
		} else {
			SetReadyFailed(c)
		}
	}
}

// PatcherRefResolver returns a Patcher that sets the Ready condition based on the presence of an error and the type of object.
func (p *Ready) PatcherRefResolver(err error, obj interface{}) Patcher {
	return func(c Conditioner) {
		if err == nil {
			return
		}
		if apierrors.IsNotFound(err) {
			SetReadyFailedWithMessage(c, fmt.Sprintf("%s not found", getType(obj)))
			return
		}
		SetReadyFailedWithMessage(c, fmt.Sprintf("Error getting %s", getType(obj)))
	}
}

// PatcherHealthy returns a Patcher that sets the Ready condition to healthy or unhealthy based on the presence of an error.
func (p *Ready) PatcherHealthy(err error) Patcher {
	return func(c Conditioner) {
		if err == nil {
			SetReadyHealthy(c)
		} else {
			SetReadyUnhealthyWithError(c, err)
		}
	}
}

type Complete struct {
	client client.Client
}

// NewComplete creates a new Complete instance with the provided client.
func NewComplete(client client.Client) *Complete {
	return &Complete{
		client: client,
	}
}

// PatcherFailed returns a Patcher that sets the Complete condition to failed with the provided message.
func (p *Complete) PatcherFailed(msg string) Patcher {
	return func(c Conditioner) {
		SetCompleteFailedWithMessage(c, msg)
	}
}

// PatcherRefResolver returns a Patcher that sets the Complete condition based on the presence of an error and the type of object.
func (p *Complete) PatcherRefResolver(err error, obj runtime.Object) Patcher {
	return func(c Conditioner) {
		if err == nil {
			return
		}
		if apierrors.IsNotFound(err) {
			SetCompleteFailedWithMessage(c, fmt.Sprintf("%s not found", getType(obj)))
			return
		}
		SetCompleteFailedWithMessage(c, fmt.Sprintf("Error getting %s", getType(obj)))
	}
}

// getType returns the type name of the provided object.
func getType(obj interface{}) string {
	if t := reflect.TypeOf(obj); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}
