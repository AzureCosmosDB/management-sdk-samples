using Azure;
using Azure.Core;
using Azure.ResourceManager;
using Azure.ResourceManager.Authorization;
using Azure.ResourceManager.Authorization.Models;
using Azure.ResourceManager.CosmosDB;
using Azure.ResourceManager.CosmosDB.Models;
using Azure.ResourceManager.Resources;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;
using Microsoft.IdentityModel.JsonWebTokens;

internal sealed class CosmosManagement
{
    private readonly ArmClient _armClient;
    private readonly TokenCredential _credential;
    private readonly ILogger<CosmosManagement> _logger;
    private readonly AppSettings _options;

    public CosmosManagement(
        ArmClient armClient,
        TokenCredential credential,
        IOptions<AppSettings> options,
        ILogger<CosmosManagement> logger)
    {
        _armClient = armClient;
        _credential = credential;
        _logger = logger;
        _options = options.Value;
    }

    public async Task RunAsync(CancellationToken cancellationToken)
    {
        await CreateOrUpdateCosmosDBAccount(cancellationToken);

        // Control plane (Azure RBAC): grants permissions to manage the Cosmos DB account via ARM.
        await CreateOrUpdateAzureRoleAssignment(await GetBuiltInCosmosDbOperatorRoleDefinition(), cancellationToken);

        await CreateOrUpdateCosmosDBDatabase(cancellationToken);
        await CreateOrUpdateCosmosDBContainer(cancellationToken);
        await UpdateThroughput(1000, cancellationToken);

        // Data plane (Cosmos DB SQL RBAC): grants permissions to work with databases/containers/items.
        await CreateOrUpdateCosmosRoleAssignment(await GetBuiltInDataContributorRoleDefinitionAsync(cancellationToken), cancellationToken);

        // Optional cleanup: set COSMOS_SAMPLE_DELETE_ACCOUNT=true to delete the account at the end of a full run.
        // (Kept opt-in to prevent accidental deletions when running the sample.)
        if (string.Equals(Environment.GetEnvironmentVariable("COSMOS_SAMPLE_DELETE_ACCOUNT"), "true", StringComparison.OrdinalIgnoreCase))
        {
            await DeleteCosmosDBAccount(cancellationToken);
        }
    }

    public async Task RunInteractiveAsync(CancellationToken cancellationToken)
    {
        // If we're not running in an interactive terminal (e.g., CI), fall back to the full sample.
        if (Console.IsInputRedirected)
        {
            await RunAsync(cancellationToken);
            return;
        }

        while (!cancellationToken.IsCancellationRequested)
        {
            Console.WriteLine();
            Console.WriteLine("Cosmos management sample - choose an action:");
            Console.WriteLine("  1) Run full sample");
            Console.WriteLine("  2) Create/update Cosmos DB account");
            Console.WriteLine("  3) Create Azure RBAC assignment (Cosmos DB Operator)");
            Console.WriteLine("  4) Create/update SQL database");
            Console.WriteLine("  5) Create/update container");
            Console.WriteLine("  6) Update container throughput (+delta)");
            Console.WriteLine("  7) Create Cosmos SQL RBAC assignment (Built-in Data Contributor)");
            Console.WriteLine("  8) Delete Cosmos DB account");
            Console.WriteLine("  0) Exit");
            Console.Write("Selection: ");

            string? selection = Console.ReadLine()?.Trim();
            if (string.IsNullOrWhiteSpace(selection))
            {
                continue;
            }

            try
            {
                switch (selection)
                {
                    case "1":
                        await RunAsync(cancellationToken);
                        break;
                    case "2":
                        await CreateOrUpdateCosmosDBAccount(cancellationToken);
                        break;
                    case "3":
                        await CreateOrUpdateAzureRoleAssignment(await GetBuiltInCosmosDbOperatorRoleDefinition(), cancellationToken);
                        break;
                    case "4":
                        await CreateOrUpdateCosmosDBDatabase(cancellationToken);
                        break;
                    case "5":
                        await CreateOrUpdateCosmosDBContainer(cancellationToken);
                        break;
                    case "6":
                    {
                        int delta = PromptInt("Throughput delta to add", 1000);
                        await UpdateThroughput(delta, cancellationToken);
                        break;
                    }
                    case "7":
                        await CreateOrUpdateCosmosRoleAssignment(await GetBuiltInDataContributorRoleDefinitionAsync(cancellationToken), cancellationToken);
                        break;
                    case "8":
                        if (ConfirmDelete())
                        {
                            await DeleteCosmosDBAccount(cancellationToken);
                        }
                        else
                        {
                            Console.WriteLine("Delete cancelled.");
                        }
                        break;
                    case "0":
                    case "q":
                    case "quit":
                    case "exit":
                        return;
                    default:
                        Console.WriteLine("Unknown selection.");
                        break;
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Operation failed.");
            }
        }
    }

    private static int PromptInt(string label, int defaultValue)
    {
        Console.Write($"{label} (default {defaultValue}): ");
        string? raw = Console.ReadLine();
        if (string.IsNullOrWhiteSpace(raw))
        {
            return defaultValue;
        }

        return int.TryParse(raw.Trim(), out int value) ? value : defaultValue;
    }

    private static bool ConfirmDelete()
    {
        Console.Write("Type DELETE to confirm deleting the Cosmos DB account: ");
        return string.Equals(Console.ReadLine()?.Trim(), "DELETE", StringComparison.Ordinal);
    }

    /// <summary>
    /// Creates or updates a Cosmos DB account 
    /// </summary>
    public async Task CreateOrUpdateCosmosDBAccount(CancellationToken cancellationToken)
    {
        // Try to get the current user's email for tagging/ownership purposes.
        string? ownerEmail = await GetCurrentUserEmailAsync(cancellationToken);

        CosmosDBAccountCreateOrUpdateContent properties = new(
            new AzureLocation(_options.Location),
            [
                new CosmosDBAccountLocation
                {
                    LocationName = _options.Location,
                    FailoverPriority = 0,
                    IsZoneRedundant = false
                }
            ])
        {
            Kind = CosmosDBAccountKind.GlobalDocumentDB,
            Capabilities =
            {
                //new CosmosDBAccountCapability
                //{ 
                //    Name = "EnableServerless" //Don't provision container throughput when using serverless
                //},
                new CosmosDBAccountCapability
                {
                    Name = "EnableNoSQLVectorSearch"
                }
            },

            // When true, Entra ID and RBAC are required for AuthN/AuthZ
            DisableLocalAuth = true,

            Tags =
            {
                // Helpful for resource ownership / chargeback. Best-effort: may be null for non-user identities.
                { "owner", ownerEmail ?? string.Empty }
            }
        };

        ResourceIdentifier resourceId = ResourceGroupResource.CreateResourceIdentifier(_options.SubscriptionId, _options.ResourceGroupName);
        ResourceGroupResource resourceGroup = _armClient.GetResourceGroupResource(resourceId);

        CosmosDBAccountCollection cosmosAccounts = resourceGroup.GetCosmosDBAccounts();

        ArmOperation<CosmosDBAccountResource> response = await cosmosAccounts.CreateOrUpdateAsync(WaitUntil.Completed, _options.AccountName, properties, cancellationToken);
        CosmosDBAccountResource resource = response.Value;

        _logger.LogInformation("Created/updated Cosmos DB account: {AccountId}", resource.Data.Id);
    }

    public async Task DeleteCosmosDBAccount(CancellationToken cancellationToken)
    {
        ResourceIdentifier accountId = CosmosDBAccountResource.CreateResourceIdentifier(
            _options.SubscriptionId,
            _options.ResourceGroupName,
            _options.AccountName);

        CosmosDBAccountResource account = _armClient.GetCosmosDBAccountResource(accountId);
        await account.DeleteAsync(WaitUntil.Completed, cancellationToken);

        _logger.LogInformation("Deleted Cosmos DB account: {AccountId}", accountId);
    }

    public async Task CreateOrUpdateCosmosDBDatabase(CancellationToken cancellationToken)
    {
        CosmosDBSqlDatabaseCreateOrUpdateContent properties = new(
            _options.Location,
            new CosmosDBSqlDatabaseResourceInfo(_options.DatabaseName));

        ResourceIdentifier resourceId = CosmosDBAccountResource.CreateResourceIdentifier(_options.SubscriptionId, _options.ResourceGroupName, _options.AccountName);
        CosmosDBAccountResource account = _armClient.GetCosmosDBAccountResource(resourceId);

        CosmosDBSqlDatabaseCollection databases = account.GetCosmosDBSqlDatabases();

        ArmOperation<CosmosDBSqlDatabaseResource> response = await databases.CreateOrUpdateAsync(WaitUntil.Completed, _options.DatabaseName, properties, cancellationToken);
        CosmosDBSqlDatabaseResource resource = response.Value;

        _logger.LogInformation("Created/updated SQL database: {DatabaseId}", resource.Data.Id);
    }

    /// <summary>
    ///  Creates or updates a Cosmos DB SQL container with partition key, indexing policy, unique keys, 
    ///  computed properties, conflict resolution (when using multi-region writes), and vector embeddings.
    /// </summary>
    public async Task CreateOrUpdateCosmosDBContainer(CancellationToken cancellationToken)
    {
        CosmosDBSqlContainerCreateOrUpdateContent properties = new(
            _options.Location,
            new CosmosDBSqlContainerResourceInfo(_options.ContainerName)
            {
                DefaultTtl = -1,
                PartitionKey = new CosmosDBContainerPartitionKey
                {
                    Paths = { "/companyId", "/departmentId", "/userId" },
                    Kind = CosmosDBPartitionKind.MultiHash,
                    Version = 2
                },
                IndexingPolicy = new CosmosDBIndexingPolicy
                {
                    IsAutomatic = true,
                    IndexingMode = CosmosDBIndexingMode.Consistent,
                    IncludedPaths = { new CosmosDBIncludedPath { Path = "/*" } },
                    ExcludedPaths = { new CosmosDBExcludedPath { Path = "/\"_etag\"/?" } },
                    VectorIndexes = { new CosmosDBVectorIndex(path: "/vectors", indexType: CosmosDBVectorIndexType.DiskAnn) }
                },
                UniqueKeys = { new CosmosDBUniqueKey { Paths = { "/userId" } } },
                ComputedProperties =
                {
                    new ComputedProperty
                    {
                        Name = "cp_lowerName",
                        Query = "SELECT VALUE LOWER(c.userName) FROM c"
                    }
                },
                ConflictResolutionPolicy = new ConflictResolutionPolicy
                {
                    Mode = ConflictResolutionMode.LastWriterWins,
                    ConflictResolutionPath = "/_ts"
                },
                VectorEmbeddings =
                {
                    new CosmosDBVectorEmbedding(
                        path: "/vectors",
                        dataType: CosmosDBVectorDataType.Float32,
                        distanceFunction: VectorDistanceFunction.Cosine,
                        dimensions: 1536)
                }
            })
        {
            Options = new CosmosDBCreateUpdateConfig
            {
                AutoscaleMaxThroughput = _options.MaxAutoScaleThroughput
            }
        };

        ResourceIdentifier resourceId = CosmosDBSqlDatabaseResource.CreateResourceIdentifier(_options.SubscriptionId, _options.ResourceGroupName, _options.AccountName, _options.DatabaseName);
        CosmosDBSqlDatabaseResource cosmosDBDatabase = _armClient.GetCosmosDBSqlDatabaseResource(resourceId);

        CosmosDBSqlContainerCollection cosmosContainers = cosmosDBDatabase.GetCosmosDBSqlContainers();

        ArmOperation<CosmosDBSqlContainerResource> response = await cosmosContainers.CreateOrUpdateAsync(WaitUntil.Completed, _options.ContainerName, properties, cancellationToken);
        CosmosDBSqlContainerResource resource = response.Value;

        _logger.LogInformation("Created/updated container: {ContainerId}", resource.Data.Id);
    }

    /// <summary>
    /// Updates the throughput settings for the specified container by adding the given delta.
    /// </summary>
    public async Task UpdateThroughput(int addThroughput, CancellationToken cancellationToken)
    {
        // Container throughput is represented by a separate ARM child resource ("throughputSettings/default").
        ResourceIdentifier resourceId = CosmosDBSqlContainerThroughputSettingResource.CreateResourceIdentifier(
            _options.SubscriptionId,
            _options.ResourceGroupName,
            _options.AccountName,
            _options.DatabaseName,
            _options.ContainerName);

        CosmosDBSqlContainerThroughputSettingResource containerThroughput = _armClient.GetCosmosDBSqlContainerThroughputSettingResource(resourceId);

        CosmosDBSqlContainerThroughputSettingResource existing;
        try
        {
            // Read current settings first so we can tell whether the container is autoscale or manual.
            existing = (await containerThroughput.GetAsync(cancellationToken)).Value;
        }
        catch (RequestFailedException ex) when (ex.Status == 404)
        {
            // 404 typically means there's no dedicated container throughput (shared database throughput or serverless).
            throw new InvalidOperationException(
                "Container throughput settings were not found. This usually means the container uses shared database throughput or serverless, and therefore does not have a dedicated throughput resource to update. Create the container with dedicated throughput (or update database throughput instead), then retry.",
                ex);
        }

        // Autoscale containers expose AutoscaleSettings; manual throughput containers expose Throughput.
        int? currentAutoscaleMax = existing.Data.Resource?.AutoscaleSettings?.MaxThroughput;
        int? currentManualThroughput = existing.Data.Resource?.Throughput;

        ThroughputSettingsUpdateData update;
        if (currentAutoscaleMax is not null)
        {
            // Autoscale: update max RU/s (not current RU/s).
            int newAutoscaleMax = checked((currentAutoscaleMax.Value == 0 ? _options.MaxAutoScaleThroughput : currentAutoscaleMax.Value) + addThroughput);
            update = new ThroughputSettingsUpdateData(
                new AzureLocation(_options.Location),
                new ThroughputSettingsResourceInfo
                {
                    AutoscaleSettings = new AutoscaleSettingsResourceInfo(newAutoscaleMax)
                });

            _logger.LogInformation(
                "Updating container autoscale max throughput from {CurrentAutoscaleMax} to {NewAutoscaleMax}",
                currentAutoscaleMax,
                newAutoscaleMax);
        }
        else
        {
            // Manual: update the fixed RU/s value.
            int baseline = currentManualThroughput ?? 0;
            if (baseline == 0)
            {
                // If the service didn't return a baseline, treat the input as the absolute target.
                baseline = addThroughput;
                addThroughput = 0;
            }

            int newManualThroughput = checked(baseline + addThroughput);
            update = new ThroughputSettingsUpdateData(
                new AzureLocation(_options.Location),
                new ThroughputSettingsResourceInfo
                {
                    Throughput = newManualThroughput
                });

            _logger.LogInformation(
                "Updating container manual throughput from {CurrentManualThroughput} to {NewManualThroughput}",
                currentManualThroughput,
                newManualThroughput);
        }

        // Apply the update, then re-read to log the values actually persisted by the service.
        ArmOperation<CosmosDBSqlContainerThroughputSettingResource> response = await containerThroughput.CreateOrUpdateAsync(WaitUntil.Completed, update, cancellationToken);
        CosmosDBSqlContainerThroughputSettingResource updated = response.Value;

        CosmosDBSqlContainerThroughputSettingResource applied = (await containerThroughput.GetAsync(cancellationToken)).Value;

        _logger.LogInformation(
            "Applied throughput settings: autoscaleMax={AutoscaleMaxThroughput}, manual={ManualThroughput}",
            applied.Data.Resource?.AutoscaleSettings?.MaxThroughput,
            applied.Data.Resource?.Throughput);
    }

    /// <summary>
    /// Creates or updates an Azure RBAC role assignment for the current user/principal.
    /// </summary>
    public async Task CreateOrUpdateAzureRoleAssignment(ResourceIdentifier roleDefinitionId, CancellationToken cancellationToken)
    {
        Guid? principalId = await GetCurrentUserPrincipalIdAsync(cancellationToken);

        string assignableScope = GetAssignableScope(Scope.Account);

        if (principalId is null)
        {
            throw new InvalidOperationException("Could not determine current user's principal (oid) from the access token.");
        }

        var scopeId = new ResourceIdentifier(assignableScope);
        RoleAssignmentCollection roleAssignments = _armClient.GetRoleAssignments(scopeId);

        string roleAssignmentName = Guid.NewGuid().ToString();

        RoleAssignmentCreateOrUpdateContent properties = new(roleDefinitionId, principalId.Value)
        {
            PrincipalType = RoleManagementPrincipalType.User,
            Description = "Role assignment for Cosmos DB"
        };

        ArmOperation<RoleAssignmentResource> response = await roleAssignments.CreateOrUpdateAsync(WaitUntil.Completed, roleAssignmentName, properties, cancellationToken);
        RoleAssignmentResource resource = response.Value;
        _logger.LogInformation("Created Azure RBAC role assignment: {RoleAssignmentId}", resource.Data.Id);
    }

    /// <summary>
    /// Retrieves the Azure built-in role definition for "Cosmos DB Operator".
    /// </summary>
    /// <remarks>
    /// This role allows management of the Cosmos DB account via ARM, but does not grant data plane permissions.
    /// This role also does not grant access to the keys or connection strings.
    /// </remarks>
    private Task<ResourceIdentifier> GetBuiltInCosmosDbOperatorRoleDefinition()
    {
        string roleDefinitionId = "230815da-be43-4aae-9cb4-875f7bd000aa";
        string roleDefinitionName = "Cosmos DB Operator";

        ResourceIdentifier resourceId = new($"/subscriptions/{_options.SubscriptionId}/providers/Microsoft.Authorization/roleDefinitions/{roleDefinitionId}");

        _logger.LogInformation("Azure built-in role: {RoleName} ({RoleId})", roleDefinitionName, roleDefinitionId);

        return Task.FromResult(resourceId);
    }

    /// <summary>
    /// Creates or updates a Cosmos DB NoSQL RBAC role assignment for the current user/principal.
    /// </summary>
    /// <remarks>
    /// Cosmos DB NoSQL RBAC role definitions are separate from Azure RBAC role definitions.
    /// - Azure RBAC (control plane) roles live under Microsoft.Authorization.
    /// - Cosmos DB NoSQL RBAC (data plane) roles live under Microsoft.DocumentDB and are scoped to the account.
    /// </remarks>
    public async Task CreateOrUpdateCosmosRoleAssignment(ResourceIdentifier roleDefinitionId, CancellationToken cancellationToken)
    {
        Guid? principalId = await GetCurrentUserPrincipalIdAsync(cancellationToken);

        // Cosmos DB SQL RBAC (data plane) assignments are created under the account, but the *permission scope*
        // can be narrowed to account / database / container for least-privilege.
        string assignableScope = GetAssignableScope(Scope.Account);

        CosmosDBSqlRoleAssignmentCreateOrUpdateContent properties = new()
        {
            RoleDefinitionId = roleDefinitionId,
            Scope = assignableScope,
            PrincipalId = principalId
        };

        string roleAssignmentId = Guid.NewGuid().ToString();
        ResourceIdentifier resourceId = CosmosDBSqlRoleAssignmentResource.CreateResourceIdentifier(_options.SubscriptionId, _options.ResourceGroupName, _options.AccountName, roleAssignmentId);
        CosmosDBSqlRoleAssignmentResource roleAssignment = _armClient.GetCosmosDBSqlRoleAssignmentResource(resourceId);

        ArmOperation<CosmosDBSqlRoleAssignmentResource> response = await roleAssignment.UpdateAsync(WaitUntil.Completed, properties, cancellationToken);
        CosmosDBSqlRoleAssignmentResource resource = response.Value;

        _logger.LogInformation("Created/updated Cosmos NoSQL RBAC role assignment: {RoleAssignmentId}", resource.Data.Id);
    }

    /// <summary>
    /// Retrieves the Cosmos DB NoSQL RBAC built-in role definition for "Cosmos DB Built-in Data Contributor".
    /// </summary>
    /// <remarks>
    /// This method prefers resolving by role name (more readable), but falls back to the well-known built-in
    /// role definition id if enumeration doesn't return a match.
    /// </remarks>
    private async Task<ResourceIdentifier> GetBuiltInDataContributorRoleDefinitionAsync(CancellationToken cancellationToken)
    {
        const string roleDefinitionName = "Cosmos DB Built-in Data Contributor";

        ResourceIdentifier accountId = CosmosDBAccountResource.CreateResourceIdentifier(_options.SubscriptionId, _options.ResourceGroupName, _options.AccountName);
        CosmosDBAccountResource account = _armClient.GetCosmosDBAccountResource(accountId);

        CosmosDBSqlRoleDefinitionCollection roleDefinitions = account.GetCosmosDBSqlRoleDefinitions();

        // Prefer name lookup so the sample is self-documenting.
        await foreach (CosmosDBSqlRoleDefinitionResource roleDefinition in roleDefinitions.GetAllAsync(cancellationToken))
        {
            if (string.Equals(roleDefinition.Data.RoleName, roleDefinitionName, StringComparison.OrdinalIgnoreCase))
            {
                return roleDefinition.Id;
            }
        }

        // Fallback: built-in Cosmos NoSQL RBAC role definition IDs are stable.
        // If listing doesn't return the role (permissions/filters/transient issues), fetch by the known id.
        string roleDefinitionId = "00000000-0000-0000-0000-000000000002";
        ResourceIdentifier roleDefinitionResourceId = CosmosDBSqlRoleDefinitionResource.CreateResourceIdentifier(_options.SubscriptionId, _options.ResourceGroupName, _options.AccountName, roleDefinitionId);
        CosmosDBSqlRoleDefinitionResource roleDefinitionById = await _armClient.GetCosmosDBSqlRoleDefinitionResource(roleDefinitionResourceId).GetAsync(cancellationToken);

        _logger.LogInformation(
            "Retrieved Cosmos NoSQL RBAC role definition from ARM: {RoleName}",
            roleDefinitionById.Data.RoleName);
    
        return roleDefinitionById.Id;
    }

    /// <summary>
    /// Creates or updates a custom Cosmos DB NoSQL RBAC role definition.
    /// </summary>
    /// <remarks>
    /// This sample shows how to create a custom role definition that excludes delete permissions.
    /// This method is not called by default; it's provided for demonstration purposes.
    /// </remarks>
    public async Task<ResourceIdentifier> CreateOrUpdateCustomCosmosDataRoleDefinition(CancellationToken cancellationToken)
    {
        // Custom Cosmos DB NoSQL RBAC role definitions also carry assignable scopes.
        // Keep this scoped (account/database/container) to reflect least-privilege options.
        string assignableScope = GetAssignableScope(Scope.Account);

        const string roleName = "My Custom Cosmos DB Data Role Except Delete";

        // Keep a stable id so rerunning the sample updates the same custom role definition
        // instead of creating duplicates.
        const string roleDefinitionId = "11111111-1111-1111-1111-111111111111";

        CosmosDBSqlRoleDefinitionCreateOrUpdateContent properties = new()
        {
            RoleName = roleName,
            RoleDefinitionType = CosmosDBSqlRoleDefinitionType.CustomRole,
            AssignableScopes = { assignableScope },
            Permissions =
            {
                new CosmosDBSqlRolePermission
                {
                    DataActions =
                    {
                        "Microsoft.DocumentDB/databaseAccounts/readMetadata",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert",
                        //"Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/delete", // Don't allow deletes
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts"
                    }
                }
            }
        };

        ResourceIdentifier resourceId = CosmosDBSqlRoleDefinitionResource.CreateResourceIdentifier(
            _options.SubscriptionId,
            _options.ResourceGroupName,
            _options.AccountName,
            roleDefinitionId);

        CosmosDBSqlRoleDefinitionResource roleDefinition = _armClient.GetCosmosDBSqlRoleDefinitionResource(resourceId);
        ArmOperation<CosmosDBSqlRoleDefinitionResource> response = await roleDefinition.UpdateAsync(WaitUntil.Completed, properties, cancellationToken);
        CosmosDBSqlRoleDefinitionResource resource = response.Value;

        _logger.LogInformation("Created/updated custom Cosmos NoSQL RBAC role definition: {RoleDefinitionId}", resource.Data.Id);
        return resource.Data.Id;
    }


    /// <summary>
    /// Produces an ARM-style scope string for use by:
    /// - Azure RBAC role assignments (control plane): subscription/resource-group/account scopes
    /// - Cosmos DB NoSQL RBAC role assignments (data plane): account/database/container scopes
    /// </summary>
    /// <remarks>
    /// Cosmos DB NoSQL RBAC role assignments are stored under the account resource, but the <c>Scope</c> on the
    /// assignment is what determines how broad the granted permissions are. Keeping database/container options
    /// enables least-privilege samples.
    /// </remarks>
    private string GetAssignableScope(Scope scope)
    {
        return scope switch
        {
            Scope.Subscription => $"/subscriptions/{_options.SubscriptionId}",
            Scope.ResourceGroup => $"/subscriptions/{_options.SubscriptionId}/resourceGroups/{_options.ResourceGroupName}",
            Scope.Account => $"/subscriptions/{_options.SubscriptionId}/resourceGroups/{_options.ResourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_options.AccountName}",
            Scope.Database => $"/subscriptions/{_options.SubscriptionId}/resourceGroups/{_options.ResourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_options.AccountName}/dbs/{_options.DatabaseName}",
            Scope.Container => $"/subscriptions/{_options.SubscriptionId}/resourceGroups/{_options.ResourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_options.AccountName}/dbs/{_options.DatabaseName}/colls/{_options.ContainerName}",
            _ => $"/subscriptions/{_options.SubscriptionId}/resourceGroups/{_options.ResourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_options.AccountName}"
        };
    }

    private enum Scope
    {
        // Azure RBAC (control plane) commonly uses these broader scopes.
        Subscription,
        ResourceGroup,

        // Cosmos DB NoSQL RBAC (data plane) permissions should generally be limited to one of these.
        Account,
        Database,
        Container
    }


    /// <summary>
    /// Attempts to extract the current user's email or UPN from the access token claims.
    /// </summary>
    private async Task<string?> GetCurrentUserEmailAsync(CancellationToken cancellationToken)
    {
        // Best-effort: for user identities, these claims are commonly populated.
        // For service principals / managed identities, email/UPN may not exist.
        var tokenRequestContext = new TokenRequestContext(new[] { "https://management.azure.com/.default" });
        AccessToken token = await _credential.GetTokenAsync(tokenRequestContext, cancellationToken);

        var handler = new JsonWebTokenHandler();
        var jwtToken = handler.ReadJsonWebToken(token.Token);

        if (jwtToken.TryGetValue("preferred_username", out string preferredUsername) && !string.IsNullOrWhiteSpace(preferredUsername))
        {
            return preferredUsername;
        }

        if (jwtToken.TryGetValue("upn", out string upn) && !string.IsNullOrWhiteSpace(upn))
        {
            return upn;
        }

        if (jwtToken.TryGetValue("unique_name", out string uniqueName) && !string.IsNullOrWhiteSpace(uniqueName))
        {
            return uniqueName;
        }

        _logger.LogInformation("Could not determine user email/UPN from access token claims; skipping owner tag value.");
        return null;
    }

    /// <summary>
    /// Attempts to extract the current user's principal id (oid) from the access token claims.
    /// </summary>
    private async Task<Guid?> GetCurrentUserPrincipalIdAsync(CancellationToken cancellationToken)
    {
        var tokenRequestContext = new TokenRequestContext(new[] { "https://management.azure.com/.default" });
        var token = await _credential.GetTokenAsync(tokenRequestContext, cancellationToken);

        var handler = new JsonWebTokenHandler();
        var jwtToken = handler.ReadJsonWebToken(token.Token);

        if (jwtToken.TryGetValue("oid", out string oid))
        {
            return new Guid(oid);
        }

        return null;
    }
}
