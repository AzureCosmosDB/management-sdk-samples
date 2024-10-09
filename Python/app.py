import asyncio
import aiohttp
import uuid
from dotenv import dotenv_values

from azure.identity.aio import DefaultAzureCredential
from msgraph import GraphServiceClient

from azure.mgmt.resource import ResourceManagementClient

from azure.mgmt.cosmosdb.aio import CosmosDBManagementClient
from azure.mgmt.cosmosdb.models import (
    DatabaseAccountCreateUpdateParameters,
    Location,
    DatabaseAccountKind,
    Capability,
    IpAddressOrRange,
    SqlDatabaseCreateUpdateParameters,
    SqlDatabaseResource,
    SqlContainerCreateUpdateParameters,
    SqlContainerResource,
    ContainerPartitionKey,
    IndexingPolicy,
    IncludedPath,
    ExcludedPath,
    UniqueKey,
    ConflictResolutionPolicy,
    ConflictResolutionMode,
    ThroughputSettingsResource,
    ThroughputSettingsUpdateParameters,
    AutoscaleSettingsResource,
    RoleDefinitionType,
    Permission,
    SqlRoleAssignmentCreateUpdateParameters,
    SqlRoleDefinitionCreateUpdateParameters
)


# Global variables
SUBSCRIPTION_ID = "00000000-0000-0000-0000-000000000000"
RESOURCE_GROUP_NAME = "myResourceGroup"
ACCOUNT_NAME = "my-cosmos-account"
LOCATION = "East US"
DATABASE_NAME = "database1"
CONTAINER_NAME = "container1"
MAX_AUTOSCALE_THROUGHPUT = 1000

# Clients
RESOURCE_CLIENT = None
COSMOS_CLIENT = None
GRAPH_CLIENT = None


async def load_config(env_name="config.env.local"):
    config = dotenv_values(env_name)
    global SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, LOCATION, ACCOUNT_NAME, DATABASE_NAME, CONTAINER_NAME, MAX_AUTOSCALE_THROUGHPUT
    SUBSCRIPTION_ID = config['subscription_id'].strip().lower().replace('"', '')
    RESOURCE_GROUP_NAME = config['resource_group_name']
    LOCATION = config['location']
    ACCOUNT_NAME = config['account_name']
    DATABASE_NAME = config['database_name']
    CONTAINER_NAME = config['container_name']
    MAX_AUTOSCALE_THROUGHPUT = int(config['max_autoscale_throughput'])

async def initialize_clients():
    
    global RESOURCE_CLIENT, COSMOS_CLIENT, GRAPH_CLIENT
    credential = DefaultAzureCredential()
    
    RESOURCE_CLIENT = ResourceManagementClient(credential=credential, subscription_id=SUBSCRIPTION_ID)
    COSMOS_CLIENT = CosmosDBManagementClient(credential=credential, subscription_id=SUBSCRIPTION_ID)
    GRAPH_CLIENT = GraphServiceClient(credentials=credential) 

async def run_az_cli_command(command):
    process = await asyncio.create_subprocess_shell(
        command,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE
    )
    stdout, stderr = await process.communicate()

    if process.returncode != 0:
        print(f"Command failed with error: {stderr.decode()}")
        return None
    return stdout.decode()

async def get_default_subscription():
    
    # Get the default subscription
    
    # The Azure Management SDK doesn't have a way to get the default subscription ID.
    # This function is a workaround to get the default subscription ID stored locally from the Azure CLI
    # Not working as expected. Need to fix this.
    
    command = "az account list --query \"[?isDefault==true].id\" --output json"
    subscription_id = await run_az_cli_command(command)
    if subscription_id:
        subscription_id = subscription_id.strip()
        print(f"Default Subscription ID: {subscription_id}")
    else:
        print("No default subscription found or command failed.")

async def get_local_ip_address():
    async with aiohttp.ClientSession() as session:
        async with session.get("https://api.ipify.org") as response:
            public_ip_address = await response.text()
            print(f"Public IP Address: {public_ip_address}")
            return public_ip_address

async def get_ip_firewall_rules():
    
    # Add IP firewall rules
    # See https://docs.microsoft.com/azure/cosmos-db/how-to-configure-firewall
    
    ip_firwall_rules = []
    
    # Add the local IP address
    ip_firwall_rules.append(IpAddressOrRange(ip_address_or_range = await get_local_ip_address()))
    
    # Add Azure datacenter IP and Portal IPs (Public Cloud).
    portal_ips = ["0.0.0.0","104.42.195.92","40.113.96.14","104.42.11.145","137.117.230.240","168.61.72.237"]
    for portal_ip in portal_ips:
        ip_firwall_rules.append(IpAddressOrRange(ip_address_or_range = portal_ip))
        
    return ip_firwall_rules

async def create_or_update_cosmos_db_account():
    
    ip_firewall_rules = []
    ip_firewall_rules = await get_ip_firewall_rules() # Uncomment out apply IP firewall rules
    
    properties = DatabaseAccountCreateUpdateParameters(
        location = LOCATION,
        kind = DatabaseAccountKind.GLOBAL_DOCUMENT_DB,
        locations = 
        [ 
            Location( location_name = LOCATION, failover_priority = 0, is_zone_redundant = False ) 
        ],
        capabilities = 
        [
            # When used with serverless, omit throughput in options in container create
            #Capability( name = "EnableServerless" )
        ],
        disable_local_auth = True,
        public_network_access = "Enabled",
        ip_rules = ip_firewall_rules,
        tags = {"key1": "value1", "key2": "value2"}
    )
    
    response = await COSMOS_CLIENT.database_accounts.begin_create_or_update(
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        create_update_parameters = properties
    )
    account = await response.result()
    print(f"Created new Account: {account.id}")

async def create_or_update_cosmos_db_database():
    properties = SqlDatabaseCreateUpdateParameters(
        location = LOCATION,
        resource = SqlDatabaseResource(id=DATABASE_NAME)
    )

    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_database(
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        database_name = DATABASE_NAME, 
        create_update_sql_database_parameters = properties
    )
    database = await response.result()
    print(f"Created new Database: {database.id}")

async def create_or_update_cosmos_db_container():
    properties = SqlContainerCreateUpdateParameters(
        location = LOCATION,
        resource = SqlContainerResource(
            id = CONTAINER_NAME,
            default_ttl = -1,
            partition_key = ContainerPartitionKey(
                paths = [ "/companyId", "/departmentId", "/userId" ],
                kind = "MultiHash",
                version = 2
            ),
            indexing_policy = IndexingPolicy(
                automatic = True,
                indexing_mode = "consistent",
                included_paths = [ IncludedPath( path = "/*" ) ], 
                excluded_paths = [ ExcludedPath( path = "/\"_etag\"/?" ) ]
            ),
            unique_keys = [ UniqueKey( paths = [ "/userId" ] ) ],
            conflict_resolution_policy = ConflictResolutionPolicy(
                mode = ConflictResolutionMode.LAST_WRITER_WINS,
                conflict_resolution_path = "/_ts"
            )
        ),
        # When using serverless, omit throughput,  options={}
        options={ "autoscaleSettings": { "maxThroughput": MAX_AUTOSCALE_THROUGHPUT } }
    )

    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_container(
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        database_name = DATABASE_NAME, 
        container_name = CONTAINER_NAME, 
        create_update_sql_container_parameters = properties
    )
    container = await response.result()
    print(f"Created new Container: {container.id}")

async def update_throughput(add_throughput):
    
    properties = ThroughputSettingsUpdateParameters(
        location = LOCATION,
        resource = ThroughputSettingsResource(
            autoscale_settings = AutoscaleSettingsResource(
                max_throughput = MAX_AUTOSCALE_THROUGHPUT + add_throughput
            )
        )
    )

    response = await COSMOS_CLIENT.sql_resources.begin_update_sql_container_throughput(
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        database_name = DATABASE_NAME, 
        container_name = CONTAINER_NAME, 
        update_throughput_parameters = properties
    )
    throughput = await response.result()
    print(f"Updated Container throughput: {throughput.id}")

async def create_or_update_role_assignment(role_definition_id):
    
    principal_id = await get_current_user_principal_id()
    assignable_scope = get_assignable_scope("Account")

    properties = SqlRoleAssignmentCreateUpdateParameters(
        role_definition_id = role_definition_id,
        principal_id = principal_id,
        scope = assignable_scope
    )

    role_assignment_id = str(uuid.uuid4())
    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_role_assignment(
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        role_assignment_id = role_assignment_id, 
        create_update_sql_role_assignment_parameters = properties
    )
    role_assignment = await response.result()
    print(f"Created new Role Assignment: {role_assignment.id}")

async def get_built_in_data_contributor_role_definition():
    
    # Built-in roles are predefined roles that are available in Azure Cosmos DB
    
    # Cosmos DB Built-in Data Contributor role definition ID
    role_definition_id = "00000000-0000-0000-0000-000000000002"
    
    role_definition = await COSMOS_CLIENT.sql_resources.get_sql_role_definition(
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        role_definition_id = role_definition_id
    )
    return role_definition.id

async def create_or_update_custom_role_definition():
    
    # Create a custom role definition that does everything Data Contributor does, but doesn't allow deletes
    assignable_scope = get_assignable_scope("Account")

    properties = SqlRoleDefinitionCreateUpdateParameters(
        role_name = "My Custom Cosmos DB Data Contributor Except Delete",
        type = RoleDefinitionType.CUSTOM_ROLE,
        assignable_scopes = [assignable_scope],
        permissions = [
            Permission(
                data_actions = [
                    "Microsoft.DocumentDB/databaseAccounts/readMetadata",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace",
                    "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert",
                    #"Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/delete",
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
        resource_group_name = RESOURCE_GROUP_NAME, 
        account_name = ACCOUNT_NAME, 
        role_definition_id = role_definition_id, 
        create_update_sql_role_definition_parameters = properties
    )
    role_definition = await response.result()
    print(f"Created new Role Definition: {role_definition.id}")
    return role_definition.id

def get_assignable_scope(scope):
    
    # Switch statement to set the permission scope
    scope_mapping = {
        "Account": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}/providers/Microsoft.DocumentDB/databaseAccounts/{ACCOUNT_NAME}",
        "Database": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}/providers/Microsoft.DocumentDB/databaseAccounts/{ACCOUNT_NAME}/dbs/{DATABASE_NAME}",
        "Container": f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP_NAME}/providers/Microsoft.DocumentDB/databaseAccounts/{ACCOUNT_NAME}/dbs/{DATABASE_NAME}/colls/{CONTAINER_NAME}"
    }
    return scope_mapping.get(scope, scope_mapping["Account"])

async def get_current_user_principal_id():
    
    # Get the principal Id of the current logged-in user
    
    me = await GRAPH_CLIENT.me.get()
    if me:
        principal_id = me.id
        print(f"User ID: {me.id}, User Principal Name: {me.user_principal_name}")
        
    return principal_id
    
def enable_debug():
    import sys
    import logging

    logging.basicConfig(level=logging.DEBUG,
                        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
                        stream=sys.stdout)

async def main():
    
    #enable_debug()
    
    await load_config()
    await initialize_clients()
    #await get_default_subscription()
    await create_or_update_cosmos_db_account()
    await create_or_update_cosmos_db_database()
    await create_or_update_cosmos_db_container()
    await update_throughput(1000)
    await create_or_update_role_assignment(await get_built_in_data_contributor_role_definition())
    await create_or_update_role_assignment(await create_or_update_custom_role_definition())

if __name__ == "__main__":
    asyncio.run(main())