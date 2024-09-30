import asyncio
import uuid
from dotenv import dotenv_values
from azure.identity.aio import DefaultAzureCredential
from azure.mgmt.resource import ResourceManagementClient
from azure.mgmt.cosmosdb import CosmosDBManagementClient
from azure.mgmt.cosmosdb.models import (
    DatabaseAccountCreateUpdateParameters,
    Location,
    DatabaseAccountKind,
    Capability,
    SqlDatabaseCreateUpdateParameters,
    SqlDatabaseResource,
    SqlContainerCreateUpdateParameters,
    SqlContainerResource,
    PartitionKey,
    IndexingPolicy,
    IncludedPath,
    ExcludedPath,
    UniqueKey,
    ConflictResolutionPolicy,
    ConflictResolutionMode,
    ThroughputSettingsUpdateParameters,
    AutoscaleSettings,
    RoleAssignmentCreateUpdateParameters,
    RoleDefinitionCreateUpdateParameters,
    RoleDefinitionType,
    RolePermission
)
from azure.mgmt.authorization.aio import AuthorizationManagementClient
from azure.mgmt.authorization.models import RoleAssignmentCreateParameters
from msgraph.core import GraphClient

# Global variables
SUBSCRIPTION_ID = "00000000-0000-0000-0000-000000000000"
RESOURCE_GROUP_NAME = "myResourceGroup"
ACCOUNT_NAME = "my-cosmos-account"
LOCATION = "East US"
DATABASE_NAME = "database1"
CONTAINER_NAME = "container1"
MAX_AUTOSCALE_THROUGHPUT = 1000

# Clients
CREDENTIAL = None
RESOURCE_CLIENT = None
COSMOS_CLIENT = None
AUTH_CLIENT = None
GRAPH_CLIENT = None


async def load_config(env_name="config.env.local"):
    config = dotenv_values(env_name)
    global SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, LOCATION, ACCOUNT_NAME, DATABASE_NAME, CONTAINER_NAME, MAX_AUTOSCALE_THROUGHPUT
    SUBSCRIPTION_ID = config['subscription_id'].strip().replace('"', ''),
    RESOURCE_GROUP_NAME = config['resource_group_name'],
    LOCATION = config['location'],
    ACCOUNT_NAME = config['account_name'],
    DATABASE_NAME = config['database_name'],
    CONTAINER_NAME = config['container_name'],
    MAX_AUTOSCALE_THROUGHPUT = int(config['max_autoscale_throughput'])
    

async def initialize_clients():
    global CREDENTIAL, RESOURCE_CLIENT, COSMOS_CLIENT, AUTH_CLIENT, GRAPH_CLIENT
    CREDENTIAL = DefaultAzureCredential()
    RESOURCE_CLIENT = ResourceManagementClient(CREDENTIAL, SUBSCRIPTION_ID)
    COSMOS_CLIENT = CosmosDBManagementClient(CREDENTIAL, SUBSCRIPTION_ID)
    AUTH_CLIENT = AuthorizationManagementClient(CREDENTIAL, SUBSCRIPTION_ID)
    GRAPH_CLIENT = GraphClient(credential=CREDENTIAL)

async def initialize_subscription():
    # Get the default subscription
    subscription = await RESOURCE_CLIENT.subscriptions.get(SUBSCRIPTION_ID)
    print(f"Subscription ID: {subscription.subscription_id}")

async def get_local_ip_address():
    async with aiohttp.ClientSession() as session:
        async with session.get("https://api.ipify.org") as response:
            public_ip_address = await response.text()
            print(f"Public IP Address: {public_ip_address}")
            return public_ip_address

async def create_or_update_cosmos_db_account():
    properties = DatabaseAccountCreateUpdateParameters(
        location=LOCATION,
        locations=[Location(location_name=LOCATION, failover_priority=0, is_zone_redundant=False)],
        kind=DatabaseAccountKind.GLOBAL_DOCUMENT_DB,
        capabilities=[Capability(name="EnableNoSQLVectorSearch")],
        disable_local_auth=True,
        public_network_access="Enabled",
        tags={"key1": "value1", "key2": "value2"}
    )

    response = await COSMOS_CLIENT.database_accounts.begin_create_or_update(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, properties
    )
    account = await response.result()
    print(f"Created new Account: {account.id}")

async def create_or_update_cosmos_db_database():
    properties = SqlDatabaseCreateUpdateParameters(
        location=LOCATION,
        resource=SqlDatabaseResource(id=DATABASE_NAME)
    )

    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_database(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, DATABASE_NAME, properties
    )
    database = await response.result()
    print(f"Created new Database: {database.id}")

async def create_or_update_cosmos_db_container():
    properties = SqlContainerCreateUpdateParameters(
        location=LOCATION,
        resource=SqlContainerResource(
            id=CONTAINER_NAME,
            default_ttl=-1,
            partition_key=PartitionKey(
                paths=["/companyId", "/departmentId", "/userId"],
                kind="MultiHash",
                version=2
            ),
            indexing_policy=IndexingPolicy(
                automatic=True,
                indexing_mode="consistent",
                included_paths=[IncludedPath(path="/*")],
                excluded_paths=[ExcludedPath(path="/\"_etag\"/?")]
            ),
            unique_keys=[UniqueKey(paths=["/userId"])],
            conflict_resolution_policy=ConflictResolutionPolicy(
                mode=ConflictResolutionMode.LAST_WRITER_WINS,
                conflict_resolution_path="/_ts"
            )
        ),
        options={"autoscale_max_throughput": MAX_AUTOSCALE_THROUGHPUT}
    )

    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_container(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, DATABASE_NAME, CONTAINER_NAME, properties
    )
    container = await response.result()
    print(f"Created new Container: {container.id}")

async def update_throughput(add_throughput):
    properties = ThroughputSettingsUpdateParameters(
        location=LOCATION,
        resource=ThroughputSettingsResource(
            autoscale_settings=AutoscaleSettings(max_throughput=MAX_AUTOSCALE_THROUGHPUT + add_throughput)
        )
    )

    response = await COSMOS_CLIENT.sql_resources.begin_update_sql_container_throughput(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, DATABASE_NAME, CONTAINER_NAME, properties
    )
    throughput = await response.result()
    print(f"Updated Container throughput: {throughput.id}")

async def create_or_update_role_assignment(role_definition_id):
    principal_id = await get_current_user_principal_id()
    assignable_scope = get_assignable_scope("Account")

    properties = RoleAssignmentCreateUpdateParameters(
        role_definition_id=role_definition_id,
        principal_id=principal_id,
        scope=assignable_scope
    )

    role_assignment_id = str(uuid.uuid4())
    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_role_assignment(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, role_assignment_id, properties
    )
    role_assignment = await response.result()
    print(f"Created new Role Assignment: {role_assignment.id}")

async def get_built_in_data_contributor_role_definition():
    # Built-in roles are predefined roles that are available in Azure Cosmos DB
    # Cosmos DB Built-in Data Contributor role definition ID
    role_definition_id = "00000000-0000-0000-0000-000000000002"
    role_definition = await COSMOS_CLIENT.sql_resources.get_sql_role_definition(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, role_definition_id
    )
    return role_definition.id

async def create_or_update_custom_role_definition():
    # Create a custom role definition that does everything Data Contributor does, but doesn't allow deletes
    assignable_scope = get_assignable_scope("Account")

    properties = RoleDefinitionCreateUpdateParameters(
        role_name="My Custom Cosmos DB Data Contributor Except Delete",
        type=RoleDefinitionType.CUSTOM_ROLE,
        assignable_scopes=[assignable_scope],
        permissions=[
            RolePermission(
                data_actions=[
                    "Microsoft.DocumentDB/databaseAccounts/readMetadata",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts"
                ]
            )
        ]
    )

    role_definition_id = str(uuid.uuid4())
    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_role_definition(
        RESOURCE_GROUP_NAME, ACCOUNT_NAME, role_definition_id, properties
    )
    role_definition = await response.result()
    print(f"Created new Role Definition: {role_definition.id}")
    return role_definition.id

def get_assignable_scope(scope):
    # Switch statement to set the permission scope
    scope_mapping = {
        "Subscription": f"/subscriptions/{SUBSCRIPTION_ID}",
        "ResourceGroup": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}",
        "Account": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}/providers/Microsoft.DocumentDB/databaseAccounts/{ACCOUNT_NAME}",
        "Database": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}/providers/Microsoft.DocumentDB/databaseAccounts/{ACCOUNT_NAME}/dbs/{DATABASE_NAME}",
        "Container": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}/providers/Microsoft.DocumentDB/databaseAccounts/{ACCOUNT_NAME}/dbs/{DATABASE_NAME}/colls/{CONTAINER_NAME}"
    }
    return scope_mapping.get(scope, scope_mapping["Account"])

async def get_current_user_principal_id():
    # Get the principal Id of the current logged-in user
    user = await GRAPH_CLIENT.get("/me")
    if not user or not user.get("id"):
        raise ValueError("User or User ID is null.")
    return user["id"]

async def main():
    await load_config()
    await initialize_clients()
    await initialize_subscription()
    await create_or_update_cosmos_db_account()
    await create_or_update_cosmos_db_database()
    await create_or_update_cosmos_db_container()
    await update_throughput(1000)
    await create_or_update_role_assignment(await get_built_in_data_contributor_role_definition())
    await create_or_update_role_assignment(await create_or_update_custom_role_definition())

if __name__ == "__main__":
    asyncio.run(main())