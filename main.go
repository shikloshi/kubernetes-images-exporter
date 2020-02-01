package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/parsers"
)

type ClusterData struct {
	namespace   string
	clusterName string
}

type ImageData struct {
	repo   string
	digest string
	tag    string
}

var imagesMap map[ImageData]int

var images *prometheus.GaugeVec

func main() {

	images = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "deployed_images",
		Help: "The total number of processed events",
	}, []string{
		"repo", "tag", "digest",
	})

	prometheus.MustRegister(images)

	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	imagesMap = make(map[ImageData]int, 0)

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().Pods().Informer()
	stopper := make(chan struct{})
	defer close(stopper)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		DeleteFunc: onDelete,
	})
	http.Handle("/metrics", prometheus.Handler())
	go http.ListenAndServe(":2112", nil)
	informer.Run(stopper)
}

func onDelete(obj interface{}) {
	log.Print("on delete")
	pod := obj.(*corev1.Pod)
	containers := pod.Spec.Containers
	for _, container := range containers {
		ig := newImageData(container.Image)
		images.WithLabelValues(ig.repo, ig.tag, ig.digest).Add(-1)
		imagesMap[*ig] -= 1
	}
}

func onAdd(obj interface{}) {
	log.Print("on add")
	pod := obj.(*corev1.Pod)
	containers := pod.Spec.Containers
	for _, container := range containers {
		ig := newImageData(container.Image)
		if _, exists := imagesMap[*ig]; !exists {
			imagesMap[*ig] = 1
		} else {
			imagesMap[*ig] += 1
		}
		images.WithLabelValues(ig.repo, ig.tag, ig.digest).Inc()
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
