package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/parsers"
)

var images *prometheus.GaugeVec

func main() {
	var port string
	var endpoint string

	log.SetFormatter(&log.JSONFormatter{})

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:        "port",
			Value:       ":9090",
			Usage:       "port for metrics http endpoint",
			Destination: &port,
		},
		&cli.StringFlag{
			Name:        "endpoint",
			Value:       "/metrics",
			Usage:       "endpoint for metrics scraping",
			Destination: &endpoint,
		},
	}

	app := cli.App{
		Name: "kubernetes-images-exporter",
		Action: func(context *cli.Context) error {
			return app(context)
		},
		Flags: flags,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err.Error())
	}
}

// ./kubernetes-image-exporter --port <port> --local --namespaces="..." --log
func app(context *cli.Context) error {
	promPort := context.String("port")
	promMetricsEndpoint := context.String("endpoint")

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
		return err
	}

	informer, err := newPodInformer(k8sConfig)
	if err != nil {
		return err
	}

	stopper := make(chan struct{})
	defer close(stopper)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		DeleteFunc: onDelete,
	})

	http.Handle(promMetricsEndpoint, prometheus.Handler())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	go http.ListenAndServe(fmt.Sprintf(":%s", promPort), nil)
	informer.Run(stopper)
	return nil
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
		repo, tag, digest, _ := parsers.ParseImageName(container.Image)
		images.WithLabelValues(repo, tag, digest, pod.Name, pod.Namespace).Add(-1)
		log.WithFields(log.Fields{
			"repo":      repo,
			"tag":       tag,
			"digest":    digest,
			"pod":       pod.Name,
			"namespace": pod.Namespace,
		}).Info()
	}
}

func onAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	containers := pod.Spec.Containers
	for _, container := range containers {
		repo, tag, digest, _ := parsers.ParseImageName(container.Image)
		images.WithLabelValues(repo, tag, digest, pod.Name, pod.Namespace).Inc()
		log.WithFields(log.Fields{
			"pod":       pod.Name,
			"namespace": pod.Namespace,
			"repo":      repo,
			"tag":       tag,
			"digest":    digest,
		}).Info()
	}
}
