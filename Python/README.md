# Azure Management Python SDK Sample for Azure Cosmos DB

This folder contains a Python sample that uses the **Azure Resource Manager (ARM) / Management Plane** SDKs to create and update **Azure Cosmos DB** resources.

This is useful when your application uses **Microsoft Entra ID** and you want to manage Cosmos DB resources (accounts, databases, containers, throughput, and RBAC) via an SDK instead of Bicep/PowerShell/Azure CLI.

> **Important**: This sample uses the *control plane* (resource provider) APIs via ARM. It does **not** use the Cosmos DB *data plane* SDK to create ARM resources.

## Sample features

### Accounts (control plane)

- Create or update a Cosmos DB **SQL (NoSQL)** account.
- Disables local/key auth (`disable_local_auth=True`) so **Entra ID + RBAC** is required.
- Enables the `EnableNoSQLVectorSearch` capability.
- Includes a commented-out **serverless** capability example.
- Adds an `owner` tag (best-effort) from the signed-in identity.

### Database and container (control plane)

- Create or update a SQL database.
- Create or update a SQL container with:
	- Hierarchical partition key (multi-hash) on `/companyId`, `/departmentId`, `/userId`.
	- Indexing policy (consistent), plus a vector index on `/vectors`.
	- Vector embedding definition (1536 dims, cosine distance).
	- Unique key on `/userId`.
	- Computed property example (`cp_lowerName`).
	- TTL enabled with no default (container `default_ttl=-1`).
	- Last-writer-wins conflict resolution (`/_ts`).
	- Autoscale max throughput from configuration.

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
- This sample is interactive-only by design (it refuses to run when stdin is not a TTY).
- Supports deleting the Cosmos DB account:
	- From the menu (requires typing `DELETE` to confirm)
	- From the full run only when `COSMOS_SAMPLE_DELETE_ACCOUNT=true` (opt-in safety guard)

## Prerequisites

- An Azure subscription and a resource group.
- Python 3.10+.
- Azure identity available to `DefaultAzureCredential` (for example: `az login`, VS Code sign-in, Managed Identity, etc.).
- Permissions:
	- To create/update Cosmos resources: typically **Contributor** on the resource group.
	- To create Azure RBAC role assignments: typically **Owner** or **User Access Administrator** at the target scope.

Notes:
- These operations require a subscription id, resource group, and an Azure region (`location`) for ARM resources.
	This `location` is typically the same region as your resource group, and it does not need to match the regions where Cosmos DB data is replicated.

### VS Code setup (recommended)

If you want to run/debug this sample from VS Code:

1. Install the Python extension: `ms-python.python`
2. Open [Python.code-workspace](../Python.code-workspace)
3. Select the interpreter from the local virtual environment (`Python/.venv`)

## Usage

1. **Open the Python workspace in VS Code**:

	 Open [Python.code-workspace](../Python.code-workspace). This configures the recommended VS Code settings for this sample.

### Configuration

Create and edit `config.env` and fill in these values:

```text
subscription_id = "..."
location = "..."
resource_group_name = "..."
account_name = "..."
database_name = "..."
container_name = "..."
max_autoscale_throughput = 1000
```

## Setup

This sample expects you to run from the `Python/` folder using a local virtual environment in `Python/.venv`.

### macOS/Linux/WSL/Git Bash

```sh
cd Python
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements.txt
cp config.env.sample config.env
```

### Windows

```powershell
cd .\Python
python -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install -r requirements.txt
copy config.env.sample config.env
```

## Running

From the `Python/` folder (with the virtual environment activated):

```sh
python app.py
```

Follow the on-screen menu prompts.

## Debugging in VS Code

Open the workspace file [Python.code-workspace](../Python.code-workspace) and press F5 to run **“Python: Debug sample”**.

In VS Code:

1. Use **File → Open Workspace from File…**
2. Select `Python.code-workspace`

This keeps each sample's debug configuration independent, so developers don't need to install debug extensions for languages they aren't using.

## Azure SDK for Python for Azure Cosmos DB

You can find the source code for the Azure Management SDK for Python for Azure Cosmos DB and additional samples at:
[azure-mgmt-cosmosdb](https://github.com/Azure/azure-sdk-for-python/tree/main/sdk/cosmos/azure-mgmt-cosmosdb)
