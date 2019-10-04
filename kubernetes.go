package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var namespace = "kad"

func getClientset() (*kubernetes.Clientset, error) {
	var (
		err    error
		config *rest.Config
	)

	if kp := os.Getenv("KUBECONFIG"); kp != "" {
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

	pc.KubernetesHost = config.Host

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// read kubernetes resources in current namespaces and save it to rootPage
func readResources() error {
	cs, err := getClientset()
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

	// list replicasets
	rl, err := cs.AppsV1().ReplicaSets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pc.Resources.ReplicaSets = rl.Items

	return nil
}

func kubernetesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	rt, ok := vars["type"]
	if !ok || rt == "" {
		http.Error(w, "Missing resource type", http.StatusBadRequest)
		return
	}

	name, ok := vars["name"]
	if !ok || name == "" {
		http.Error(w, "Missing resource name", http.StatusBadRequest)
		return
	}

	cs, err := getClientset()
	if err != nil {
		http.Error(w, "Can't connect to kubernetes", http.StatusBadRequest)
		return
	}

	switch rt {
	case "pod":
		if err := cs.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
			http.Error(w, "Failed deleting pod", http.StatusBadRequest)
			return
		}
	case "deploy":
		if err := cs.AppsV1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
			http.Error(w, "Failed deleting deployment", http.StatusBadRequest)
			return
		}

	case "rs":
		if err := cs.AppsV1().ReplicaSets(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
			http.Error(w, "Failed deleting service", http.StatusBadRequest)
			return
		}

	case "service":
		if err := cs.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
			http.Error(w, "Failed deleting pod", http.StatusBadRequest)
			return
		}

	default:
		http.Error(w, "Unknown resource", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "http://"+r.Host, http.StatusPermanentRedirect)
}
