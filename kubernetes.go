package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var namespace = "kad"

func getClientset(kp string) (*kubernetes.Clientset, error) {
	var (
		err    error
		config *rest.Config
	)

	if kp != "" {
		// we have kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kp)
		if err != nil {
			return nil, err
		}
	} else {
		// use incluster
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// read kubernetes resources in current namespaces and save it to rootPage
func readResources() error {
	cs, err := getClientset("/home/tom/.kube/config")
	if err != nil {
		return err
	}

	// list pods
	pl, err := cs.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pc.Resources.Pods = pl.Items

	// list services
	sl, err := cs.CoreV1().Services(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pc.Resources.Services = sl.Items

	// list deployments
	dl, err := cs.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pc.Resources.Deployments = dl.Items

	// list keplicasets
	rl, err := cs.ExtensionsV1beta1().ReplicaSets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pc.Resources.ReplicaSets = rl.Items

	return nil
}
