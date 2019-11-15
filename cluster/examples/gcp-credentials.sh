#!/usr/bin/env bash
#
# This is a helper script to create a project, service account, and credentials.json
# file for use in Crossplane GCP examples
#
# gcloud is required for use and must be configured with privileges to perform these tasks
#
set -e -o pipefail
ROLES=(roles/iam.serviceAccountUser roles/cloudsql.admin roles/container.admin roles/redis.admin roles/compute.networkAdmin)
SERVICES=(container.googleapis.com sqladmin.googleapis.com redis.googleapis.com compute.googleapis.com servicenetworking.googleapis.com)
KEYFILE=crossplane-gcp-provider-key.json
RAND=$RANDOM

if ! command -v gcloud > /dev/null; then
	echo "Please install gcloud: https://cloud.google.com/sdk/install"
	exit 1
fi

tab () { sed 's/^/    /' ; }
# list your organizations (if applicable), take note of the specific organization ID you want to use
# if you have more than one organization (not common)
gcloud organizations list --format '[box]' 2>&1 | tab

ORGANIZATION_ID=$(gcloud organizations list --format 'value(ID)' --limit 1)
read -e -p "Choose an Organization ID [$ORGANIZATION_ID]: " PROMPT_ORGANIZATION_ID
ORGANIZATION_ID=${PROMPT_ORGANIZATION_ID:-$ORGANIZATION_ID}

gcloud projects list --format '[box]' 2>&1 | tab

# create a new id
EXAMPLE_PROJECT_ID="crossplane-example-$RAND"
read -e -p "Choose or create a Project ID [$EXAMPLE_PROJECT_ID]: " PROMPT_EXAMPLE_PROJECT_ID
EXAMPLE_PROJECT_ID=${PROMPT_EXAMPLE_PROJECT_ID:-$EXAMPLE_PROJECT_ID}

EXAMPLE_PROJECT_ID_FOUND=$(gcloud projects list --filter PROJECT_ID="$EXAMPLE_PROJECT_ID" --format="value(PROJECT_ID)")

if [[ -z $EXAMPLE_PROJECT_ID_FOUND ]]; then
	ACCOUNT_ID=$(gcloud beta billing accounts list --format 'value(ACCOUNT_ID)' --limit 1)
	gcloud beta billing accounts list --format '[box]' 2>&1 | tab
	read -e -p "Choose a Billing Account ID [$ACCOUNT_ID]: " PROMPT_ACCOUNT_ID
	ACCOUNT_ID=${PROMPT_ACCOUNT_ID:-$ACCOUNT_ID}

	echo -e "\n* Creating Project $EXAMPLE_PROJECT_ID ... "
	gcloud projects create $EXAMPLE_PROJECT_ID --enable-cloud-apis --organization $ORGANIZATION_ID 2>&1 | tab

	echo "* Linking Billing Account $ACCOUNT_ID with Project $EXAMPLE_PROJECT_ID ... "
	gcloud beta billing projects link $EXAMPLE_PROJECT_ID --billing-account=$ACCOUNT_ID 2>&1 | tab
else
	echo -n "\n* Using Project $EXAMPLE_PROJECT_NAME ... $EXAMPLE_PROJECT_ID"
fi

# enable Kubernetes API
for service in "${SERVICES[@]}"; do
	# enable Google API
	echo "* Enabling Service $service on $EXAMPLE_PROJECT_ID"
	gcloud --project $EXAMPLE_PROJECT_ID services enable $service 2>&1 | tab
done

# create service account
SA_NAME="example-$RAND"
echo " * Creating a Service Account"
gcloud --project $EXAMPLE_PROJECT_ID iam service-accounts create $SA_NAME --display-name "Crossplane Example" 2>&1 | tab
# export service account email
EXAMPLE_SA="${SA_NAME}@${EXAMPLE_PROJECT_ID}.iam.gserviceaccount.com"

# assign roles
for role in "${ROLES[@]}"; do
	echo "* Adding Role $role to $EXAMPLE_SA on $EXAMPLE_PROJECT_ID"
	gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="$role" 2>&1 | tab
done

# create service account key (this will create a `crossplane-gcp-provider-key.json` file in your current working directory)
echo " * Creating $EXAMPLE_SA Key File $KEYFILE"
gcloud --project $EXAMPLE_PROJECT_ID iam service-accounts keys create --iam-account $EXAMPLE_SA $KEYFILE 2>&1 | tab

cat <<EOS
#
# Run the following for the variables that are used throughout the GCP example projects
#
export ORGANIZATION_ID=$ORGANIZATION_ID
export PROJECT_ID=$EXAMPLE_PROJECT_ID
export EXAMPLE_SA=$EXAMPLE_SA
export BASE64ENCODED_GCP_PROVIDER_CREDS=\$(base64 $KEYFILE | tr -d "\n")
EOS
