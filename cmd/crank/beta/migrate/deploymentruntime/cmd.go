package deploymentruntime

import (
	"io"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

// Cmd arguments and flags for migrate deployment-runtime subcommand.
type Cmd struct {
	// Arguments.
	InputFile string `short:"i" type:"path" placeholder:"PATH" help:"The ControllerConfig file to be Converted."`

	OutputFile string `short:"o" type:"path" placeholder:"PATH" help:"The file to write the generated DeploymentRuntimeConfig to."`
}

func (c *Cmd) Help() string {
	return `
This command converts a Crossplane ControllerConfig to a DeploymentRuntimeConfig.

DeploymentRuntimeConfig was introduced in Crossplane 1.14 and ControllerConfig is
deprecated.

Examples:

  # Write out a DeploymentRuntimeConfigFile from a ControllerConfig 

  crossplane-migrator new-deployment-runtime -i examples/enable-flags.yaml -o my-drconfig.yaml

  # Create a new DeploymentRuntimeConfigFile via Stdout

  crossplane-migrator new-deployment-runtime -i cc.yaml | grep -v creationTimestamp | kubectl apply -f - 

`
}

func (c *Cmd) Run(logger logging.Logger) error {
	var data []byte
	var err error

	if c.InputFile != "" {
		data, err = os.ReadFile(c.InputFile)
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return errors.Wrap(err, "Unable to read input")
	}

	// Set up schemes for our API types
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = v1alpha1.AddToScheme(sch)
	_ = v1beta1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	cc := &v1alpha1.ControllerConfig{}
	_, _, err = decode(data, &v1alpha1.ControllerConfigGroupVersionKind, cc)
	if err != nil {
		return errors.Wrap(err, "Decode Error")
	}
	if cc.Spec.ServiceAccountName != nil && *cc.Spec.ServiceAccountName != "" {
		logger.Info("WARNING: serviceAccountName is set in the ControllerConfig.\nDeploymentRuntime does not create serviceAccounts, please create the service account separately.", "serviceAccountName", *cc.Spec.ServiceAccountName)
	}
	drc, err := ControllerConfigToDeploymentRuntimeConfig(cc)
	if err != nil {
		return errors.Wrap(err, "Cannot migrate to Deployment Runtime")
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var output io.Writer

	if c.OutputFile != "" {
		f, err := os.OpenFile(c.OutputFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return errors.Wrap(err, "Unable to open output file")
		}
		defer f.Close()
		output = f
	} else {
		output = os.Stdout
	}

	err = s.Encode(drc, output)
	if err != nil {
		return errors.Wrap(err, "Unable to encode output")
	}
	return nil
}
