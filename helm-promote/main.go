package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"  // Corrected import
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	http.HandleFunc("/check-release", checkReleaseHandler)
	http.ListenAndServe(":8081", nil)
}

func checkReleaseHandler(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	if namespace == "" || name == "" {
		http.Error(w, "Namespace and name are required", http.StatusBadRequest)
		return
	}

	var kubeconfig string
	if filename, found := os.LookupEnv("KUBECONFIG"); found {
		kubeconfig = filename
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	sch := runtime.NewScheme()  // Create a new scheme
	fluxhelmv2beta1.AddToScheme(sch)  // Add HelmRelease to the scheme

	k8sClient, err := client.New(config, client.Options{Scheme: sch})  // Pass the scheme to the client
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	helmRelease := &fluxhelmv2beta1.HelmRelease{}
	err = k8sClient.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, helmRelease)
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("HelmRelease not found: %v", err), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	conditions := helmRelease.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == "Ready" {  // Corrected line
			response, err := json.Marshal(map[string]string{"status": string(condition.Status)})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(response)
			return
		}
	}

	http.Error(w, "Ready condition not found", http.StatusNotFound)
}
