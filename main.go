package main

import (
	"context"
	"os"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)

	// TODO: Test in-cluster config
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		klog.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		klog.Fatal("NAMESPACE must be provided")
	}

	labelSelector := os.Getenv("LABEL_SELECTOR")
	if labelSelector == "" {
		klog.Fatal("LABEL_SELECTOR must be provided")
	}

	watchStatefulSets(client, namespace, labelSelector)
}

func watchStatefulSets(client *kubernetes.Clientset, namespace string, labelSelector string) {
	options := meta.ListOptions{
		LabelSelector: labelSelector,
	}

	services := client.CoreV1().Services(namespace)
	watcher, err := client.CoreV1().Pods(namespace).Watch(context.Background(), options)
	if err != nil {
		klog.Fatalf("Could not watch pods: %v", err)
	}

	for event := range watcher.ResultChan() {
		pod := event.Object.(*core.Pod)

		switch event.Type {
		case watch.Added:
			klog.InfoS("Pod has been added", "pod", pod.Name)
			createService(services, pod)
		case watch.Modified:
		case watch.Bookmark:
		case watch.Error:
		case watch.Deleted:
			klog.InfoS("Pod has been deleted", "pod", pod.Name)
			deleteService(services, pod)
		}
	}
}

func createService(services v1.ServiceInterface, pod *core.Pod) {
	var port int32
	ports := pod.Spec.Containers[0].Ports

	switch len(ports) {
	case 0:
		klog.Errorf("Pod %s does not specify any ports", pod.Name)
		return
	case 1:
		port = ports[0].ContainerPort
	default:
		klog.Errorf("Pod %s specifies more than one port", pod.Name)
		return
	}

	service := &core.Service{
		ObjectMeta: meta.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Spec: core.ServiceSpec{
			Type: core.ServiceTypeClusterIP,
			Selector: map[string]string{
				"statefulset.kubernetes.io/pod-name": pod.Name,
			},
			Ports: []core.ServicePort{
				{
					Protocol:   core.ProtocolTCP,
					Port:       port,
					TargetPort: intstr.FromInt32(port),
				},
			},
		},
	}

	createdService, err := services.Create(context.TODO(), service, meta.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			klog.InfoS("Service already exists", "service", service.Name)
			return
		}

		klog.Errorf("Could not create service: %v", err)
		return
	}

	klog.InfoS("Created service", "service", createdService.GetName())
}

func deleteService(services v1.ServiceInterface, pod *core.Pod) {
	err := services.Delete(context.TODO(), pod.Name, meta.DeleteOptions{})

	if err != nil {
		klog.Errorf("Could not delete pod: %v", err)
		return
	}

	klog.InfoS("Deleted service", "service", pod.Name)
}
