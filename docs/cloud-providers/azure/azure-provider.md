# Adding Microsoft Azure to Crossplane

In this guide, we will walk through the steps necessary to configure your Azure
account to be ready for integration with Crossplane. The general steps we will
take are summarized below:

* Create a new service principal (account) that Crossplane will use to create
  and manage Azure resources
* Add the required permissions to the account
* Consent to the permissions using an administrator account

## Preparing your Microsoft Azure Account

In order to manage resources in Azure, you must provide credentials for a Azure
service principal that Crossplane can use to authenticate. This assumes that you
have already [set up the Azure CLI
client](https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli?view=azure-cli-latest)
with your credentials.

Create a JSON file that contains all the information needed to connect and
authenticate to Azure:

```bash
# create service principal with Owner role
az ad sp create-for-rbac --sdk-auth --role Owner > crossplane-azure-provider-key.json
```

Take note of the `clientID` value from the JSON file that we just created, and
save it to an environment variable:

```bash
export AZURE_CLIENT_ID=<clientId value from json file>
```

Now add the required permissions to the service principal that will allow it to
manage the necessary resources in Azure:

```bash
# add required Azure Active Directory permissions
az ad app permission add --id ${AZURE_CLIENT_ID} --api 00000002-0000-0000-c000-000000000000 --api-permissions 1cda74f2-2616-4834-b122-5cb1b07f8a59=Role 78c8a3c8-a07e-4b9e-af1b-b5ccab50a175=Role

# grant (activate) the permissions
az ad app permission grant --id ${AZURE_CLIENT_ID} --api 00000002-0000-0000-c000-000000000000 --expires never
```

You might see an error similar to the following, but that is OK, the permissions
should have gone through still:

```console
Operation failed with status: 'Conflict'. Details: 409 Client Error: Conflict for url: https://graph.windows.net/e7985bc4-a3b3-4f37-b9d2-fa256023b1ae/oauth2PermissionGrants?api-version=1.6
```

Finally, you need to grant admin permissions on the Azure Active Directory to
the service principal because it will need to create other service principals
for your `AKSCluster`:

```bash
# grant admin consent to the service princinpal you created
az ad app permission admin-consent --id "${AZURE_CLIENT_ID}"
```

Note: You might need `Global Administrator` role to `Grant admin consent for
Default Directory`. Please contact the administrator of your Azure subscription.
To check your role, go to `Azure Active Directory` -> `Roles and
administrators`. You can find your role(s) by clicking on `Your Role (Preview)`

After these steps are completed, you should have the following file on your
local filesystem:

* `crossplane-azure-provider-key.json`

## Setup Azure ProviderConfig

Before creating any resources, we need to create and configure an Azure cloud
provider resource in Crossplane, which stores the cloud account information in
it. All the requests from Crossplane to Azure Cloud will use the credentials
attached to this provider resource. The following command assumes that you have
a `crossplane-azure-provider-key.json` file that belongs to the account you’d
like Crossplane to use.

```bash
BASE64ENCODED_AZURE_ACCOUNT_CREDS=$(base64 crossplane-azure-provider-key.json | tr -d "\n")
```

Now we’ll create our `Secret` that contains the credential and `ProviderConfig`
resource that refers to that secret:

```bash
cat > provider.yaml <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: azure-account-creds
  namespace: crossplane-system
type: Opaque
data:
  credentials: ${BASE64ENCODED_AZURE_ACCOUNT_CREDS}
---
apiVersion: azure.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: azure-account-creds
      key: credentials
EOF

# apply it to the cluster:
kubectl apply -f "provider.yaml"

# delete the credentials variable
unset BASE64ENCODED_AZURE_ACCOUNT_CREDS
```

The output will look like the following:

```bash
secret/azure-user-creds created
provider.azure.crossplane.io/default created
```

Crossplane resources use the `ProviderConfig` named `default` if no specific
`ProviderConfig` is specified, so this `ProviderConfig` will be the default for
all Azure resources.
