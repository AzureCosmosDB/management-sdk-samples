# Azure Management Python SDK Samples for Azure Cosmos DB

Repository of Azure Management Python SDK samples for creating and updating Azure Cosmos DB resources.

Applications using Entra Id cannot use Azure Cosmos DB (Data Plane) SDK `azure-cosmos` to create and modify Cosmos DB resources. These must be done through the service's Control Plane, typically using Bicep, PowerShell or Azure CLI. 

To avoid having to templates or shell scripts to do these operations, developers can use the samples here to minimize changes to their applications. 

These samples are designed to help developers who are moving from key-base authentication to Entra Id and currently use `create_if_not_exist()` functions to create database and container resources or modify throughput.

## Sample features

### Accounts
Create or update a Cosmos DB account. Includes option to enable Serverless for the account. Also includes adding firewall rules that will add local ip address, Azure data center and Portal access.

### Database and Containers
Create or update a container with hierarchical partition key (multi-hash vs. hash), Index policy, Unique keys, Container TTL with no default (set using ttl property in documents), Last Writer Wins Conflict Resolution (when using multi-region writes) and Autoscale throughput (Remove when using Serverless).

### Throughput
Upate autoscale throughput on a container.

### Role-base access control (RBAC)
Create or update a Role Defition including built-in Data Contributor as well as custom role definition with role assignment on the container. Also includes helper function to return the service principal id for the current logged in user and helper function to set the assignable scopes for the RBAC assignment.


> **Important**: Unlike Cosmos DB Data Plane SDKs, these samples require a resource group, subscription id and a location (Azure region) access Cosmos DB resources. The location is the region where the ARM resources are located (*typically the region for the resource group*). These do not need to be the same as the regions where data is stored.

## Azure SDK for Python for Azure Cosmos DB

You can find the source code for the Azure Management SDK for Python for Azure Cosmos DB and additional samples at:
[azure-mgmt-cosmosdb](https://github.com/Azure/azure-sdk-for-python/tree/main/sdk/cosmos/azure-mgmt-cosmosdb)

## Setup

### MacOS/Linux/WSL/GitBash
```sh
# Navigate to /Python folder
# Create a virtual environment
python -m venv venv

# Activate virtual environment
source venv/Scripts/activate

# Install dependencies
pip install -r requirements.txt

# Copy config.env.sample to config.env
# Then modify with your own values
cp config.env.sample config.env
```

### Windows
```sh
# Navigate to \Python folder
# Create a virtual environment
python -m venv venv

# Activate virtual environment
source venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Copy config.env.sample to config.env
# Then modify with your own values
copy config.env.sample config.env
```

## Running

Once the project is configured, set a breakpoint in main() and run in debugger to step through