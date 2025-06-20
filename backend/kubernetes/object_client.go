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

type patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type object[T any] interface {
	name() string
	mutate(mutate func(object T) error) (T, []patch, error)
	checkUpdate(newVersion T) bool
}

type objectCRUD[T object[T]] interface {
	// Create creates the specified object in the Kubernetes database. This operation waits until the local cache
	// has been updated.
	create(ctx context.Context, object T) error
	// Delete deletes the specified object by name and kind from the Kubernetes database. This operation waits until
	// the local cache has been updated.
	delete(ctx context.Context, name string) error
	// Get returns the cached version of the object by name.
	get(ctx context.Context, name string) (T, error)
	// Set updates the specified object by name using the specified mutator function.
	set(ctx context.Context, name string, mutate func(object T) error) error
	// Close cleans up the CRUD provider.
	close(ctx context.Context) error
}

func newObjectCRUD[T object[T]](
	ctx context.Context,
	dynamicClient *dynamic.DynamicClient,
	namespace string,
	kind string,
	groupVersionResource schema.GroupVersionResource,
	logger *slog.Logger,
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
		result = backend.ErrBackendRequestFailed.Wrap(err).
			WithAttr(slog.String("reason", string(kubeError.Status().Reason))).
			WithAttr(slog.String("status", kubeError.Status().Status))
	}
	result = result.WithAttr(slog.String("namespace", o.namespace))
	result = result.WithAttr(slog.String("kind", o.kind))
	if name != "" {
		result = result.WithAttr(slog.String("name", name))
	}

	return result
}

func (o *objectClient[T]) create(ctx context.Context, object T) error { //nolint:unused // This is used through objectCRUD
	// We need to re-encode the object into Unstructured. Unfortunately, the only way to do this is to marshal
	// everything into a JSON string and then unmarshal it. The mapstructure library doesn't respect some of the JSON
	// tags.
	data, err := json.Marshal(object)
	if err != nil {
		return err
	}
	unstructuredObj := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	if err := json.Unmarshal(data, &unstructuredObj.Object); err != nil {
		return err
	}
	if _, err := o.dynamicClient.Resource(o.groupVersionResource).Namespace(o.namespace).Create(
		ctx,
		unstructuredObj,
		v1.CreateOptions{
			TypeMeta: v1.TypeMeta{
				Kind:       kind,
				APIVersion: groupVersion.String(),
			},
		},
	); err != nil {
		return o.processError(err, object.name())
	}
	return o.createWait.wait(ctx, object, func(object T) (bool, error) {
		_, ok := o.objects[object.name()]
		return ok, nil
	})
}

func (o *objectClient[T]) delete(ctx context.Context, name string) error { //nolint:unused // This is used through objectCRUD
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
	return o.deleteWait.wait(ctx, original, func(object T) (bool, error) {
		_, ok := o.objects[object.name()]
		return !ok, nil
	})
}

func (o *objectClient[T]) get(_ context.Context, name string) (T, error) { //nolint:unused // This is used through objectCRUD
	o.lock.RLock()
	defer o.lock.RUnlock()
	object, ok := o.objects[name]
	if !ok {
		return object, backend.ErrDomainNotInBackend.
			WithAttr(slog.String("name", name)).
			WithAttr(slog.String("namespace", o.namespace)).
			WithAttr(slog.String("kind", o.kind))
	}
	return object, nil
}

func (o *objectClient[T]) set(ctx context.Context, name string, mutate func(object T) error) error { //nolint:unused // This is used through objectCRUD
	ctx = o.getLoggerContext(ctx)

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
			Resource(groupVersionResource).
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
			Wrap(fmt.Errorf("exhausted retries while trying to update domain")).
			WithAttr(slog.String("name", name)).
			WithAttr(slog.String("namespace", o.namespace)).
			WithAttr(slog.String("kind", o.kind))
	}
	return o.waitForUpdate(ctx, object)
}

func (o *objectClient[T]) waitForUpdate(ctx context.Context, object T) error { //nolint:unused // This is used
	return o.updateWait.wait(ctx, object, func(object T) (bool, error) {
		obj, ok := o.objects[object.name()]
		if !ok {
			return false, backend.ErrBackendRequestFailed.Wrap(fmt.Errorf("object deleted whiile waiting for update"))
		}
		return object.checkUpdate(obj), nil
	})
}

func (o *objectClient[T]) loadAndWatch(ctx context.Context) error {
	// We opt to explicitly fetch the domain list so we can return an error if the fetch doesn't work.
	// The goal is to make sure the DNS server is actually ready once the startup completes.
	ctx = o.getLoggerContext(ctx)
	unstructuredList, err := o.dynamicClient.
		Resource(groupVersionResource).
		Namespace(o.namespace).
		List(ctx, v1.ListOptions{})
	if err != nil {
		return o.processError(err, "")
	}
	for _, domain := range unstructuredList.Items {
		o.onAdd(domain)
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		o.dynamicClient,
		0,
		o.namespace,
		nil,
	)
	informer := factory.ForResource(groupVersionResource).Informer()

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

func (o *objectClient[T]) close(_ context.Context) error {
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
	newObject, err := o.unstructuredToDomain(object)
	if err != nil {
		panic(err)
	}

	o.lock.Lock()
	defer o.lock.Unlock()
	o.logger.Debug("Kubernetes cluster reports object added", slog.String("name", newObject.name()))
	o.objects[newObject.name()] = newObject
	if err := o.createWait.submit(newObject); err != nil {
		panic(err)
	}
}

func (o *objectClient[T]) onUpdate(oldObjectAny, newObjectAny any) {
	oldObject, err := o.unstructuredToDomain(oldObjectAny.(*unstructured.Unstructured))
	if err != nil {
		panic(err)
	}
	newObject, err := o.unstructuredToDomain(newObjectAny.(*unstructured.Unstructured))
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
	if oldObject.name() != newObject.name() {
		delete(o.objects, oldObject.name())
		if err := o.deleteWait.submit(oldObject); err != nil {
			panic(err)
		}
	}
	if err := o.updateWait.submit(newObject); err != nil {
		panic(err)
	}
}

func (o *objectClient[T]) onDelete(object any) {
	oldDomain, err := o.unstructuredToDomain(object.(*unstructured.Unstructured))
	if err != nil {
		panic(err)
	}

	o.lock.Lock()
	defer o.lock.Unlock()
	o.logger.Debug("Kubernetes cluster reports object deleted", slog.String("name", oldDomain.name()))
	obj := o.objects[oldDomain.name()]
	delete(o.objects, oldDomain.name())
	if err := o.deleteWait.submit(obj); err != nil {
		panic(err)
	}
}

func (o *objectClient[T]) unstructuredToDomain(obj any) (T, error) {
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
