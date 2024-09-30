using Azure;
using Azure.Identity;
using Azure.Core;
using Azure.ResourceManager;
using Azure.ResourceManager.Resources;
using Azure.ResourceManager.CosmosDB;
using Azure.ResourceManager.CosmosDB.Models;
using System.Net;
using Microsoft.Graph;
using Microsoft.Graph.Models;
using Microsoft.Extensions.Configuration;
using System.IO;

public partial class Program
{
    
    private static string _subscriptionId = "your-subscription-id";
    private static string _resourceGroupName = "your-resource-group-name";
    private static string _accountName = "your-cosmosdb-account-name";
    private static string _location = "East US";
    private static string _databaseName = "your-database-name";
    private static string _containerName = "your-container-name";
    private static int _maxAutoScaleThroughput = 1000;

    
    private static readonly TokenCredential _credential = new DefaultAzureCredential();
    private static readonly ArmClient _armClient = new ArmClient(_credential);

    
    // Example usage of Azure Cosmos DB Management SDK for .NET
    public static async Task Main(string[] args)
    {
        LoadConfiguration();

        await InitializeSubscriptionAsync();
        await CreateOrUpdateCosmosDBAccount();
        await CreateOrUpdateCosmosDBDatabase();
        await CreateOrUpdateCosmosDBContainer();
        await UpdateThroughput(1000);
        await CreateOrUpdateRoleAssignment(await GetBuiltInDataContributorRoleDefinitionAsync());
        await CreateOrUpdateRoleAssignment(await CreateOrUpdateCustomRoleDefinition());

    }

    private static void LoadConfiguration()
    {
        var builder = new ConfigurationBuilder()
            .SetBasePath(Directory.GetCurrentDirectory())
            .AddJsonFile("appsettings.json", optional: false, reloadOnChange: true);

        IConfigurationRoot configuration = builder.Build();

        _subscriptionId = configuration["SubscriptionId"]!;
        _resourceGroupName = configuration["ResourceGroupName"]!;
        _accountName = configuration["AccountName"]!;
        _location = configuration["Location"]!;
        _databaseName = configuration["DatabaseName"]!;
        _containerName = configuration["ContainerName"]!;
        _maxAutoScaleThroughput = int.Parse(configuration["MaxAutoScaleThroughput"]!);
    }    

    private static async Task InitializeSubscriptionAsync()
    {
        
        // GetDefaultSubscriptionAsync() does not work correctly getting the default subscription

        // Get the default subscription
        SubscriptionResource _subscription = await _armClient.GetDefaultSubscriptionAsync();
        
        // Get the subscription Id
        //_subscriptionId = _subscription.Data.SubscriptionId;

        //Console.WriteLine($"Subscription ID: {_subscriptionId}");
    }

    private static async Task<string> GetLocalIpAddressAsync()
    {

        // Get the public IP address making outbound http requests
        string publicIpAddress;
        using (HttpClient httpClient = new())
        {
            publicIpAddress = await httpClient.GetStringAsync("https://api.ipify.org");
            Console.WriteLine($"Public IP Address: {publicIpAddress}");
        }

        return publicIpAddress;
    }

    public static async Task CreateOrUpdateCosmosDBAccount()
    {
        // Create a CosmosDB account
        CosmosDBAccountCreateOrUpdateContent properties = 
        new CosmosDBAccountCreateOrUpdateContent(
            new AzureLocation(_location),
            [
                new CosmosDBAccountLocation()
                {
                    LocationName = _location,
                    FailoverPriority = 0,
                    IsZoneRedundant = false
                }
            ]
        )
        {
            Kind = CosmosDBAccountKind.GlobalDocumentDB,
            Capabilities = 
            { 
                //new CosmosDBAccountCapability
                //{ 
                //    Name = "EnableServerless" //Remove container throughput when using serverless
                //},
                new CosmosDBAccountCapability
                {
                    Name = "EnableNoSQLVectorSearch"
                }
            },

            //When true, Entra ID and RBAC are required for AuthN/AuthZ
            DisableLocalAuth = true,

            //Networking properties can take up to 10 minutes to take effect
            PublicNetworkAccess = "Enabled",
            /*IPRules = 
            { 
                new CosmosDBIPAddressOrRange()
                {
                    IPAddressOrRange = "0.0.0.0"  //Allow access from Azure Portal and data centers
                },
                new CosmosDBIPAddressOrRange()
                {
                    IPAddressOrRange = await GetLocalIpAddress()  //Allow access from local machine
                }
            },*/
            
            Tags =
            {
                { "key1", "value1" },
                { "key2", "value2" }
            }
        };

        //Get the resource group
        ResourceIdentifier resourceId = ResourceGroupResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName);
        ResourceGroupResource resourceGroup = _armClient.GetResourceGroupResource(resourceId);
        
        //Get the CosmosDB accounts collection
        CosmosDBAccountCollection cosmosAccounts = resourceGroup.GetCosmosDBAccounts();

        //Create or update the CosmosDB account
        ArmOperation<CosmosDBAccountResource> response = await cosmosAccounts.CreateOrUpdateAsync(WaitUntil.Completed, _accountName, properties);
        CosmosDBAccountResource resource = response.Value;
        
        Console.WriteLine($"Created new Account: {resource.Data.Id}");
    }

    public static async Task CreateOrUpdateCosmosDBDatabase()
    {
        
        //Database properties
        CosmosDBSqlDatabaseCreateOrUpdateContent properties = 
            new CosmosDBSqlDatabaseCreateOrUpdateContent(
            _location,
            new CosmosDBSqlDatabaseResourceInfo(_databaseName));


        //Get the account
        ResourceIdentifier resourceId = CosmosDBAccountResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName, _accountName);
        CosmosDBAccountResource account = _armClient.GetCosmosDBAccountResource(resourceId);
        
        //Get the databases collection
        CosmosDBSqlDatabaseCollection databases = account.GetCosmosDBSqlDatabases();

        //Create or update the database
        ArmOperation<CosmosDBSqlDatabaseResource> response = await databases.CreateOrUpdateAsync(WaitUntil.Completed, _databaseName, properties);
        CosmosDBSqlDatabaseResource resource = response.Value;

        Console.WriteLine($"Created new Database: {resource.Data.Id}");
        
    }

    public static async Task CreateOrUpdateCosmosDBContainer()
    {    

        // Container properties
        CosmosDBSqlContainerCreateOrUpdateContent properties = 
        new CosmosDBSqlContainerCreateOrUpdateContent(
            _location,
            new CosmosDBSqlContainerResourceInfo(_containerName)
            {
                DefaultTtl = -1,
                PartitionKey = new CosmosDBContainerPartitionKey()
                {
                    Paths = { "/companyId", "/departmentId", "/userId" },
                    Kind = CosmosDBPartitionKind.MultiHash,  //Hash for single partition key, MultiHash for hierarchical partition key
                    Version = 2
                },
                IndexingPolicy = new CosmosDBIndexingPolicy()
                {
                    IsAutomatic = true,
                    IndexingMode = CosmosDBIndexingMode.Consistent,
                    IncludedPaths = 
                    {
                        new CosmosDBIncludedPath()
                        {
                            Path = "/*"
                        }
                    },
                    ExcludedPaths = 
                    {
                        new CosmosDBExcludedPath()
                        {
                            Path = "/\"_etag\"/?"
                        }
                    }
                },
                UniqueKeys = 
                {
                    new CosmosDBUniqueKey
                    {
                        Paths = 
                        { 
                            "/userId" 
                        }
                    }
                },
                ComputedProperties = 
                {
                    new ComputedProperty()
                    {
                        Name = "cp_lowerName",
                        Query = "SELECT VALUE LOWER(c.userName) FROM c"
                    }
                },
                ConflictResolutionPolicy = new ConflictResolutionPolicy()  //Only used when multi-region write is enabled
                {
                    Mode = ConflictResolutionMode.LastWriterWins,
                    ConflictResolutionPath = "/_ts"
                }
            })
            {
                Options = new CosmosDBCreateUpdateConfig() //Only use if not Serverless account
                {
                    AutoscaleMaxThroughput = _maxAutoScaleThroughput
                }
            };


        //Get the CosmosDB database
        ResourceIdentifier resourceId = CosmosDBSqlDatabaseResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName, _accountName, _databaseName);
        CosmosDBSqlDatabaseResource cosmosDBDatabase = _armClient.GetCosmosDBSqlDatabaseResource(resourceId);
        
        //Get the containers collection
        CosmosDBSqlContainerCollection cosmosContainers = cosmosDBDatabase.GetCosmosDBSqlContainers();

        //Create or update the container
        ArmOperation<CosmosDBSqlContainerResource> response = await cosmosContainers.CreateOrUpdateAsync(WaitUntil.Completed, _containerName, properties);
        CosmosDBSqlContainerResource resource = response.Value;

        Console.WriteLine($"Created new Container: {resource.Data.Id}");
    }

    public static async Task UpdateThroughput(int addThroughput)
    {
        
        // Update the throughput of the CosmosDB container
        ThroughputSettingsUpdateData throughput = new ThroughputSettingsUpdateData(
            new AzureLocation(_location),
            new ThroughputSettingsResourceInfo()
            {
                AutoscaleSettings = new AutoscaleSettingsResourceInfo(_maxAutoScaleThroughput + addThroughput)
            }
        );

        // Get the container throughput
        ResourceIdentifier resourceId = CosmosDBSqlContainerThroughputSettingResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName, _accountName, _databaseName, _containerName);
        CosmosDBSqlContainerThroughputSettingResource containerThroughput = _armClient.GetCosmosDBSqlContainerThroughputSettingResource(resourceId);

        // Update the throughput
        ArmOperation<CosmosDBSqlContainerThroughputSettingResource> response = await containerThroughput.CreateOrUpdateAsync(WaitUntil.Completed, throughput);
        CosmosDBSqlContainerThroughputSettingResource resource = response.Value;

        Console.WriteLine($"Updated Container throughput: {resource.Data.Id}");
    }

    public static async Task CreateOrUpdateRoleAssignment(ResourceIdentifier roleDefintionId)
    {

        //Get the principal ID of the current logged-in user
        Guid? principalId = await GetCurrentUserPrincipalIdAsync();

        //Select the type of role to assign
        //ResourceIdentifier roleDefintionId = await GetBuiltInDataContributorRoleDefinitionAsync();
        //ResourceIdentifier roleDefinitionId = await CreateOrUpdateCustomRoleDefinition();

        //Select the scope of the role permissions
        string assignableScope = GetAssignableScope(Scope.Account);

        //Role assignment properties
        CosmosDBSqlRoleAssignmentCreateOrUpdateContent properties = new CosmosDBSqlRoleAssignmentCreateOrUpdateContent()
        {
            RoleDefinitionId = roleDefintionId, 
            Scope = assignableScope,
            PrincipalId = principalId
        };

        //Construct a new role assignment resource
        string roleAssignmentId = Guid.NewGuid().ToString();
        ResourceIdentifier resourceId = CosmosDBSqlRoleAssignmentResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName, _accountName, roleAssignmentId);
        CosmosDBSqlRoleAssignmentResource roleAssignment = _armClient.GetCosmosDBSqlRoleAssignmentResource(resourceId);

        //Update the role assignment with the new properties
        ArmOperation<CosmosDBSqlRoleAssignmentResource> response = await roleAssignment.UpdateAsync(WaitUntil.Completed, properties);
        CosmosDBSqlRoleAssignmentResource resource = response.Value;

        Console.WriteLine($"Created new Role Assignment: {resource.Data.Id}");
    }
    
    private static async Task<ResourceIdentifier> GetBuiltInDataContributorRoleDefinitionAsync()
    {

        //Built-in roles are predefined roles that are available in Azure Cosmos DB
        //Cosmos DB Built-in Data Contributor role definition ID
        string roleDefinitionId = "00000000-0000-0000-0000-000000000002";

        //Get the role definition
        ResourceIdentifier resourceId = CosmosDBSqlRoleDefinitionResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName, _accountName, roleDefinitionId);
        CosmosDBSqlRoleDefinitionResource roleDefinition = await _armClient.GetCosmosDBSqlRoleDefinitionResource(resourceId).GetAsync();
        
        return roleDefinition.Id;
    }

    private static async Task<ResourceIdentifier> CreateOrUpdateCustomRoleDefinition()
    {
        //Create a custom role definition that does everything Data Contributor does, but doesn't allow deletes

        //Select the scope to assign the role to
        string assignableScope = GetAssignableScope(Scope.Account);

        //Custom role definition properties
        CosmosDBSqlRoleDefinitionCreateOrUpdateContent properties = new CosmosDBSqlRoleDefinitionCreateOrUpdateContent()
        {
            RoleName = "My Custom Cosmos DB Data Contributor Except Delete",
            RoleDefinitionType = CosmosDBSqlRoleDefinitionType.CustomRole,
            AssignableScopes = 
            { 
                assignableScope 
            },
            Permissions = 
            {
                new CosmosDBSqlRolePermission()
                {
                    DataActions = 
                    { 
                        "Microsoft.DocumentDB/databaseAccounts/readMetadata",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert",
                        //"Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/delete", //Don't allow deletes
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure",
                        "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts"
                    }
                }
            }
        };

        //Construct a new role definition resource
        string roleDefinitionId = Guid.NewGuid().ToString();
        ResourceIdentifier resourceId = CosmosDBSqlRoleDefinitionResource.CreateResourceIdentifier(_subscriptionId, _resourceGroupName, _accountName, roleDefinitionId);
        CosmosDBSqlRoleDefinitionResource newRoleDefinition = _armClient.GetCosmosDBSqlRoleDefinitionResource(resourceId);

        //Create or update the custom role definition
        ArmOperation<CosmosDBSqlRoleDefinitionResource> response = await newRoleDefinition.UpdateAsync(WaitUntil.Completed, properties);
        CosmosDBSqlRoleDefinitionResource resource = response.Value;

        Console.WriteLine($"Created new Role Definition: {resource.Data.Id}");

        return resource.Data.Id;

    }

    private static string GetAssignableScope(Scope scope)
    {
        // Switch statement to set the permission scope
        string scopeString = scope switch
        {
            Scope.Subscription => $"/subscriptions/{_subscriptionId}",
            Scope.ResourceGroup => $"/subscriptions/{_subscriptionId}/resourceGroups/{_resourceGroupName}",
            Scope.Account => $"/subscriptions/{_subscriptionId}/resourceGroups/{_resourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_accountName}",
            Scope.Database => $"/subscriptions/{_subscriptionId}/resourceGroups/{_resourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_accountName}/dbs/{_databaseName}",
            Scope.Container => $"/subscriptions/{_subscriptionId}/resourceGroups/{_resourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_accountName}/dbs/{_databaseName}/colls/{_containerName}",
            _ => $"/subscriptions/{_subscriptionId}/resourceGroups/{_resourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/{_accountName}",
        };
        return scopeString;
    }

    private static async Task<Guid?> GetCurrentUserPrincipalIdAsync()
    {
        
        // Get the principal Id of the current logged-in user
        GraphServiceClient graphClient = new(_credential);

        User? user = await graphClient.Me.GetAsync();
        
        if (user == null || user.Id == null)
            throw new InvalidOperationException("User or User ID is null.");
        
        Guid principalId = new(user.Id);

        return principalId;

    }

    private enum Scope
    {
        Subscription,
        ResourceGroup,
        Account,
        Database,
        Container
    }
}
