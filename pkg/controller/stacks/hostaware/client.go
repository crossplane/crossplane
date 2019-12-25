package hostaware

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// GetClients is the function to get Host configuration in case workload and resource API's are different
func GetClients() (client.Client, *kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		err = errors.Wrap(err, "failed to initialize host config with in cluster config")
		return nil, nil, err
	}
	hostKube, err := client.New(cfg, client.Options{})
	if err != nil {
		err = errors.Wrap(err, "failed to initialize host client with in cluster config")
		return nil, nil, err
	}
	hostClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		err = errors.Wrap(err, "failed to initialize host clientset with in cluster config")
		return nil, nil, err
	}

	return hostKube, hostClient, nil
}
