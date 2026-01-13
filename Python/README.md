# Azure Cosmos DB Management SDK Sample (Python)

This sample uses the **Azure Management SDK for Python** (control plane) to create and update Azure Cosmos DB resources.

If your application authenticates with **Microsoft Entra ID**, you still use the control plane (ARM) to create or modify Cosmos DB accounts/databases/containers/throughput. This sample demonstrates those operations with a simple **interactive menu**.

> **Important**: These operations require a subscription ID, resource group, and an Azure region (`location`) for the ARM resources. This `location` is typically the same region as your resource group, and it does not need to match the regions where Cosmos DB data is replicated.

## What this sample does

The app is **menu-driven** (no “run full sample” mode). Each option is safe to rerun because it uses **create-or-update** operations.

- **Create/update Cosmos DB account**
	- Adds the `owner` tag (best-effort) from the signed-in identity.
	- Optionally enables **Serverless** (see the commented `EnableServerless` capability in the code).
	- Enables the `EnableNoSQLVectorSearch` capability.
- **Create/update NoSQL database**
- **Create/update NoSQL container**
	- Demonstrates partition key, indexing (including vector index), unique keys, computed property, TTL, and conflict resolution settings (when using multi-region writes).
- **Update container throughput**
	- Updates either autoscale or manual throughput based on what is configured.
	- If the container has no dedicated throughput (inherited from the database) or the account is serverless, the sample prints a message and does not change throughput.
- **Assign Azure RBAC role (Cosmos DB Operator)**
	- Creates/updates an Azure RBAC role assignment for the current identity at the **account** scope.
- **Assign Cosmos NoSQL RBAC role (Built-in Data Contributor)**
	- Creates/updates an Azure Cosmos DB NoSQL RBAC role assignment for the current identity at the **account** scope.
- **Delete Cosmos DB account**
	- Requires typing **`DELETE`** to confirm.

## Azure SDK for Python for Azure Cosmos DB

You can find the source code for the Azure Management SDK for Python for Azure Cosmos DB and additional samples at:
[azure-mgmt-cosmosdb](https://github.com/Azure/azure-sdk-for-python/tree/main/sdk/cosmos/azure-mgmt-cosmosdb)

## Setup

### Prerequisites

- Python installed
- An Azure identity available to `DefaultAzureCredential` (for example, run `az login`)
- An existing resource group
- Permissions to create Cosmos DB resources and (for the RBAC menu options) permissions to create role assignments

### Create and activate a virtual environment

This sample expects you to run from the `Python` folder using a local virtual environment in `Python/.venv`.

### macOS/Linux/WSL/Git Bash

```sh
# Navigate to /Python folder
cd Python

# Create a virtual environment
python3 -m venv .venv

# Activate virtual environment
source .venv/bin/activate

# Install dependencies
python -m pip install -r requirements.txt

# Copy config.env.sample to config.env
# Then modify with your own values
cp config.env.sample config.env
```

### Windows

```sh
# Navigate to \Python folder
cd .\Python

# Create a virtual environment
python -m venv .venv

# Activate virtual environment (PowerShell)
.\.venv\Scripts\Activate.ps1

# Activate virtual environment (cmd.exe)
# venv\Scripts\activate.bat

# Install dependencies
python -m pip install -r requirements.txt

# Copy config.env.sample to config.env
# Then modify with your own values
copy config.env.sample config.env
```

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

## Running

From the `Python` folder:

```sh
python app.py
```

Follow the on-screen menu prompts.

## Debugging in VS Code

Open the workspace file [Python.code-workspace](../Python.code-workspace) and press F5 to run **“Python: Debug sample”**.

### How to open a workspace file (any language)

In VS Code:

1. Use **File → Open Workspace from File…**
2. Select the `*.code-workspace` file you want (for example: `Python.code-workspace`, `Csharp.code-workspace`, `Go.code-workspace`, or `Java.code-workspace`).

This keeps each sample's debug configuration independent, so developers don't need to install debug extensions for languages they aren't using.
