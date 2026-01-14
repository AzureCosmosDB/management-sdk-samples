import asyncio
import base64
from dataclasses import dataclass
import json
import os
from pathlib import Path
import sys
import uuid
from dotenv import dotenv_values

from azure.core.exceptions import HttpResponseError

from azure.identity.aio import DefaultAzureCredential

from azure.mgmt.authorization.aio import AuthorizationManagementClient
from azure.mgmt.authorization.models import RoleAssignmentCreateParameters

from azure.mgmt.cosmosdb.aio import CosmosDBManagementClient
from azure.mgmt.cosmosdb.models import (
    DatabaseAccountCreateUpdateParameters,
    Location,
    DatabaseAccountKind,
    Capability,
    SqlDatabaseCreateUpdateParameters,
    SqlDatabaseResource,
    SqlContainerCreateUpdateParameters,
    SqlContainerResource,
    ContainerPartitionKey,
    IndexingPolicy,
    IncludedPath,
    ExcludedPath,
    UniqueKey,
    UniqueKeyPolicy,
    ComputedProperty,
    ConflictResolutionPolicy,
    ConflictResolutionMode,
    ThroughputSettingsResource,
    ThroughputSettingsUpdateParameters,
    AutoscaleSettingsResource,
    RoleDefinitionType,
    Permission,
    SqlRoleAssignmentCreateUpdateParameters,
    SqlRoleDefinitionCreateUpdateParameters,
    VectorIndex,
    VectorIndexType,
    VectorEmbeddingPolicy,
    VectorEmbedding,
    VectorDataType,
    DistanceFunction
)


@dataclass(frozen=True)
class Settings:
    subscription_id: str
    resource_group_name: str
    account_name: str
    location: str
    database_name: str
    container_name: str
    max_autoscale_throughput: int = 1000

settings: Settings | None = None

# Clients
COSMOS_CLIENT = None
AUTHZ_CLIENT = None
CREDENTIAL = None


def load_config(env_name: str = "config.env") -> Settings:
    """Load required settings from a local `config.env` file and fail fast.

    This sample intentionally reads configuration from the `config.env` file next to this script
    (see `config.env.sample`) to keep setup predictable.

    Args:
        env_name: Config filename (or absolute path) to load.

    Returns:
        Parsed Settings for the sample.

    Raises:
        FileNotFoundError: If the config file does not exist.
        ValueError: If required keys are missing or invalid.
    """
    base_dir = Path(__file__).resolve().parent
    config_path = (base_dir / env_name) if not Path(env_name).is_absolute() else Path(env_name)

    if not config_path.exists():
        raise FileNotFoundError(
            f"Missing {config_path.name}. Copy config.env.sample to {config_path.name} and fill in your values."
        )

    env = dotenv_values(str(config_path))

    def _clean(value: str | None) -> str | None:
        if value is None:
            return None
        value = str(value).strip().strip('"')
        return value if value else None

    required_keys = [
        "subscription_id",
        "resource_group_name",
        "account_name",
        "location",
        "database_name",
        "container_name",
    ]
    missing = [key for key in required_keys if not _clean(env.get(key))]
    if missing:
        raise ValueError(
            f"Missing required keys in {config_path.name}: {', '.join(missing)}. "
            f"Copy config.env.sample to {config_path.name} and fill them in."
        )

    max_autoscale_throughput_raw = _clean(env.get("max_autoscale_throughput"))
    max_autoscale_throughput = 1000
    if max_autoscale_throughput_raw is not None:
        try:
            max_autoscale_throughput = int(max_autoscale_throughput_raw)
        except ValueError as ex:
            raise ValueError("max_autoscale_throughput must be an integer") from ex

    if max_autoscale_throughput < 1000:
        raise ValueError("max_autoscale_throughput must be >= 1000")

    return Settings(
        subscription_id=_clean(env.get("subscription_id")) or "",
        resource_group_name=_clean(env.get("resource_group_name")) or "",
        account_name=_clean(env.get("account_name")) or "",
        location=_clean(env.get("location")) or "",
        database_name=_clean(env.get("database_name")) or "",
        container_name=_clean(env.get("container_name")) or "",
        max_autoscale_throughput=max_autoscale_throughput,
    )

async def initialize_clients():
    """Initialize Azure SDK clients used by this sample.

    Creates a `DefaultAzureCredential`, then initializes:
    - Cosmos control-plane client (`CosmosDBManagementClient`)
    - Azure RBAC client (`AuthorizationManagementClient`)
    """
    print("Initializing clients...")
    
    global COSMOS_CLIENT, AUTHZ_CLIENT, CREDENTIAL

    CREDENTIAL = DefaultAzureCredential()

    COSMOS_CLIENT = CosmosDBManagementClient(credential=CREDENTIAL, subscription_id=settings.subscription_id)
    AUTHZ_CLIENT = AuthorizationManagementClient(credential=CREDENTIAL, subscription_id=settings.subscription_id)

async def close_clients():
    """Close SDK clients and release credential resources."""
    global COSMOS_CLIENT, AUTHZ_CLIENT, CREDENTIAL

    try:
        if COSMOS_CLIENT is not None:
            await COSMOS_CLIENT.close()
    finally:
        COSMOS_CLIENT = None

    try:
        if AUTHZ_CLIENT is not None:
            await AUTHZ_CLIENT.close()
    finally:
        AUTHZ_CLIENT = None

    try:
        if CREDENTIAL is not None:
            await CREDENTIAL.close()
    finally:
        CREDENTIAL = None

def _b64url_decode(data: str) -> bytes:
    """Decode base64url-encoded JWT segments."""
    padding = "=" * (-len(data) % 4)
    return base64.urlsafe_b64decode(data + padding)

async def get_current_principal_id() -> str:
    """Return the current principal's Entra object id (oid) without Microsoft Graph.

    Resolution order:
    1) `AZURE_PRINCIPAL_OBJECT_ID` or `principal_object_id` (explicit override)
    2) `oid` claim from an ARM access token acquired via `DefaultAzureCredential`
    """

    override = os.getenv("AZURE_PRINCIPAL_OBJECT_ID") or os.getenv("principal_object_id")
    if override and override.strip():
        return override.strip()

    # Token claims (works for users, service principals, managed identities).
    token = await CREDENTIAL.get_token("https://management.azure.com/.default")
    parts = token.token.split(".")
    if len(parts) >= 2:
        payload = json.loads(_b64url_decode(parts[1]).decode("utf-8"))
        oid = payload.get("oid")
        if oid:
            return oid

    raise RuntimeError(
        "Could not determine current principal object id (oid) from the access token. "
        "Set AZURE_PRINCIPAL_OBJECT_ID (or principal_object_id) to the object id you want to assign roles to."
    )

async def get_current_user_email() -> str | None:
    """Return best-effort email/UPN for tagging/ownership metadata."""
    token = await CREDENTIAL.get_token("https://management.azure.com/.default")
    parts = token.token.split(".")
    if len(parts) < 2:
        return None

    payload = json.loads(_b64url_decode(parts[1]).decode("utf-8"))
    for key in ("preferred_username", "upn", "unique_name"):
        value = payload.get(key)
        if isinstance(value, str) and value.strip():
            return value
    return None


async def create_or_update_cosmos_db_account():
    """Create or update a Cosmos DB account (control plane).

    Configures:
    - `DisableLocalAuth=True` to require Entra ID + RBAC
    - Vector search capability (NoSQL vector search)
    - Best-effort `owner` tag from the current identity
    """
    print ("Creating Cosmos DB account...")
    
    owner_email = await get_current_user_email()

    properties = DatabaseAccountCreateUpdateParameters(
        location = settings.location,
        kind = DatabaseAccountKind.GLOBAL_DOCUMENT_DB,
        locations = 
        [ 
            Location( 
                location_name = settings.location, 
                failover_priority = 0, 
                is_zone_redundant = False 
            ) 
        ],
        capabilities = 
        [
            # When used with serverless, omit throughput in options in container create
            #Capability( name = "EnableServerless" )
            Capability( name = "EnableNoSQLVectorSearch" )
        ],
        disable_local_auth = True,
        tags = {"owner": owner_email or ""}
    )
    
    response = await COSMOS_CLIENT.database_accounts.begin_create_or_update(
        resource_group_name = settings.resource_group_name, 
        account_name = settings.account_name, 
        create_update_parameters = properties
    )
    
    account = await response.result()
    print(f"Created new Account: {account.id}")

async def create_or_update_cosmos_db_database():
    """Create or update a SQL database under the configured Cosmos DB account."""
    print ("Creating Cosmos DB database...")
    
    properties = SqlDatabaseCreateUpdateParameters(
        location = settings.location,
        resource = SqlDatabaseResource(id=settings.database_name)
    )

    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_database(
        resource_group_name = settings.resource_group_name, 
        account_name = settings.account_name, 
        database_name = settings.database_name, 
        create_update_sql_database_parameters = properties
    )
    
    database = await response.result()
    print(f"Created new Database: {database.id}")

async def create_or_update_cosmos_db_container():
    """Create or update a SQL container with common advanced settings.

    Demonstrates partition keys, indexing policy (including vector index), unique keys,
    computed properties, conflict resolution (for multi-region writes), and vector embeddings.
    """
    print ("Creating Cosmos DB container...")
    
    properties = SqlContainerCreateUpdateParameters(
        location=settings.location,
        resource=SqlContainerResource(
            id=settings.container_name,
            default_ttl=-1,
            partition_key=ContainerPartitionKey(
                paths=[
                    "/companyId",
                    "/departmentId",
                    "/userId"
                ],
                kind="MultiHash",
                version=2
            ),
            indexing_policy=IndexingPolicy(
                automatic=True,
                indexing_mode="consistent",
                included_paths=[
                    IncludedPath(
                        path="/*"
                    )
                ],
                excluded_paths=[
                    ExcludedPath(
                        path="/\"_etag\"/?"
                    )
                ],
                vector_indexes=[
                    VectorIndex(
                        path="/vectors",
                        type=VectorIndexType.DISK_ANN
                    )
                ]
            ),
            unique_key_policy=UniqueKeyPolicy(
                unique_keys=[
                    UniqueKey(paths=["/userId"])
                ]
            ),
            computed_properties=[
                ComputedProperty(
                    name="cp_lowerName",
                    query="SELECT VALUE LOWER(c.userName) FROM c"
                )
            ],
            # Only needed for multi-region write accounts
            conflict_resolution_policy=ConflictResolutionPolicy(
                mode=ConflictResolutionMode.LAST_WRITER_WINS,
                conflict_resolution_path="/_ts"
            ),
            vector_embedding_policy=VectorEmbeddingPolicy(
                vector_embeddings=[
                    VectorEmbedding(
                        path="/vectors",
                        dimensions=1536,
                        data_type=VectorDataType.FLOAT32,
                        distance_function=DistanceFunction.COSINE
                    )
                ]
            )
        ),
        # When using serverless, omit throughput, options={}
        options={
            "autoscaleSettings": {
                "maxThroughput": settings.max_autoscale_throughput
            }
        }
    )

    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_container(
        resource_group_name = settings.resource_group_name, 
        account_name = settings.account_name, 
        database_name = settings.database_name, 
        container_name = settings.container_name, 
        create_update_sql_container_parameters = properties
    )
    container = await response.result()
    print(f"Created new Container: {container.id}")

async def update_throughput(add_throughput):
    """Update the container throughput by a delta.

    If the container is autoscale, increases autoscale max throughput.
    If the container is manual throughput, increases the provisioned RU/s.

    Args:
        add_throughput: Delta to add (e.g. 1000).
    """
    print("Updating container throughput...")

    try:
        existing = await COSMOS_CLIENT.sql_resources.get_sql_container_throughput(
            resource_group_name=settings.resource_group_name,
            account_name=settings.account_name,
            database_name=settings.database_name,
            container_name=settings.container_name
        )
    except HttpResponseError as ex:
        if getattr(ex, "status_code", None) == 404:
            raise RuntimeError(
                "Container throughput settings were not found. Container using database throughput or serverless."
            ) from ex
        raise

    existing_resource = getattr(existing, "resource", None)
    autoscale_settings = getattr(existing_resource, "autoscale_settings", None)
    current_autoscale_max = getattr(autoscale_settings, "max_throughput", None)
    current_manual_throughput = getattr(existing_resource, "throughput", None)

    if current_autoscale_max is not None:
        baseline = current_autoscale_max or settings.max_autoscale_throughput
        new_autoscale_max = baseline + add_throughput

        update = ThroughputSettingsUpdateParameters(
            location=settings.location,
            resource=ThroughputSettingsResource(
                autoscale_settings=AutoscaleSettingsResource(
                    max_throughput=new_autoscale_max
                )
            )
        )

        print(f"Updating container autoscale max throughput from {current_autoscale_max} to {new_autoscale_max}")
    else:
        baseline = current_manual_throughput or 0
        if baseline == 0:
            baseline = add_throughput
            add_throughput = 0

        new_manual_throughput = baseline + add_throughput

        update = ThroughputSettingsUpdateParameters(
            location=settings.location,
            resource=ThroughputSettingsResource(
                throughput=new_manual_throughput
            )
        )

        print(f"Updating container manual throughput from {current_manual_throughput} to {new_manual_throughput}")

    # Apply the update
    response = await COSMOS_CLIENT.sql_resources.begin_update_sql_container_throughput(
        resource_group_name=settings.resource_group_name,
        account_name=settings.account_name,
        database_name=settings.database_name,
        container_name=settings.container_name,
        update_throughput_parameters=update
    )
    await response.result()

    # Verify applied settings
    applied = await COSMOS_CLIENT.sql_resources.get_sql_container_throughput(
        resource_group_name=settings.resource_group_name,
        account_name=settings.account_name,
        database_name=settings.database_name,
        container_name=settings.container_name
    )

    applied_resource = getattr(applied, "resource", None)
    applied_autoscale = getattr(getattr(applied_resource, "autoscale_settings", None), "max_throughput", None)
    applied_manual = getattr(applied_resource, "throughput", None)
    print(f"Applied throughput settings: autoscaleMax={applied_autoscale}, manual={applied_manual}")

async def delete_cosmos_db_account():
    """Delete the configured Cosmos DB account (irreversible)."""
    print("Deleting Cosmos DB account...")

    response = await COSMOS_CLIENT.database_accounts.begin_delete(
        resource_group_name=settings.resource_group_name,
        account_name=settings.account_name
    )
    await response.result()
    print("Deleted Cosmos DB account.")


async def create_or_update_azure_rbac_assignment(role_definition_id: str) -> None:
    """Create or update an Azure RBAC role assignment for the current principal.

    This is control-plane RBAC (ARM): grants permission to manage the Cosmos DB account resource.
    The assignment name is deterministic so repeated runs are idempotent.

    Args:
        role_definition_id: Full Azure role definition resource id.
    """
    print("Creating Azure RBAC role assignment...")

    principal_id = await get_current_principal_id()
    scope = get_assignable_scope("Account")

    # Use a stable GUID so repeated runs are idempotent (same PUT resource each time).
    role_assignment_name = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{scope}|{role_definition_id}|{principal_id}"))
    parameters = RoleAssignmentCreateParameters(
        role_definition_id=role_definition_id,
        principal_id=principal_id,
        principal_type="User",
    )

    assignment = await AUTHZ_CLIENT.role_assignments.create(
        scope=scope,
        role_assignment_name=role_assignment_name,
        parameters=parameters,
    )

    print(f"Created Azure RBAC role assignment: {assignment.id}")

async def get_azure_role_definition_cosmos_operator() -> str:
    """Resolve the built-in Azure RBAC role definition id for "Cosmos DB Operator"."""
    print("Getting Azure RBAC role definition (Cosmos DB Operator)...")
    subscription_scope = f"/subscriptions/{settings.subscription_id}"
    return await get_azure_role_definition_id_by_name(subscription_scope, "Cosmos DB Operator")

async def create_or_update_cosmos_role_assignment(role_definition_id: str):

    """Create a Cosmos DB SQL RBAC role assignment (data plane) for the current principal.

    Data-plane RBAC controls permissions to work with databases/containers/items.

    Args:
        role_definition_id: Cosmos SQL role definition id (resource id).
    """

    print("Creating Cosmos DB Role Assignment...")

    principal_id = await get_current_principal_id()
    assignable_scope = get_assignable_scope("Account")

    properties = SqlRoleAssignmentCreateUpdateParameters(
        role_definition_id=role_definition_id,
        principal_id=principal_id,
        scope=assignable_scope
    )

    # Use a stable GUID so repeated runs are idempotent (same PUT resource each time).
    role_assignment_id = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{assignable_scope}|{role_definition_id}|{principal_id}"))
    response = await COSMOS_CLIENT.sql_resources.begin_create_update_sql_role_assignment(
        resource_group_name=settings.resource_group_name,
        account_name=settings.account_name,
        role_assignment_id=role_assignment_id,
        create_update_sql_role_assignment_parameters=properties
    )
    role_assignment = await response.result()
    print(f"Created new Role Assignment: {role_assignment.id}")

async def get_cosmos_role_definition_data_contributor():
    """Get the built-in Cosmos DB SQL RBAC Data Contributor role definition id."""
    
    # Built-in roles are predefined roles that are available in Azure Cosmos DB
    print ("Getting Built-in Data Contributor Role Definition...")
    
    # Cosmos DB Built-in Data Contributor role definition ID
    role_definition_id = "00000000-0000-0000-0000-000000000002"
    
    role_definition = await COSMOS_CLIENT.sql_resources.get_sql_role_definition(
        resource_group_name=settings.resource_group_name,
        account_name=settings.account_name,
        role_definition_id = role_definition_id
    )
    return role_definition.id

async def create_or_update_custom_data_role_definition():
    """Create a custom Cosmos DB SQL RBAC role definition.

    Example only: similar to Data Contributor, but excludes delete permissions.
    This helper is not called in the main flow.
    """
    
    # Create a custom role definition that does everything Data Contributor does, but doesn't allow deletes
    print ("Creating Custom Role Definition...")
    
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
        resource_group_name=settings.resource_group_name,
        account_name=settings.account_name,
        role_definition_id = role_definition_id, 
        create_update_sql_role_definition_parameters = properties
    )
    
    role_definition = await response.result()
    print(f"Created new Role Definition: {role_definition.id}")
    return role_definition.id

async def get_azure_role_definition_id_by_name(scope: str, role_name: str) -> str:
    """Get an Azure RBAC role definition id by role display name.
    This function is just an example of how to look up role definitions by name.

    This is used for Azure RBAC (control-plane) assignments.

    Args:
        scope: Scope at which to query role definitions (e.g. subscription scope).
        role_name: Azure RBAC role display name (e.g. "Cosmos DB Operator").

    Returns:
        The full role definition resource id.

    Raises:
        RuntimeError: If the role definition cannot be listed or is not found.
    """
    role_filter = f"roleName eq '{role_name}'"

    try:
        async for role_definition in AUTHZ_CLIENT.role_definitions.list(scope=scope, filter=role_filter):
            role_definition_id = getattr(role_definition, "id", None)
            if role_definition_id:
                return role_definition_id
    except HttpResponseError as ex:
        raise RuntimeError(f"Failed to list Azure RBAC role definitions at scope '{scope}'.") from ex

    raise RuntimeError(
        f"Azure RBAC role definition '{role_name}' was not found at scope '{scope}'. "
        "Verify the role name and that you have permission to read role definitions."
    )

def get_assignable_scope(scope):
    """Return an assignable scope resource id for the given scope name.

    Args:
        scope: One of "ResourceGroup", "Account", "Database", "Container".

    Returns:
        ARM resource id string for the requested scope.
    """
    # Switch statement to set the permission scope
    scope_mapping = {
        "ResourceGroup": f"/subscriptions/{settings.subscription_id}/resourceGroups/{settings.resource_group_name}",
        "Account": f"/subscriptions/{settings.subscription_id}/resourceGroups/{settings.resource_group_name}/providers/Microsoft.DocumentDB/databaseAccounts/{settings.account_name}",
        "Database": f"/subscriptions/{settings.subscription_id}/resourceGroups/{settings.resource_group_name}/providers/Microsoft.DocumentDB/databaseAccounts/{settings.account_name}/dbs/{settings.database_name}",
        "Container": f"/subscriptions/{settings.subscription_id}/resourceGroups/{settings.resource_group_name}/providers/Microsoft.DocumentDB/databaseAccounts/{settings.account_name}/dbs/{settings.database_name}/colls/{settings.container_name}"
    }
    
    return scope_mapping.get(scope, scope_mapping["Account"])


def enable_debug():
    """Enable DEBUG-level logging to stdout for Azure SDK troubleshooting."""
    import sys
    import logging

    logging.basicConfig(level=logging.DEBUG,
                        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
                        stream=sys.stdout)

async def main():
    """Program entry point.

    Loads configuration, initializes SDK clients, then runs the interactive menu.
    """
    #enable_debug()
    
    global settings
    settings = load_config()
    await initialize_clients()

    # Interactive-only by design.
    if not sys.stdin.isatty():
        print("This sample is interactive-only. Run it in a terminal (or VS Code debug) so it can prompt for input.")
        return

    try:
        await run_interactive_menu()
    finally:
        await close_clients()

async def _prompt(text: str) -> str:
    """Prompt the user in a thread to avoid blocking the event loop."""
    return (await asyncio.to_thread(input, text)).strip()

async def run_interactive_menu():
    """Run the interactive menu for the sample."""
    while True:
        print("\nCosmos management sample - choose an action:")
        print("  1) Run full sample")
        print("  2) Create/update Cosmos DB account")
        print("  3) Create Azure RBAC assignment (Cosmos DB Operator)")
        print("  4) Create/update SQL database")
        print("  5) Create/update container")
        print("  6) Update container throughput (+delta)")
        print("  7) Create Cosmos SQL RBAC assignment (Built-in Data Contributor)")
        print("  8) Delete Cosmos DB account")
        print("  0) Exit")

        selection = (await _prompt("Selection: ")).lower()

        try:
            if selection in ("0", "q", "quit", "exit"):
                return
            if selection == "1":
                await run_full_sample()
            elif selection == "2":
                await create_or_update_cosmos_db_account()
            elif selection == "3":
                await create_or_update_azure_rbac_assignment(await get_azure_role_definition_cosmos_operator())
            elif selection == "4":
                await create_or_update_cosmos_db_database()
            elif selection == "5":
                await create_or_update_cosmos_db_container()
            elif selection == "6":
                raw = await _prompt("Throughput delta to add (default 1000): ")
                delta = 1000 if not raw else int(raw)
                await update_throughput(delta)
            elif selection == "7":
                await create_or_update_cosmos_role_assignment(await get_cosmos_role_definition_data_contributor())
            elif selection == "8":
                confirm = (await _prompt("Type DELETE to confirm deleting the Cosmos DB account: ")).strip()
                if confirm == "DELETE":
                    await delete_cosmos_db_account()
                else:
                    print("Delete cancelled.")
            else:
                print("Unknown selection.")
        except Exception as ex:
            print(f"Operation failed: {ex}")

async def run_full_sample():
    """Run the full sample flow (create resources, set RBAC, update throughput).

    Mirrors the C# sample ordering and supports opt-in cleanup via `COSMOS_SAMPLE_DELETE_ACCOUNT=true`.
    """
    await create_or_update_cosmos_db_account()
    await create_or_update_azure_rbac_assignment(await get_azure_role_definition_cosmos_operator())
    await create_or_update_cosmos_db_database()
    await create_or_update_cosmos_db_container()
    await update_throughput(1000)
    await create_or_update_cosmos_role_assignment(await get_cosmos_role_definition_data_contributor())

    # Optional cleanup: set COSMOS_SAMPLE_DELETE_ACCOUNT=true to delete the account at the end of a full run.
    if os.environ.get("COSMOS_SAMPLE_DELETE_ACCOUNT", "").lower() == "true":
        await delete_cosmos_db_account()

if __name__ == "__main__":
    asyncio.run(main())