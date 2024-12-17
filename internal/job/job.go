package job

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Job interface {
	Run(ctx context.Context, itemsToKeep map[string]struct{}, keepTopNItems int) (int, error)
}

const removeUnusedCompositionRevisionJob = "removeUnusedCompositionRevision"

func NewJobs(log logging.Logger, k8sClient kubernetes.Interface, crossplaneClient client.Client) map[string]Job {
	return map[string]Job{
		removeUnusedCompositionRevisionJob: NewCompositionRevisionCleanupJob(log, k8sClient, crossplaneClient),
	}
}
