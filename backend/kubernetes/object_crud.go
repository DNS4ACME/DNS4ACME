package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/lang/E"
	"github.com/go-logr/logr"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"log/slog"
	"net/http"
	"sync"
)

// patch is a single entry in an RFC 6902 JSON patch request.
type patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// object is an abstraction for entities stored in Kubernetes.
type object[T any] interface {
	// name returns the name of the object in Kubernetes. This name uniquely identifies the object within a namespace
	// among objects of its kind.
	name() string
	// mutate is a function that performs a change using a mutator function and produces a new object, as well as a
	// patch object.
	mutate(mutate func(object T) error) (T, []patch, error)
	// checkUpdate returns true if the newVersion object matches the current object, or is newer than the current
	// object.
	checkUpdate(newVersion T) bool
}

type objectCRUD[T object[T]] interface {
	// create creates the specified object in the Kubernetes database. This operation waits until the local cache
	// has been updated.
	create(ctx context.Context, object T) (T, error)
	// delete deletes the specified object by name and zoneKind from the Kubernetes database. This operation waits until
	// the local cache has been updated.
	delete(ctx context.Context, name string) error
	// get returns the cached version of the object by name.
	get(ctx context.Context, name string) (T, error)
	// set updates the specified object by name using the specified mutator function.
	set(ctx context.Context, name string, mutate func(object T) error) error
	// close cleans up the CRUD provider.
	close(ctx context.Context) error
}

type changeType int

const (
	changeTypeAdd changeType = iota
	changeTypeUpdate
	changeTypeDelete
)

func newObjectCRUD[T object[T]](
	ctx context.Context,
	dynamicClient *dynamic.DynamicClient,
	namespace string,
	kind string,
	groupVersionResource schema.GroupVersionResource,
	logger *slog.Logger,
	changeHandler func(change changeType, object T, oldObject T),
) (objectCRUD[T], error) {
	lock := &sync.RWMutex{}
	cli := &objectClient[T]{
		dynamicClient:        dynamicClient,
		namespace:            namespace,
		kind:                 kind,
		groupVersionResource: groupVersionResource,
		logger:               logger.With(slog.String("kind", kind)).With(slog.String("namespace", namespace)),
		objects:              map[string]T{},
		lock:                 lock,
		createWait:           newWaiter[T](lock, slog.String("kind", kind), slog.String("namespace", namespace)),
		updateWait:           newWaiter[T](lock, slog.String("kind", kind), slog.String("namespace", namespace)),
		deleteWait:           newWaiter[T](lock, slog.String("kind", kind), slog.String("namespace", namespace)),
		changeHandler:        changeHandler,
	}
	if err := cli.loadAndWatch(ctx); err != nil {
		_ = cli.close(ctx)
		return nil, err
	}
	return cli, nil
}

type objectClient[T object[T]] struct {
	dynamicClient        *dynamic.DynamicClient
	namespace            string
	kind                 string
	groupVersionResource schema.GroupVersionResource
	logger               *slog.Logger
	objects              map[string]T
	lock                 *sync.RWMutex
	createWait           *waiter[T]
	updateWait           *waiter[T]
	deleteWait           *waiter[T]
	cancel               context.CancelFunc
	closeDone            chan struct{}
	changeHandler        func(change changeType, object T, oldObject T)
}

func (o *objectClient[T]) getLoggerContext(ctx context.Context) context.Context {
	return logr.NewContextWithSlogLogger(ctx, o.logger)
}

func (o *objectClient[T]) processError(err error, name string) error {
	var kubeError *kubeerrors.StatusError
	var result E.Error
	if !errors.As(err, &kubeError) {
		result = backend.ErrBackendRequestFailed.Wrap(err)
	} else {
		switch kubeError.Status().Code {
		case http.StatusNotFound:
			result = backend.ErrObjectNotInBackend.WithAttr(slog.String("object", o.kind))
		case http.StatusConflict:
			result = backend.ErrObjectBackendConflict.WithAttr(slog.String("object", o.kind))
		default:
			result = backend.ErrBackendRequestFailed.Wrap(err).
				WithAttr(slog.String("reason", string(kubeError.Status().Reason))).
				WithAttr(slog.String("status", kubeError.Status().Status))
		}
	}
	result = result.WithAttr(slog.String("namespace", o.namespace))
	result = result.WithAttr(slog.String("kind", o.kind))
	if name != "" {
		result = result.WithAttr(slog.String("name", name))
	}

	return result
}

func (o *objectClient[T]) create(ctx context.Context, object T) (T, error) { //nolint:unused // This is used through objectCRUD
	// We need to re-encode the object into Unstructured. Unfortunately, the only way to do this is to marshal
	// everything into a JSON string and then unmarshal it. The mapstructure library doesn't respect some of the JSON
	// tags.
	var deflt T
	data, err := json.Marshal(object)
	if err != nil {
		return deflt, err
	}
	ctx = o.getLoggerContext(ctx)
	o.logger.DebugContext(
		ctx,
		"Creating Kubernetes object",
	)
	unstructuredObj := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	if err := json.Unmarshal(data, &unstructuredObj.Object); err != nil {
		return deflt, err
	}
	unstructuredReturnObject, err := o.dynamicClient.Resource(o.groupVersionResource).Namespace(o.namespace).Create(
		ctx,
		unstructuredObj,
		v1.CreateOptions{
			TypeMeta: v1.TypeMeta{
				Kind:       zoneKind,
				APIVersion: groupVersion.String(),
			},
		},
	)
	if err != nil {
		return deflt, o.processError(err, object.name())
	}
	newObj, err := o.unstructuredToObject(unstructuredReturnObject)
	if err != nil {
		return deflt, o.processError(err, object.name())
	}
	return newObj, o.createWait.wait(ctx, newObj, func() (bool, error) {
		_, ok := o.objects[newObj.name()]
		return ok, nil
	})
}

func (o *objectClient[T]) delete(ctx context.Context, name string) error { //nolint:unused // This is used through objectCRUD
	ctx = o.getLoggerContext(ctx)
	o.logger.DebugContext(
		ctx,
		"Deleting Kubernetes object",
		slog.String("name", name),
	)
	original, err := o.get(ctx, name)
	if err != nil {
		var statusErr *kubeerrors.StatusError
		if !errors.As(err, &statusErr) || statusErr.Status().Code != http.StatusNotFound {
			return o.processError(err, original.name())
		}
		return nil
	}

	ctx = o.getLoggerContext(ctx)
	if err := o.dynamicClient.
		Resource(o.groupVersionResource).
		Namespace(o.namespace).
		Delete(ctx, original.name(), v1.DeleteOptions{}); err != nil {
		var statusErr *kubeerrors.StatusError
		if !errors.As(err, &statusErr) || statusErr.Status().Code != http.StatusNotFound {
			return o.processError(err, original.name())
		}
		return nil
	}
	o.logger.DebugContext(
		ctx,
		"Waiting for Kubernetes object deletion",
		slog.String("name", name),
	)
	return o.deleteWait.wait(ctx, original, func() (bool, error) {
		_, ok := o.objects[original.name()]
		return !ok, nil
	})
}

func (o *objectClient[T]) get(_ context.Context, name string) (T, error) { //nolint:unused // This is used through objectCRUD
	o.lock.RLock()
	defer o.lock.RUnlock()
	object, ok := o.objects[name]
	if !ok {
		return object, backend.ErrObjectNotInBackend.
			WithAttr(slog.String("name", name)).
			WithAttr(slog.String("namespace", o.namespace)).
			WithAttr(slog.String("kind", o.kind))
	}
	return object, nil
}

func (o *objectClient[T]) set(ctx context.Context, name string, mutate func(object T) error) error { //nolint:unused // This is used through objectCRUD
	ctx = o.getLoggerContext(ctx)
	o.logger.DebugContext(
		ctx,
		"Updating Kubernetes object",
		slog.String("name", name),
	)
	type Patch struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}
	var object T
	hasError := false
	for range 3 {
		original, err := o.get(ctx, name)
		if err != nil {
			return err
		}
		var changes []patch
		object, changes, err = original.mutate(mutate)
		if err != nil {
			return backend.ErrBackendRequestFailed.
				Wrap(err).
				WithAttr(slog.String("name", name)).
				WithAttr(slog.String("namespace", o.namespace)).
				WithAttr(slog.String("kind", o.kind))
		}
		encodedChanges, err := json.Marshal(changes)
		if err != nil {
			return backend.ErrBackendRequestFailed.
				Wrap(err).
				WithAttr(slog.String("name", name)).
				WithAttr(slog.String("namespace", o.namespace)).
				WithAttr(slog.String("kind", o.kind))
		}
		if _, err := o.dynamicClient.
			Resource(o.groupVersionResource).
			Namespace(o.namespace).
			Patch(ctx, name, types.JSONPatchType, encodedChanges, v1.PatchOptions{}); err != nil {
			var statusErr *kubeerrors.StatusError
			if !errors.As(err, &statusErr) || statusErr.Status().Code != http.StatusUnprocessableEntity {
				return o.processError(err, original.name())
			}
			if err := o.waitForUpdate(ctx, original); err != nil {
				return err
			}
			hasError = true
		} else {
			hasError = false
			break
		}
	}
	if hasError {
		return backend.ErrBackendRequestFailed.
			Wrap(fmt.Errorf("exhausted retries while trying to update zone")).
			WithAttr(slog.String("name", name)).
			WithAttr(slog.String("namespace", o.namespace)).
			WithAttr(slog.String("kind", o.kind))
	}
	return o.waitForUpdate(ctx, object)
}

func (o *objectClient[T]) waitForUpdate(ctx context.Context, object T) error { //nolint:unused // This is used
	return o.updateWait.wait(ctx, object, func() (bool, error) {
		obj, ok := o.objects[object.name()]
		if !ok {
			return false, backend.ErrBackendRequestFailed.Wrap(fmt.Errorf("object deleted whiile waiting for update"))
		}
		return object.checkUpdate(obj), nil
	})
}

func (o *objectClient[T]) loadAndWatch(ctx context.Context) error {
	// We opt to explicitly fetch the zone list so we can return an error if the fetch doesn't work.
	// The goal is to make sure the DNS server is actually ready once the startup completes.
	ctx = o.getLoggerContext(ctx)
	unstructuredList, err := o.dynamicClient.
		Resource(o.groupVersionResource).
		Namespace(o.namespace).
		List(ctx, v1.ListOptions{})
	if err != nil {
		return o.processError(err, "")
	}
	for _, item := range unstructuredList.Items {
		o.onAdd(item)
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		o.dynamicClient,
		0,
		o.namespace,
		nil,
	)
	informer := factory.ForResource(o.groupVersionResource).Informer()

	if _, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.onAdd,
		UpdateFunc: o.onUpdate,
		DeleteFunc: o.onDelete,
	}); err != nil {
		return err
	}
	ctx = o.getLoggerContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	o.cancel = cancel
	go func() {
		o.lock.Lock()
		o.closeDone = make(chan struct{})
		o.lock.Unlock()
		informer.RunWithContext(ctx)
		o.lock.Lock()
		defer o.lock.Unlock()
		close(o.closeDone)
	}()
	return nil
}

func (o *objectClient[T]) close(ctx context.Context) error {
	o.logger.DebugContext(
		ctx,
		"Closing Kubernetes object monitor",
	)
	o.lock.Lock()
	if o.cancel != nil {
		o.cancel()
	}
	if o.closeDone != nil {
		closeChan := o.closeDone
		o.lock.Unlock()
		<-closeChan
	} else {
		o.lock.Unlock()
	}
	return nil
}

func (o *objectClient[T]) onAdd(object any) {
	newObject, err := o.unstructuredToObject(object)
	if err != nil {
		panic(err)
	}

	o.lock.Lock()
	defer o.lock.Unlock()
	o.logger.Debug(
		"Kubernetes cluster reports object added",
		slog.String("name", newObject.name()),
	)
	o.objects[newObject.name()] = newObject
	if o.changeHandler != nil {
		var deflt T
		o.changeHandler(changeTypeAdd, newObject, deflt)
	}
	if err := o.createWait.submit(newObject); err != nil {
		panic(err)
	}
}

func (o *objectClient[T]) onUpdate(oldObjectAny, newObjectAny any) {
	oldObject, err := o.unstructuredToObject(oldObjectAny.(*unstructured.Unstructured))
	if err != nil {
		panic(err)
	}
	newObject, err := o.unstructuredToObject(newObjectAny.(*unstructured.Unstructured))
	if err != nil {
		panic(err)
	}
	o.lock.Lock()
	defer o.lock.Unlock()
	o.logger.Debug(
		"Kubernetes cluster reports object updated",
		slog.String("name", newObject.name()),
	)
	o.objects[newObject.name()] = newObject
	if o.changeHandler != nil {
		o.changeHandler(changeTypeUpdate, newObject, oldObject)
	}
	if err := o.updateWait.submit(newObject); err != nil {
		panic(err)
	}
}

func (o *objectClient[T]) onDelete(object any) {
	oldObject, err := o.unstructuredToObject(object.(*unstructured.Unstructured))
	if err != nil {
		panic(err)
	}

	o.lock.Lock()
	defer o.lock.Unlock()
	o.logger.Debug(
		"Kubernetes cluster reports object deleted",
		slog.String("name", oldObject.name()),
	)
	obj := o.objects[oldObject.name()]
	delete(o.objects, oldObject.name())
	if o.changeHandler != nil {
		var deflt T
		o.changeHandler(changeTypeDelete, deflt, oldObject)
	}
	if err := o.deleteWait.submit(obj); err != nil {
		panic(err)
	}
}

func (o *objectClient[T]) unstructuredToObject(obj any) (T, error) {
	var result T
	unstructuredObject, ok := obj.(unstructured.Unstructured)
	if !ok {
		unstructuredObjectPointer, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return result, fmt.Errorf("object is not an unstructured object (%T)", obj)
		}
		unstructuredObject = *unstructuredObjectPointer
	}
	data, err := json.Marshal(unstructuredObject.Object)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}
