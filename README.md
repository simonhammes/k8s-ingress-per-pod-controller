# k8s-ingress-per-pod-controller

## Development

```bash
kind create cluster
kind export kubeconfig --kubeconfig kubeconfig.yaml
export KUBECONFIG="${PWD}/kubeconfig.yml"

kubectl apply -f manifests/

NAMESPACE=default LABEL_SELECTOR=app=nginx go run main.go
```
