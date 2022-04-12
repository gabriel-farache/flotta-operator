package watchers

import (
	"context"
	"os"
	"os/exec"
	"path"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"k8s.io/client-go/kubernetes"
)

const (
	namespace = "default"
	LOG_LEVEL = "LOG_LEVEL"
)

var _ = Describe("configmap_watcher", func() {

	var (
		clientset  *kubernetes.Clientset
		err        error
		logger     = zap.New(zap.UseFlagOptions(&zap.Options{}))
		crashIsSet bool
	)
	BeforeEach(func() {
		clientset, err = newClientset()
		Expect(err).To(BeNil())
		_, crashIsSet = os.LookupEnv("CRASH")

	})

	AfterEach(func() {
		_ = clientset.CoreV1().ConfigMaps(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})

	})
	Context("Sanity", func() {

		It("Test configmap update when LOG_LEVEL changes", func() {
			if !crashIsSet {
				currentDir, _ := os.Getwd()
				ctx := context.Background()
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "ginkgo", "run ", currentDir)
				cmd.Env = append(os.Environ(), "CRASH=1")
				log, err := cmd.Output()
				e, ok := err.(*exec.ExitError)

				Expect(string(log)).To(ContainSubstring("restarting pod to update the logging level"))
				Expect(ok).To(BeTrue())
				Expect(e.ExitCode()).To(BeIdenticalTo(1))

			} else {
				configMap := createDefaultConfigMap()
				_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &configMap, metav1.CreateOptions{})
				Expect(err).To(BeNil())
				//when
				go WatchForChanges(clientset, namespace, configMap.Name, LOG_LEVEL, configMap.Data[LOG_LEVEL], logger)

				configMap.Data[LOG_LEVEL] = "debug"
				_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), &configMap, metav1.UpdateOptions{})
				Expect(err).To(BeNil())
				//wait for the change event to be catch by the watcher
				time.Sleep(5 * time.Second)

				os.Exit(-1)
			}
		})

		It("Test configmap update when LOG_LEVEL does not change", func() {
			if !crashIsSet {
				configMap := createDefaultConfigMap()

				_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &configMap, metav1.CreateOptions{})
				Expect(err).To(BeNil())
				//when
				go WatchForChanges(clientset, namespace, configMap.Name, LOG_LEVEL, configMap.Data[LOG_LEVEL], logger)

				configMap.Data[LOG_LEVEL] = "info"
				_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), &configMap, metav1.UpdateOptions{})
				Expect(err).To(BeNil())

				return
			}
		})

		It("Test configmap update when other parameter changes", func() {
			if !crashIsSet {
				configMap := createDefaultConfigMap()
				_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &configMap, metav1.CreateOptions{})
				Expect(err).To(BeNil())
				//when
				go WatchForChanges(clientset, namespace, configMap.Name, LOG_LEVEL, configMap.Data[LOG_LEVEL], logger)

				configMap.Data["HTTP_PORT"] = "8080"
				_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), &configMap, metav1.UpdateOptions{})
				Expect(err).To(BeNil())

				return
			}
		})
	})

})

func createDefaultConfigMap() corev1.ConfigMap {
	configMapData := map[string]string{"EDGEDEPLOYMENT_CONCURRENCY": "5", "ENABLE_WEBHOOKS": "false", "HTTP_PORT": "8888", "WEBHOOK_PORT": "9443", LOG_LEVEL: "info"}
	configMapName := "configmap-test"
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
		Data: configMapData,
	}
}

func newClientset() (*kubernetes.Clientset, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.BuildConfigFromFlags("", path.Join(homedir, ".kube/config"))
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
