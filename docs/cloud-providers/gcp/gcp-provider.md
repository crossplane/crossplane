# Adding Google Cloud Platform (GCP) to Crossplane

In this guide, we will walk through the steps necessary to configure your GCP
account to be ready for integration with Crossplane. The general steps we will
take are summarized below:

* Create a new example project that all resources will be deployed to
* Enable required APIs such as Kubernetes and CloudSQL
* Create a service account that will be used to perform GCP operations from
  Crossplane
* Assign necessary roles to the service account
* Enable billing

For your convenience, the specific steps to accomplish those tasks are provided
for you below using either the `gcloud` command line tool, or the GCP console in
a web browser. You can choose whichever you are more comfortable with.

## Option 1: gcloud Command Line Tool

If you have the `gcloud` tool installed, you can run the commands below from the
crossplane directory.

Instructions for installing `gcloud` can be found in the [Google
docs](https://cloud.google.com/sdk/install).

### Using `gcp-credentials.sh`

Crossplane provides a helper script for configuring GCP credentials. This script
will prompt you for the organization, project, and billing account that will be
used by `gcloud` when creating a project, service account, and credentials file
(`crossplane-gcp-provider-key.json`).  The chosen project and created service
account will have access to the services and roles sufficient to run the
Crossplane GCP examples.

```bash
curl -O https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/gcp/credentials.sh
./credentials.sh
# ... EXAMPLE OUTPUT ONLY
# export ORGANIZATION_ID=987654321
# export PROJECT_ID=crossplane-example-1234
# export EXAMPLE_SA=example-1234@crossplane-example-1234.iam.gserviceaccount.com
# export BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
```

After running `gcp-credentials.sh`, a series of `export` commands will be shown.
Copy and paste the `export` commands that are provided.  These variable names
will be referenced throughout the Crossplane examples, generally with a `sed`
command.

You will also find a `crossplane-gcp-provider-key.json` file in the current
working directory.  Be sure to remove this file when you are done with the
example projects.

### Running `gcloud` by hand

```bash
# list your organizations (if applicable), take note of the specific organization ID you want to use
# if you have more than one organization (not common)
gcloud organizations list

# create a new project (project id must be <=30 characters)
export EXAMPLE_PROJECT_ID=crossplane-example-123
gcloud projects create $EXAMPLE_PROJECT_ID --enable-cloud-apis # [--organization $ORGANIZATION_ID]

# or, record the PROJECT_ID value of an existing project
# export EXAMPLE_PROJECT_ID=$(gcloud projects list --filter NAME=$EXAMPLE_PROJECT_NAME --format="value(PROJECT_ID)")

# link billing to the new project
gcloud beta billing accounts list
gcloud beta billing projects link $EXAMPLE_PROJECT_ID --billing-account=$ACCOUNT_ID

# enable Kubernetes API
gcloud --project $EXAMPLE_PROJECT_ID services enable container.googleapis.com

# enable CloudSQL API
gcloud --project $EXAMPLE_PROJECT_ID services enable sqladmin.googleapis.com

# enable Redis API
gcloud --project $EXAMPLE_PROJECT_ID services enable redis.googleapis.com

# enable Compute API
gcloud --project $EXAMPLE_PROJECT_ID services enable compute.googleapis.com

# enable Service Networking API
gcloud --project $EXAMPLE_PROJECT_ID services enable servicenetworking.googleapis.com

# enable Additional APIs needed for the example or project
# See `gcloud services list` for a complete list

# create service account
gcloud --project $EXAMPLE_PROJECT_ID iam service-accounts create example-123 --display-name "Crossplane Example"

# export service account email
export EXAMPLE_SA="example-123@$EXAMPLE_PROJECT_ID.iam.gserviceaccount.com"

# create service account key (this will create a `crossplane-gcp-provider-key.json` file in your current working directory)
gcloud --project $EXAMPLE_PROJECT_ID iam service-accounts keys create --iam-account $EXAMPLE_SA crossplane-gcp-provider-key.json

# assign roles
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/iam.serviceAccountUser"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/cloudsql.admin"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/container.admin"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/redis.admin"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/compute.networkAdmin"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/storage.admin"
```

## Option 2: GCP Console in a Web Browser

If you chose to use the `gcloud` tool, you can skip this section entirely.

Create a GCP example project which we will use to host our example GKE cluster,
as well as our example CloudSQL instance.

- Login into [GCP Console](https://console.cloud.google.com)
- Create a [new
  project](https://console.cloud.google.com/flows/enableapi?apiid=container.googleapis.com,sqladmin.googleapis.com,redis.googleapis.com)
  (either stand alone or under existing organization)
- Create Example Service Account
  - Navigate to: [Create Service
    Account](https://console.cloud.google.com/iam-admin/serviceaccounts)
  - `Service Account Name`: type "example"
  - `Service Account ID`: leave auto assigned
  - `Service Account Description`: type "Crossplane example"
  - Click `Create and Continue` button
    - This should advance to the next section `2 Grant this service account to
      project (optional)`
  - We will assign this account 4 roles:
    - `Service Account User`
    - `Cloud SQL Admin`
    - `Kubernetes Engine Admin`
    - `Compute Network Admin`
  - Click `Continue` button
    - This should advance to the next section `3 Grant users access to this
      service account (optional)`
  - We don't need to assign any user or admin roles to this account for the
    example purposes, so you can leave following two fields blank:
    - `Service account users role`
    - `Service account admins role`
  - Next, we will create and export service account key
    - Click `+ Create Key` button.
      - This should open a `Create Key` side panel
    - Select `json` for the Key type (should be selected by default)
    - Click `Create`
      - This should show `Private key saved to your computer` confirmation
        dialog
      - You also should see `crossplane-example-1234-[suffix].json` file in your
        browser's Download directory
      - Save (copy or move) this file into example (this) directory, with new
        name `crossplane-gcp-provider-key.json`
- Enable `Cloud SQL API`
  - Navigate to [Cloud SQL Admin
    API](https://console.developers.google.com/apis/api/sqladmin.googleapis.com/overview)
  - Click `Enable`
- Enable `Kubernetes Engine API`
  - Navigate to [Kubernetes Engine
    API](https://console.developers.google.com/apis/api/container.googleapis.com/overview)
  - Click `Enable`
- Enable `Cloud Memorystore for Redis`
  - Navigate to [Cloud Memorystore for
    Redis](https://console.developers.google.com/apis/api/redis.googleapis.com/overview)
  - Click `Enable`
- Enable `Compute Engine API`
  - Navigate to [Compute Engine
    API](https://console.developers.google.com/apis/api/compute.googleapis.com/overview)
  - Click `Enable`
- Enable `Service Networking API`
  - Navigate to [Service Networking
    API](https://console.developers.google.com/apis/api/servicenetworking.googleapis.com/overview)
  - Click `Enable`

### Enable Billing

You will need to enable billing for your account in order to create and use
Kubernetes clusters with GKE.

- Go to [GCP Console](https://console.cloud.google.com)
  - Select example project
  - Click `Enable Billing`
- Go to [Kubernetes Clusters](https://console.cloud.google.com/kubernetes/list)
  - Click `Enable Billing`

## Setup GCP ProviderConfig

Before creating any resources, we need to create and configure a GCP cloud
`ProviderConfig` resource in Crossplane, which stores the cloud account
information in it. All the requests from Crossplane to GCP will use the
credentials attached to this `ProviderConfig` resource. The following command
assumes that you have a `crossplane-gcp-provider-key.json` file that belongs to
the account that will be used by Crossplane, which has GCP project id. You
should be able to get the project id from the JSON credentials file or from the
GCP console. Without loss of generality, let's assume the project id is
`my-cool-gcp-project` in this guide.

First, let's encode the credential file contents and put it in a variable:

```bash
# base64 encode the GCP credentials
BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
```

Next, store the project ID of the GCP project in which you would like to
provision infrastructure as a variable:

```bash
# replace this with your own gcp project id
PROJECT_ID=my-cool-gcp-project
```

Finally, store the namespace in which you want to save the provider's secret as
a variable:

```bash
# change this namespace value if you want to use a different namespace (e.g. gitlab-managed-apps)
PROVIDER_SECRET_NAMESPACE=crossplane-system
```

Now we’ll create the `Secret` resource that contains the credential, and
 `ProviderConfig` resource which refers to that secret:

```bash
cat > provider.yaml <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: gcp-account-creds
  namespace: ${PROVIDER_SECRET_NAMESPACE}
type: Opaque
data:
  credentials: ${BASE64ENCODED_GCP_PROVIDER_CREDS}
---
apiVersion: gcp.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  # replace this with your own gcp project id
  projectID: ${PROJECT_ID}
  credentials:
    source: Secret
    secretRef:
      namespace: ${PROVIDER_SECRET_NAMESPACE}
      name: gcp-account-creds
      key: credentials
EOF

# apply it to the cluster:
kubectl apply -f "provider.yaml"

# delete the credentials
unset BASE64ENCODED_GCP_PROVIDER_CREDS
```

The output will look like the following:

```bash
secret/gcp-account-creds created
provider.gcp.crossplane.io/default created
```

Crossplane resources use the `ProviderConfig` named `default` if no specific
`ProviderConfig` is specified, so this `ProviderConfig` will be the default for
all GCP resources.
