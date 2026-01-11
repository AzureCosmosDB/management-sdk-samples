# Azure Management C# SDK Sample for Azure Cosmos DB

This folder contains a C# (.NET) sample that uses the **Azure Resource Manager (ARM) / Management Plane** SDKs to create and update **Azure Cosmos DB** resources.

This is useful when your application uses **Microsoft Entra ID** and you want to manage Cosmos DB resources (accounts, databases, containers, throughput, and RBAC) via an SDK instead of Bicep/PowerShell/Azure CLI.

> **Important**: This sample uses the *control plane* (resource provider) APIs via ARM. It does **not** use the Cosmos DB *data plane* SDK (`Microsoft.Azure.Cosmos`) to create ARM resources.

## Sample features

### Accounts (control plane)
- Create or update a Cosmos DB **SQL (NoSQL)** account.
- Sets `DisableLocalAuth = true` (disables key-based auth; Entra ID + RBAC required).
- Enables the `EnableNoSQLVectorSearch` capability.
- Leaves a **serverless** capability example commented out.

### Database and container (control plane)
- Create or update a SQL database.
- Create or update a SQL container with:
  - Hierarchical partition key (multi-hash) on `/companyId`, `/departmentId`, `/userId`.
  - Indexing policy (consistent), plus a vector index on `/vectors`.
  - Vector embedding definition (1536 dims, cosine distance).
  - Unique key on `/userId`.
  - Computed property example.
  - TTL enabled with no default (container `DefaultTtl = -1`).
  - Last-writer-wins conflict resolution (`/_ts`).
  - Autoscale max throughput from configuration.

### Throughput
- Updates **container dedicated throughput** by reading current settings first and then:
  - Updating autoscale max throughput when the container is autoscale, or
  - Updating RU/s when the container is manual throughput.
- Re-reads and logs the applied settings after the update.
- Throws a clear error when the throughput resource doesn’t exist (common for **serverless** accounts or **shared database throughput**).

### Role-based access control (RBAC)
This sample creates **two role assignments by default** for the currently signed-in principal:

- **Azure RBAC (control plane)**: assigns the built-in `Cosmos DB Operator` role to the Cosmos account scope.
- **Cosmos DB SQL RBAC (data plane)**: assigns the built-in `Cosmos DB Built-in Data Contributor` role.

It also includes a **custom Cosmos DB SQL RBAC role definition** example (not used by default).

### Interactive menu + safe delete
- Runs an interactive menu by default.
- Includes a "Run full sample" menu option.
- Supports deleting the Cosmos DB account:
  - From the menu (requires typing `DELETE` to confirm)
  - From the full run only when `COSMOS_SAMPLE_DELETE_ACCOUNT=true` (opt-in safety guard)

## Prerequisites

- An Azure subscription and a resource group.
- .NET SDK 9 installed.
- Azure identity available to `DefaultAzureCredential` (e.g., `az login`, VS Code sign-in, Managed Identity, etc.).
- Permissions:
  - To create/update Cosmos resources: typically **Contributor** on the resource group.
  - To create Azure RBAC role assignments: typically **Owner** or **User Access Administrator** at the target scope.

## Configuration

Configuration is bound to `AppSettings` from `appsettings.json` / environment-specific settings.

Keys:
- `SubscriptionId`
- `ResourceGroupName`
- `AccountName`
- `Location`
- `DatabaseName`
- `ContainerName`
- `MaxAutoScaleThroughput`

Notes:
- `Host.CreateDefaultBuilder()` loads `appsettings.json` and `appsettings.{Environment}.json`.
- Set `DOTNET_ENVIRONMENT=Development` to use `appsettings.development.json`.

## Setup

From the repo root:

```sh
cd Csharp
dotnet restore CosmosManagement.csproj
```

Update configuration in `appsettings.json` (or run with `DOTNET_ENVIRONMENT=Development` and update `appsettings.development.json`).

## Running

### Interactive menu

```sh
cd Csharp
dotnet run
```

### Full end-to-end run (via the menu)

Run `dotnet run`, then choose **"Run full sample"** from the menu.

### Full run + delete the account at the end (opt-in)

Run `dotnet run`, then choose **"Run full sample"** from the menu.

Windows (PowerShell):
```powershell
$env:COSMOS_SAMPLE_DELETE_ACCOUNT = "true"
dotnet run
```

macOS/Linux:
```sh
export COSMOS_SAMPLE_DELETE_ACCOUNT=true
dotnet run
```

## Debugging in VS Code

This repo includes a VS Code launch configuration named **“C#: Debug sample”** that:
- Builds the C# project first
- Sets `DOTNET_ENVIRONMENT=Development`
- Launches `Csharp/bin/Debug/net9.0/Csharp.dll`

Use the **Run and Debug** panel to start it.

## Troubleshooting

- **Throughput update fails with “settings not found”**: the container likely uses **shared database throughput** or the account is **serverless**. Create the container with dedicated throughput (or update database throughput instead).
- **RBAC assignment fails**: your identity likely lacks `Microsoft.Authorization/roleAssignments/write` at the chosen scope.
- **Options validation fails on startup**: one or more required config values are missing/empty.
