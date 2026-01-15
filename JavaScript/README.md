# Azure Management JavaScript SDK Sample for Azure Cosmos DB

This folder contains a JavaScript (Node.js) sample that uses the **Azure Resource Manager (ARM) / Management Plane** SDKs to create and update **Azure Cosmos DB** resources.

This is useful when your application uses **Microsoft Entra ID** and you want to manage Cosmos DB resources (accounts, databases, containers, throughput, and RBAC) via an SDK instead of Bicep/PowerShell/Azure CLI.

> **Important**: This sample uses the *control plane* (resource provider) APIs via ARM. It does **not** use the Cosmos DB *data plane* SDK to create ARM resources.

## Sample features

### Accounts (control plane)

- Create or update a Cosmos DB **SQL (NoSQL)** account.
- Disables local/key auth (`disableLocalAuth: true`) so **Entra ID + RBAC** is required.
- Enables the `EnableNoSQLVectorSearch` capability.
- Adds an `owner` tag (best-effort) from the signed-in identity.

### Database and container (control plane)

- Create or update a SQL database.
- Create or update a SQL container with:
  - Hierarchical partition key (multi-hash) on `/companyId`, `/departmentId`, `/userId`.
  - Indexing policy (consistent), plus a vector index on `/vectors`.
  - Vector embedding definition (1536 dims, cosine distance).
  - Unique key on `/userId`.
  - Computed property example (`cp_lowerName`).
  - TTL enabled with no default (container `defaultTtl: -1`).
  - Last-writer-wins conflict resolution (`/_ts`).
  - Autoscale max throughput from configuration.

### Throughput

- Updates **container dedicated throughput** by reading current settings first and then:
  - Updating autoscale max throughput when the container is autoscale, or
  - Updating RU/s when the container is manual throughput.
- Re-reads and prints the applied settings after the update.
- Throws a clear error when the throughput resource doesn’t exist (common for **serverless** accounts or **shared database throughput**).

### Role-based access control (RBAC)

This sample creates **two role assignments** for the currently signed-in principal:

- **Azure RBAC (control plane)**: assigns the built-in `Cosmos DB Operator` role at the Cosmos account scope.
- **Cosmos DB SQL RBAC**: assigns the built-in `Cosmos DB Built-in Data Contributor` role.

It also includes a **custom Cosmos DB SQL RBAC role definition** example (not used by default).

### Interactive menu + safe delete

- Runs an interactive menu by default.
- Includes a "Run full sample" menu option.
- This sample is interactive-only by default (it refuses to run when stdin is not a TTY).
- Supports deleting the Cosmos DB account from the menu (requires typing `DELETE` to confirm).
- Supports deleting the Cosmos DB account from the full run only when `COSMOS_SAMPLE_DELETE_ACCOUNT=true` (opt-in safety guard).

## Prerequisites

- An Azure subscription and a resource group.
- Node.js 18.18+.
- Azure identity available to `DefaultAzureCredential` (for example: `az login`, VS Code sign-in, Managed Identity, etc.).
- Permissions:
  - To create/update Cosmos resources: typically **Contributor** on the resource group.
  - To create Azure RBAC role assignments: typically **Owner** or **User Access Administrator** at the target scope.

Notes:

- These operations require a subscription id, resource group, and an Azure region (`location`) for ARM resources.
  This `location` is typically the same region as your resource group, and it does not need to match the regions where Cosmos DB data is replicated.

### VS Code setup (recommended)

If you want to run/debug this sample from VS Code:

1. Open [JavaScript.code-workspace](../JavaScript.code-workspace)
2. Ensure you have Node.js installed (18.18+)

## Usage

1. **Open the JavaScript workspace in VS Code**:

   Open [JavaScript.code-workspace](../JavaScript.code-workspace). This configures the recommended VS Code settings for this sample.

### Configuration

Copy the sample config and fill in these values:

```text
subscriptionId="..."
location="..."
resourceGroupName="..."
accountName="..."
databaseName="..."
containerName="..."
maxAutoscaleThroughput=1000
```

## Setup

This sample expects you to run from the `JavaScript/` folder.

### macOS/Linux/WSL/Git Bash

```sh
cd JavaScript
npm install
cp config.env.sample config.env
```

### Windows

```powershell
cd .\JavaScript
npm install
copy config.env.sample config.env
```

## Running

From the `JavaScript/` folder:

```sh
npm run start
```

Follow the on-screen menu prompts.

### Non-interactive execution (optional)

If you want to run specific operations without the interactive menu, pass explicit `--run-*` flags:

- `npm run start -- --run-account`
- `npm run start -- --run-db`
- `npm run start -- --run-container`
- `npm run start -- --run-throughput --throughput-delta=1000`
- `npm run start -- --run-azure-rbac`
- `npm run start -- --run-cosmos-rbac`

When stdin is not a TTY (for example, CI), the sample requires these flags and will not open the menu.

## Debugging in VS Code

Open the workspace file [JavaScript.code-workspace](../JavaScript.code-workspace) and press F5 to run **“Node.js: Debug sample”**.

In VS Code:

1. Use **File → Open Workspace from File…**
2. Select `JavaScript.code-workspace`

This keeps each sample's debug configuration independent, so developers don't need to install debug extensions for languages they aren't using.

## Azure SDK for JavaScript for Azure Cosmos DB

You can find the source code for the Azure Management SDK for JavaScript for Azure Cosmos DB and additional samples at:

- GitHub: [Azure SDK for JS - arm-cosmosdb](https://github.com/Azure/azure-sdk-for-js/tree/main/sdk/cosmosdb/arm-cosmosdb)
- npm: [@azure/arm-cosmosdb](https://www.npmjs.com/package/@azure/arm-cosmosdb)
