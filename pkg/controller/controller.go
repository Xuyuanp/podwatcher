package controller

import (
	"fmt"

	"github.com/Xuyuanp/podwatcher/pkg/handlers"
	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	clientset kubernetes.Interface
	queue     workqueue.RateLimitingInterface
	informer  cache.SharedIndexInformer
	handler   handlers.Handler
}

func NewController(clientset kubernetes.Interface, handler handlers.Handler, namespace string) *Controller {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.CoreV1().Pods(namespace).Watch(options)
			},
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.CoreV1().Pods(namespace).List(options)
			},
		},
		&apiv1.Pod{},
		0,
		cache.Indexers{},
	)
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(old)
			if err != nil {
				glog.Errorf("Unknown object: %v", old)
				return
			}
			queue.Add(key)
		},
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				glog.Errorf("Unknown object: %v", obj)
				return
			}
			queue.Add(key)
		},
	})

	return &Controller{
		clientset: clientset,
		handler:   handler,
		informer:  informer,
		queue:     queue,
	}
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.Infof("Starting podwatcher controller")

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		c.queue.ShutDown()
		utilruntime.HandleError(fmt.Errorf("timed out for waiting for caches sync"))
		return
	}
	glog.Infof("Podwatcher controller synced and ready")
	go func() {
		<-stopCh
		glog.Infof("Podwatcher controller is shutting down")
		c.queue.ShutDown()
	}()
	c.runWorker()
}

func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)
	err := c.processItem(key.(string))
	if err == nil {
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < 3 {
		glog.Errorf("Error processing %s (will retry): %v", key, err)
		c.queue.AddRateLimited(key)
	} else {
		glog.Errorf("Error processing %s (giving up): %v", key, err)
		c.queue.Forget(key)
		utilruntime.HandleError(err)
	}
	return true
}

func (c *Controller) processItem(key string) error {
	obj, _, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("error fetching object with key %s from store: %v", key, err)
	}
	if obj == nil {
		return nil
	}
	pod := obj.(*apiv1.Pod)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			var tailLines int64 = 20
			rawLog, err := c.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apiv1.PodLogOptions{
				Container: cs.Name,
				TailLines: &tailLines,
			}).DoRaw()
			if err != nil {
				return fmt.Errorf("get log failed: %v", err)
			}
			glog.Errorf("Container %s in pod %s crashed", cs.Name, key)
			return c.handler.Handle(&handlers.Event{
				Namespace:     pod.Namespace,
				Name:          pod.Name,
				ContainerName: cs.Name,
				Reason:        cs.State.Waiting.Reason,
				Message:       cs.State.Waiting.Message,
				RawLog:        string(rawLog),
			})
		}
	}
	return nil
}
