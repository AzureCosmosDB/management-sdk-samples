# Azure Management Go SDK Samples for Azure Cosmos DB

Repository of Azure Management Go SDK samples for creating and updating Azure Cosmos DB resources.

## Azure Go SDK for Azure Cosmos DB

You can find the source code and for this and other Azure SDKs at:
[management-sdk-samples](https://github.com/AzureCosmosDB/management-sdk-samples)


## Prerequisites
 - Go 1.22+ (this sample is validated with Go 1.24.x)
 - VS Code + Go extension: `golang.go`

## Setup

If you already have Go installed, the only thing you should need is to restore dependencies.

From the repo root:

```bash
cd Go
go version
go mod tidy
```

Notes:
- `go mod tidy` is safe to re-run; it ensures your local module cache has everything needed.

### Configuration

This sample reads configuration from `Go/config.json`.

You can override any value using environment variables (useful for CI).

1. Copy the sample config:

```bash
cd Go
copy config.json.sample config.json
```

2. Edit `config.json` and fill in your values:

```json
{
  "SubscriptionId": "{SubscriptionId}",
  "ResourceGroupName": "{ResourceGroupName}",
  "AccountName": "{AccountName}",
  "Location": "West US 3",
  "DatabaseName": "{DatabaseName}",
  "ContainerName": "{ContainerName}",
  "MaxAutoScaleThroughput": 1000
}
```

Environment variable equivalents:
- `AZURE_SUBSCRIPTION_ID`
- `AZURE_RESOURCE_GROUP`
- `AZURE_LOCATION`
- `COSMOS_ACCOUNT_NAME`
- `COSMOS_DATABASE_NAME`
- `COSMOS_CONTAINER_NAME`
- `COSMOS_MAX_AUTOSCALE_THROUGHPUT` (optional, default 1000)

### Running the Code

Run the Go program:

``` bash
cd Go
go run .
```

## Debugging in VS Code

1. Open the workspace file [Go.code-workspace](../Go.code-workspace)
2. Make sure the VS Code Go extension is installed: `golang.go`
3. Press **F5** and select the launch config **“Go: Debug sample”**

If VS Code prompts to install the Go debugger (Delve), allow it. If debugging fails, run the VS Code command:
**Go: Install/Update Tools**.

### Project Structure

 - main.go: The main application file containing the logic to manage Cosmos DB resources.
 - config.json: Configuration file for the project.
 - config.json.sample: Template config (copy to config.json).
 - to/to.go: Helper functions for converting values to pointers.


## Notes
 - Ensure you have the necessary permissions in your Azure subscription to create and manage Cosmos DB resources.
 - If the Azure RBAC role assignment step fails, your identity likely lacks `Microsoft.Authorization/roleAssignments/write` at the Cosmos account scope (you typically need Owner or User Access Administrator).
 - The Go `armcosmos` management SDK currently doesn't expose some newer container fields (e.g., computed properties and vector settings). To keep parity with other samples, this Go sample applies those settings via a best-effort ARM PATCH using `Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers@2024-12-01-preview`.
   - To skip that step, set `applyAdvancedContainerSettingsPatch = false` in [Go/main.go](main.go).
 - For Go-related issues, refer to the official [Go documentation](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos#section-documentation).