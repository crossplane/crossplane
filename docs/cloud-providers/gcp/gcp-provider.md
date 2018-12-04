## Google Cloud Platform (GCP)

Create a GCP example project which we will use to host our example GKE cluster, as well as our example CloudSQL instance.

- Login into [GCP Console](https://console.cloud.google.com)
- Create a new project (either stand alone or under existing organization)
- Create Example Service Account
    - Navigate to: [Create Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts)
    - `Service Account Name`: type "example"
    - `Service Account ID`: leave auto assigned
    - `Service Account Description`: type "Crossplane example"
    - Hit `Create` button
        - This should advance to the next section `2 Grant this service account to project (optional)`
    - We will assign this account 3 roles:
        - `Service Account User`
        - `Cloud SQL Admin`
        - `Kubernetes Engine Admin`  
    - Hit `Create` button
        - This should advance to the next section `3 Grant users access to this service account (optinoal)`
    - We don't need to assign any user or admin roles to this account for the example purposes, so you can leave following two fields blank:
        - `Service account users role`
        - `Service account admins role`
    - Next, we will create and export service account key
        - Hit `+ Create Key` button. 
            - This should open a `Create Key` side panel
        - Select `json` for the Key type (should be selected by default) 
        - Hit `Create`
            - This should show `Private key saved to your computer` confirmation dialog
            - You also should see `crossplane-example-1234-[suffix].json` file in your browser's Download directory
            - Save (copy or move) this file into example (this) directory, with new name `key.json`
- Enable `Cloud SQL API`
    - Navigate to [Cloud SQL Admin API](https://console.developers.google.com/apis/api/sqladmin.googleapis.com/overview)
    - Hit `Enable`
- Enable `Kubernetes Engine API`
    - Navigate to [Kubernetes Engine API](https://console.developers.google.com/apis/api/container.googleapis.com/overview)
    - Hit `Enable`

If you have `gcloud` utility, you can ran following commands from the example directory 

```bash
# list your organizations (if applicable)
gcloud organizations list
    
# create a new project
export EXAMPLE_PROJECT_NAME=crossplane-example-123
gcloud projects create $EXAMPLE_PROJECT_NAME --enable-cloud-apis [--organization ORANIZATION_ID]

# record the PROJECT_ID value of the newly created project
export EXAMPLE_PROJECT_ID=$(gcloud projects list --filter NAME=$EXAMPLE_PROJECT_NAME --format="value(PROJECT_ID)")   

# enable Kubernetes API
gcloud --project $EXAMPLE_PROJECT_ID services enable container.googleapis.com
# enable CloudSQL API
gcloud --project $EXAMPLE_PROJECT_ID services enable sqladmin.googleapis.com 

# create service account
gcloud --project $EXAMPLE_PROJECT_ID iam service-accounts create example-123 --display-name "Crossplane Example"
# export service account email
export EXAMPLE_SA="example-123@$EXAMPLE_PROJECT_ID.iam.gserviceaccount.com"

# create service account key (this will create a `key.json` file in your current working directory)
gcloud --project $EXAMPLE_PROJECT_ID iam service-accounts keys create --iam-account $EXAMPLE_SA key.json 

# assign roles
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/iam.serviceAccountUser"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/cloudsql.admin"
gcloud projects add-iam-policy-binding $EXAMPLE_PROJECT_ID --member "serviceAccount:$EXAMPLE_SA" --role="roles/container.admin"
```

### Enable Billing
In order to create GKE clusters you must enable Billing.
- Go to [GCP Console](https://console.cloud.google.com)
    - Select example project
    - Hit "Enable Billing"
- Go to [Kubernetes Clusters](https://console.cloud.google.com/kubernetes/list)
    - Hit "Enable Billing"