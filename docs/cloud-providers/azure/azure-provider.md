## Microsoft Azure

Azure service principal credentials are needed for an admin account, which must be created before starting this Wordpress Workload example.

### Preparing your Microsoft Azure Account

In order to manage resources in Azure, you must provide credentials for a Azure service principal that Crossplane can use to authenticate.
This assumes that you have already [set up the Azure CLI client](https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli?view=azure-cli-latest) with your credentials.

Create a JSON file that contains all the information needed to connect and authenticate to Azure:

```console
# create service principal with Owner role
az ad sp create-for-rbac --sdk-auth --role Owner > crossplane-azure-provider-key.json
```

Save the `clientID` value from the JSON file we just created to an environment variable:

```console
export AZURE_CLIENT_ID=<clientId value from json file>
```

Now add the required permissions to the service principal we created that allow us to manage the necessary resources in Azure:

```console
# add required Azure Active Directory permissions
az ad app permission add --id ${AZURE_CLIENT_ID} --api 00000002-0000-0000-c000-000000000000 --api-permissions 1cda74f2-2616-4834-b122-5cb1b07f8a59=Role 78c8a3c8-a07e-4b9e-af1b-b5ccab50a175=Role

# grant (activate) the permissions
az ad app permission grant --id ${AZURE_CLIENT_ID} --api 00000002-0000-0000-c000-000000000000 --expires never
```

You might see an error similar to the following, but that is OK, the permissions should have gone through still:

```console
Operation failed with status: 'Conflict'. Details: 409 Client Error: Conflict for url: https://graph.windows.net/e7985bc4-a3b3-4f37-b9d2-fa256023b1ae/oauth2PermissionGrants?api-version=1.6
```

After these steps are completed, you should have the following file on your local filesystem:

* `crossplane-azure-provider-key.json`

## Grant Consent to application
1. `echo ${AZURE_CLIENT_ID}` and note id
1. Navigate to azure console: https://portal.azure.com
1. Click Azure Active Directory
1. Click `App registrations (Preview)`
1. Click on app item where client id matches step 1
1. Click `API permissions`
1. Click `Grant admin consent for Default Directory`
1. Click `Yes`
