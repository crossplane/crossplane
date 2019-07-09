# Adding Microsoft Azure to Crossplane

In this guide, we will walk through the steps necessary to configure your Azure account to be ready for integration with Crossplane.
The general steps we will take are summarized below:

* Create a new service principal (account) that Crossplane will use to create and manage Azure resources
* Add the required permissions to the account
* Consent to the permissions using an administrator account

## Preparing your Microsoft Azure Account

In order to manage resources in Azure, you must provide credentials for a Azure service principal that Crossplane can use to authenticate.

This assumes that you have already [set up the Azure CLI client](https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli?view=azure-cli-latest) with your credentials.

Create a JSON file that contains all the information needed to connect and authenticate to Azure:

```console
# create service principal with Owner role
az ad sp create-for-rbac --sdk-auth --role Owner > crossplane-azure-provider-key.json
```

Take note of the `clientID` value from the JSON file that we just created, and save it to an environment variable:

```console
export AZURE_CLIENT_ID=<clientId value from json file>
```

This can be automated with `jq`.

```console
export AZURE_CLIENT_ID=$(jq -r .clientId < crossplane-azure-provider-key.json)
```

Now add the required permissions to the service principal that will allow it to manage the necessary resources with the Azure Active Directory Graph:

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

## Grant Consent to Application Permissions

One more step is required to fully grant the permissions to the new service principal.
From the Azure Portal, you need to grant consent for the permissions using an admin account.
The steps to perform this action are listed below:

1. `echo ${AZURE_CLIENT_ID}` and note this ID value
1. Navigate to the Azure Portal: https://portal.azure.com
1. Click `Azure Active Directory`, or find it in the `All services` list
1. Click `App registrations (Preview)`
1. Click on the application from the list where the application (client) ID matches the value from step 1
1. Click `API permissions`
1. Click `Grant admin consent for Default Directory`
1. Click `Yes`
