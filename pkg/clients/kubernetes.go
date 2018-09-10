package clients

import (
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetClientset() (kubernetes.Interface, error) {
	log.Printf("getting clientset...")

	// create the k8s client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config. %+v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset. %+v", err)
	}

	return clientset, nil
}

func GetSecret(clientset kubernetes.Interface, namespace string, name string, key string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to fetch secret %s from namespace %s: %+v", name, namespace, err)
	}

	password := secret.Data[key]
	return string(password), nil
}
