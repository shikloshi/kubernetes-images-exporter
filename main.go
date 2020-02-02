package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/parsers"
)

type ImageData struct {
	repo   string
	digest string
	tag    string
}

var images *prometheus.GaugeVec

// ./kubernetes-image-exporter --port <port> --local --namespaces="..." --log
func main() {

	log.SetFormatter(&log.JSONFormatter{})

	images = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "deployed_images",
		Help: "The number of deployed images",
	}, []string{
		// image repo
		"repo",
		// image tag
		"tag",
		// image digest if exists
		"digest",
		// pod which runs this image
		"pod",
		// namespace of which the pod is running
		"namespace",
	})

	prometheus.MustRegister(images)

	local := true

	k8sConfig, err := createKubernetesConfig(local)
	if err != nil {
		log.Fatal(err.Error())
	}

	informer, err := newPodInformer(k8sConfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	stopper := make(chan struct{})
	defer close(stopper)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		DeleteFunc: onDelete,
	})

	promMetricsEndpoint := getEnvWithDefault("K8S_IMAGE_EXOPRTER_ENDPOINT", "/metrics")
	promPort := getEnvWithDefault("K8S_IMAGE_EXOPRTER_PORT", "9090")

	http.Handle(promMetricsEndpoint, prometheus.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%s", promPort), nil)
	informer.Run(stopper)
}

func createKubernetesConfig(local bool) (*rest.Config, error) {
	if local {
		kubeconfig := os.Getenv("KUBECONFIG")
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()

}

func newPodInformer(config *rest.Config) (cache.SharedIndexInformer, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	return factory.Core().V1().Pods().Informer(), nil
}

func onDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)
	containers := pod.Spec.Containers
	for _, container := range containers {
		ig := newImageData(container.Image)
		images.WithLabelValues(ig.repo, ig.tag, ig.digest, pod.Name, pod.Namespace).Add(-1)
		log.WithFields(log.Fields{
			"repo":      ig.repo,
			"tag":       ig.tag,
			"digest":    ig.digest,
			"pod":       pod.Name,
			"namespace": pod.Namespace,
		}).Info()
	}
}

func onAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	containers := pod.Spec.Containers
	for _, container := range containers {
		ig := newImageData(container.Image)
		images.WithLabelValues(ig.repo, ig.tag, ig.digest, pod.Name, pod.Namespace).Inc()
		log.WithFields(log.Fields{
			"pod":       pod.Name,
			"namespace": pod.Namespace,
			"repo":      ig.repo,
			"tag":       ig.tag,
			"digest":    ig.digest,
		}).Info()
	}

}

func newImageData(image string) *ImageData {
	repo, tag, digest, _ := parsers.ParseImageName(image)
	return &ImageData{
		repo:   repo,
		tag:    tag,
		digest: digest,
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}
