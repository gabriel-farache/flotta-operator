package watchers

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	clientWatch "k8s.io/client-go/tools/watch"
)

var (
	logger  logr.Logger
	backoff = wait.Backoff{
		Steps:    10,
		Duration: 10 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
		Cap:      2 * time.Hour,
	}
)

func WatchForChanges(clientset kubernetes.Interface, namespace string, configMapName string, dataField string, dataValue string, setupLogger logr.Logger) {
	logger = setupLogger
	logger.V(1).Info("watch for changes", "namespace", namespace, "configMap name", configMapName, "data field", dataField, "current value", dataValue)
	retryWatcher, err := createWatcher(clientset, namespace, configMapName)

	if err != nil {
		logger.Error(err, "cannot create watcher", "namespace", namespace, "configMap name", configMapName)
		os.Exit(1)
	}
	for {
		checkConfigMapChanges(retryWatcher.ResultChan(), dataField, dataValue)
	}
}

func createWatcher(clientset kubernetes.Interface, namespace string, configMapName string) (*clientWatch.RetryWatcher, error) {
	watchConfigMapFunc := func(options metav1.ListOptions) (apiWatch.Interface, error) {
		return clientset.CoreV1().ConfigMaps(namespace).Watch(context.TODO(),
			metav1.SingleObject(metav1.ObjectMeta{Name: configMapName, Namespace: namespace}))
	}
	logger.Info("Starting watcher", "namespace", namespace, "configMap name", configMapName)

	return clientWatch.NewRetryWatcher("1", &cache.ListWatch{WatchFunc: watchConfigMapFunc})
}

func checkConfigMapChanges(eventChannel <-chan apiWatch.Event, dataField string, dataValue string) {
	for {
		event, open := <-eventChannel
		if open {
			switch event.Type {
			case apiWatch.Added:
				fallthrough
			case apiWatch.Modified:
				if updatedMap, ok := event.Object.(*corev1.ConfigMap); ok {
					if updatedValue, ok := updatedMap.Data[dataField]; ok {
						if updatedValue != dataValue {
							logger.Info("restarting pod to update the logging level", "current level", dataValue, "new level", updatedValue)
							os.Exit(1)
						}
					}
				}
			case apiWatch.Deleted:
				fallthrough
			default:
				// Do nothing
			}
		} else {
			// If eventChannel is closed, it means the server has closed the connection
			return
		}
	}
}

func retriable(err error) bool {
	retry := err != nil
	logger.V(1).Info("cannot create watcher", "retriable", retry, "error", err)
	return retry
}
