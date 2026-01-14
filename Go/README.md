# Azure Management Go SDK Sample for Azure Cosmos DB

This folder contains a Go sample that uses the **Azure Resource Manager (ARM) / Management Plane** SDKs to create and update **Azure Cosmos DB** resources.

This is useful when your application uses **Microsoft Entra ID** and you want to manage Cosmos DB resources (accounts, databases, containers, throughput, and RBAC) via an SDK instead of Bicep/PowerShell/Azure CLI.

## Important note about feature support

The Go `armcosmos` management SDK does **not** currently expose some newer Azure Cosmos DB SQL container fields (for example, **computed properties** and **vector settings** like `vectorEmbeddingPolicy` / `vectorIndexes`).

This sample intentionally uses only the fields currently supported by the Go management SDK.

Note: This sample's account creation includes the `EnableNoSQLVectorSearch` capability (forward-looking), but the container-level vector configuration is not set by the Go sample because it isn't exposed in the Go management SDK yet.

If you need computed properties or vector container configuration today, use one of the other language samples in this repository instead:

- [Csharp/](../Csharp/)
- [Java/](../Java/)
- [Python/](../Python/)

Alternatively, you can create/configure the container using Azure Portal, PowerShell, Azure CLI, or Bicep.

> **Important**: This sample uses the *control plane* (resource provider) APIs via ARM. It does **not** use the Cosmos DB *data plane* SDK to create ARM resources.

## Sample features

### Accounts (control plane)

- Create or update a Cosmos DB **SQL (NoSQL)** account.
- Disables local/key auth (`DisableLocalAuth=true`) so **Entra ID + RBAC** is required.
- Includes the `EnableNoSQLVectorSearch` account capability (note: container vector settings are not configured by this Go sample yet).
- Includes a commented-out **serverless** capability example.
- Adds an `owner` tag (best-effort) from the signed-in identity.

### Database and container (control plane)

- Create or update a SQL database.
- Create or update a SQL container with:
  - Hierarchical partition key (multi-hash) on `/companyId`, `/departmentId`, `/userId`.
  - Indexing policy (consistent).
  - Unique key on `/userId`.
  - TTL enabled with no default (container `DefaultTTL=-1`).
  - Last-writer-wins conflict resolution (`/_ts`).
  - Autoscale max throughput from configuration.

Notes:
- The Go `armcosmos` management SDK does not currently expose some newer container fields (for example, computed properties and vector settings like `vectorEmbeddingPolicy` / `vectorIndexes`).
- This sample intentionally creates the container using only the fields currently supported by the Go management SDK.
- If you need computed properties or vector container configuration today, use [Csharp/](../Csharp/), [Java/](../Java/), or [Python/](../Python/).

### Throughput

- Updates **container dedicated throughput** by reading current settings first and then:
  - Updating autoscale max throughput when the container is autoscale, or
  - Updating RU/s when the container is manual throughput.
- Re-reads and prints the applied settings after the update.
- Throws a clear error when the throughput resource doesn’t exist (common for **serverless** accounts or **shared database throughput**).

### Role-based access control (RBAC)

This sample creates **two role assignments by default** for the currently signed-in principal:

- **Azure RBAC (control plane)**: assigns the built-in `Cosmos DB Operator` role at the Cosmos account scope.
- **Cosmos DB SQL RBAC**: assigns the built-in `Cosmos DB Built-in Data Contributor` role.

It also includes a **custom Cosmos DB SQL RBAC role definition** example (not used by default).

### Interactive menu + safe delete

- Runs an interactive menu by default.
- Includes a "Run full sample" menu option.
- If stdin is not a TTY (for example, CI), the sample falls back to running the full sample.
- Supports deleting the Cosmos DB account:
  - From the menu (requires typing `DELETE` to confirm)
  - From the full run only when `COSMOS_SAMPLE_DELETE_ACCOUNT=true` (opt-in safety guard)

## Prerequisites

- An Azure subscription and a resource group.
- Go 1.22+ (this sample is validated with Go 1.24.x).
- Azure identity available to `DefaultAzureCredential`.
- Sign in with the Azure CLI before running the sample: `az login`
  - Other supported options include VS Code sign-in, Managed Identity, etc.
- Permissions:
  - To create/update Cosmos resources: typically **Contributor** on the resource group.
  - To create Azure RBAC role assignments: typically **Owner** or **User Access Administrator** at the target scope.

Notes:
- These operations require a subscription id, resource group, and an Azure region (`location`) for ARM resources.
  This `location` is typically the same region as your resource group, and it does not need to match the regions where Cosmos DB data is replicated.

### VS Code setup (recommended)

If you want to run/debug this sample from VS Code:

1. Install the Go extension: `golang.go`
2. Open [Go.code-workspace](../Go.code-workspace)

## Usage

1. **Open the Go workspace in VS Code**:

  Open [Go.code-workspace](../Go.code-workspace). This configures the recommended VS Code settings for this sample.

### Configuration

Copy and edit `config.json` and fill in these values:

```json
{
  "SubscriptionId": "...",
  "Location": "...",
  "ResourceGroupName": "...",
  "AccountName": "...",
  "DatabaseName": "...",
  "ContainerName": "...",
  "MaxAutoScaleThroughput": 1000
}
```

Notes:
- `MaxAutoScaleThroughput` is required and must be >= 1000.

## Setup

This sample expects you to run from the `Go/` folder.

```sh
cd Go
go mod tidy
```

Copy the sample config:

```sh
copy config.json.sample config.json
```

## Running

From the `Go/` folder:

```sh
go run .
```

Follow the on-screen menu prompts.

## Debugging in VS Code

Open the workspace file [Go.code-workspace](../Go.code-workspace) and press F5 to run **“Go: Debug sample”**.

In VS Code:

1. Use **File → Open Workspace from File…**
2. Select `Go.code-workspace`

This keeps each sample's debug configuration independent, so developers don't need to install debug extensions for languages they aren't using.

## Azure SDK for Go for Azure Cosmos DB

You can find the source code for the Azure Management SDK for Go for Azure Cosmos DB and additional samples at:
https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos