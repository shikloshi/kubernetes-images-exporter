# kubernetes-images-exporter

This tool allows exporting current images running inside our kubernetes cluster and be collected by `Prometheus`.

This tool also logs when Pod is created or deleted and can be combined using shippers (like `fluentbit`) to send events into `Elasitcsearch` cluster or something similar. 

## Deployment

`deploy/` folder containes basic helm chart you can use to install on your cluster.
```bash
helm3 install kubernetes-images-exporter ./deploy
```

## RBAC

`kubernetes-images-exporter` should be allowed to `read,list,wathc` all `pod` resources from core API group.

You can bind `kubernetes-images-exporter` ServicAccount to `view` ClusterRole.

## Running 

### Example
```
deployed_images{digest="",namespace="default",pod="prometheus-operator-prometheus-node-exporter-mfvvw",repo="quay.io/prometheus/node-exporter",tag="v0.18.1"} 1
deployed_images{digest="",namespace="default",pod="prometheus-prometheus-operator-prometheus-0",repo="quay.io/coreos/configmap-reload",tag="v0.0.1"} 1
deployed_images{digest="",namespace="default",pod="prometheus-prometheus-operator-prometheus-0",repo="quay.io/coreos/prometheus-config-reloader",tag="v0.35.0"} 1
deployed_images{digest="",namespace="default",pod="prometheus-prometheus-operator-prometheus-0",repo="quay.io/prometheus/prometheus",tag="v2.15.2"} 1
```
## Configuration

TBD