package com.example;

import com.azure.core.credential.TokenCredential;
import com.azure.core.management.AzureEnvironment;
import com.azure.core.management.Region;
import com.azure.core.management.profile.AzureProfile;
import com.azure.identity.DefaultAzureCredentialBuilder;
import com.azure.resourcemanager.cosmos.CosmosManager;
import com.azure.resourcemanager.cosmos.fluent.models.SqlRoleDefinitionGetResultsInner;
import com.azure.resourcemanager.cosmos.models.AutoscaleSettings;
import com.azure.resourcemanager.cosmos.models.AutoscaleSettingsResource;
import com.azure.resourcemanager.cosmos.models.ComputedProperty;
import com.azure.resourcemanager.cosmos.models.ConflictResolutionMode;
import com.azure.resourcemanager.cosmos.models.ConflictResolutionPolicy;
import com.azure.resourcemanager.cosmos.models.ContainerPartitionKey;
import com.azure.resourcemanager.cosmos.models.CosmosDBAccount;
import com.azure.resourcemanager.cosmos.models.CreateUpdateOptions;
import com.azure.resourcemanager.cosmos.models.ExcludedPath;
import com.azure.resourcemanager.cosmos.models.IncludedPath;
import com.azure.resourcemanager.cosmos.models.IndexingMode;
import com.azure.resourcemanager.cosmos.models.IndexingPolicy;
import com.azure.resourcemanager.cosmos.models.Location;
import com.azure.resourcemanager.cosmos.models.SqlContainerCreateUpdateParameters;
import com.azure.resourcemanager.cosmos.models.SqlDatabaseCreateUpdateParameters;
import com.azure.resourcemanager.cosmos.models.SqlDatabaseResource;
import com.azure.resourcemanager.cosmos.models.SqlContainerResource;
import com.azure.resourcemanager.cosmos.models.SqlRoleDefinitionCreateUpdateParameters;
import com.azure.resourcemanager.cosmos.models.ThroughputSettingsResource;
import com.azure.resourcemanager.cosmos.models.ThroughputSettingsUpdateParameters;
import com.azure.resourcemanager.cosmos.models.UniqueKey;
import com.azure.resourcemanager.cosmos.models.UniqueKeyPolicy;
import com.azure.resourcemanager.cosmos.models.SqlRoleAssignmentCreateUpdateParameters;
import com.microsoft.graph.authentication.TokenCredentialAuthProvider;
import com.microsoft.graph.models.User;
import com.microsoft.graph.requests.GraphServiceClient;
import okhttp3.Request;
import java.util.List;
import java.util.UUID;
import java.util.stream.Collectors;

public class CosmosDBManagement {
    private static String resourceGroupName = System.getenv("AZURE_RESOURCE_GROUP");
    private static String tenantId = System.getenv("AZURE_TENANT_ID");
    private static String subscriptionId = System.getenv("AZURE_SUBSCRIPTION_ID");
    private static String accountName = "<your account name>";
    private static String databaseName = "database";
    private static String containerName = "container";
    private static int maxAutoScaleThroughput = 1000;
    private static int maxAutoScaleThroughputUpdate = 4000;
    private static Region region = Region.US_EAST;
    private static Region writeRegion = Region.US_WEST;
    private static Region readRegion = Region.US_CENTRAL;

    public static CosmosManager cosmosManager;
    
    public static void main(String[] args) {
        try {
            TokenCredential credential = authenticate();
            cosmosManager = CosmosManager.authenticate(credential, createAzureProfile());
            
            createCosmosDBAccount(cosmosManager);
            createDatabase(cosmosManager);
            createContainer(cosmosManager);
            updateThroughput(cosmosManager, 4000);
            createOrUpdateRoleAssignment(getBuiltInDataContributorRoleDefinition());
            getCosmosDbAccount(cosmosManager);

            System.out.println("Cosmos DB resources created successfully.");
        } catch (Exception e) {
            System.err.println("An error occurred: " + e.getMessage());
        }
    }

    private static TokenCredential authenticate() {
        AzureProfile profile = createAzureProfile();
        return new DefaultAzureCredentialBuilder()
                .authorityHost(profile.getEnvironment().getActiveDirectoryEndpoint())
                .build();
    }

    private static AzureProfile createAzureProfile() {
        return new AzureProfile(tenantId, subscriptionId, AzureEnvironment.AZURE);
    }

    private static void createCosmosDBAccount(CosmosManager cosmosManager) {
        cosmosManager
            .databaseAccounts()
            .define(accountName)
            .withRegion(region)
            .withNewResourceGroup(resourceGroupName)
            .withDataModelSql()
            .withEventualConsistency()
            .withWriteReplication(writeRegion)
            .withReadReplication(readRegion)
            .withMultipleWriteLocationsEnabled(true)
            .createAsync().block();
        System.out.println("Cosmos DB account created: " + accountName);
    }

    private static void createDatabase(CosmosManager cosmosManager) {
        SqlDatabaseResource sqlDatabaseResource = new SqlDatabaseResource();
        sqlDatabaseResource.withId(databaseName);
        SqlDatabaseCreateUpdateParameters sqlDatabaseCreateUpdateParameters = new SqlDatabaseCreateUpdateParameters();
        sqlDatabaseCreateUpdateParameters.withResource(sqlDatabaseResource);
        cosmosManager.serviceClient().getSqlResources().createUpdateSqlDatabase(resourceGroupName, accountName, databaseName, sqlDatabaseCreateUpdateParameters);
        System.out.println("SQL Database created: " + databaseName);
    }

    private static void createContainer(CosmosManager cosmosManager) {

        // set up update options
        SqlContainerCreateUpdateParameters sqlContainerCreateUpdateParameters = new SqlContainerCreateUpdateParameters();
        SqlContainerResource sqlContainerResource = new SqlContainerResource();
        sqlContainerResource.withId(containerName);

        // Partition key
        sqlContainerResource.withPartitionKey(new ContainerPartitionKey().withPaths(List.of("/id")));

        // Indexing policy
        IndexingPolicy indexingPolicy = new IndexingPolicy();
        indexingPolicy.withAutomatic(true);
        indexingPolicy.withIndexingMode(IndexingMode.CONSISTENT);
        indexingPolicy.withIncludedPaths(List.of(new IncludedPath().withPath("/*")));
        indexingPolicy.withExcludedPaths(List.of(new ExcludedPath().withPath("/\"_etag\"/?")));
        sqlContainerResource.withIndexingPolicy(indexingPolicy);

        // Unique key policy
        UniqueKey uniqueKey = new UniqueKey().withPaths(List.of("/userName"));
        UniqueKeyPolicy uniqueKeyPolicy = new UniqueKeyPolicy().withUniqueKeys(List.of(uniqueKey));
        sqlContainerResource.withUniqueKeyPolicy(uniqueKeyPolicy);

        // Computed property
        ComputedProperty computedProperty = new ComputedProperty().withName("myComputedProperty").withName("cp_lowerName").withQuery("SELECT VALUE LOWER(c.userName) FROM c");
        sqlContainerResource.withComputedProperties(List.of(computedProperty));

        // Conflict resolution policy
        ConflictResolutionPolicy conflictResolutionPolicy = new ConflictResolutionPolicy().withMode(ConflictResolutionMode.LAST_WRITER_WINS).withConflictResolutionPath("/_ts");
        sqlContainerResource.withConflictResolutionPolicy(conflictResolutionPolicy);

        // apply policies to the container
        sqlContainerCreateUpdateParameters.withResource(sqlContainerResource);

        // Add autoscale settings
        CreateUpdateOptions options = new CreateUpdateOptions();
        options.withAutoscaleSettings(new AutoscaleSettings().withMaxThroughput(maxAutoScaleThroughput));
        sqlContainerCreateUpdateParameters.withOptions(options);

        // Create the container
        cosmosManager.serviceClient().getSqlResources().createUpdateSqlContainer(resourceGroupName, accountName, databaseName, containerName, sqlContainerCreateUpdateParameters);
        System.out.println("SQL Container created: " + containerName);
    }

    private static void updateThroughput(CosmosManager cosmosManager, int maxAutoScaleThroughput) {
        ThroughputSettingsUpdateParameters throughputSettingsUpdateParameters = new ThroughputSettingsUpdateParameters();
        ThroughputSettingsResource throughputSettingsResource = new ThroughputSettingsResource();
        AutoscaleSettingsResource autoscaleSettings = new AutoscaleSettingsResource();
        autoscaleSettings.withMaxThroughput(maxAutoScaleThroughputUpdate);
        throughputSettingsResource.withAutoscaleSettings(autoscaleSettings);
        throughputSettingsUpdateParameters.withResource(throughputSettingsResource);
        cosmosManager.serviceClient().getSqlResources().updateSqlContainerThroughput(resourceGroupName, accountName, databaseName, containerName, throughputSettingsUpdateParameters);
        System.out.println("SQL Container throughput updated to: " + maxAutoScaleThroughputUpdate);
    }

    private static void createOrUpdateRoleAssignment(String roleDefinitionId) {
        try {

            var credential = new DefaultAzureCredentialBuilder().build();
            // Get the principal ID of the current logged-in user
            String principalId = String.valueOf(getCurrentUserPrincipalIdAsync(credential));


            // Select the scope of the role permissions
            String assignableScope = getAssignableScope(Scope.Account);

            // Role assignment properties
            SqlRoleAssignmentCreateUpdateParameters properties = new SqlRoleAssignmentCreateUpdateParameters()
                    .withRoleDefinitionId(roleDefinitionId)
                    .withScope(assignableScope)
                    .withPrincipalId(principalId);

            // Generate a unique role assignment ID
            String roleAssignmentId = UUID.randomUUID().toString();

            System.out.println("Role Assignment ID: " + roleAssignmentId);

            // Create or update the role assignment
            cosmosManager.serviceClient().getSqlResources().createUpdateSqlRoleAssignment(roleAssignmentId,resourceGroupName, accountName, properties);

            // Get the new update role assignment ID
            roleAssignmentId = cosmosManager.serviceClient().getSqlResources().getSqlRoleAssignment(roleAssignmentId,resourceGroupName, accountName).id();

            // Print the created role assignment ID
            System.out.println("Created new Role Assignment: " + roleAssignmentId);
        } catch (Exception e) {
            System.err.println("An error occurred while creating/updating the role assignment: " + e.getMessage());
        }
    }

    private static String getAssignableScope(Scope scope) {
        String scopeString;
        switch (scope) {
            case Subscription:
                scopeString = String.format("/subscriptions/%s", subscriptionId);
                break;
            case ResourceGroup:
                scopeString = String.format("/subscriptions/%s/resourceGroups/%s", subscriptionId, resourceGroupName);
                break;
            case Account:
                scopeString = String.format("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s",
                        subscriptionId, resourceGroupName, accountName);
                break;
            case Database:
                scopeString = String.format("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s/dbs/%s",
                        subscriptionId, resourceGroupName, accountName, databaseName);
                break;
            case Container:
                scopeString = String.format("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s/dbs/%s/colls/%s",
                        subscriptionId, resourceGroupName, accountName, databaseName, containerName);
                break;
            default:
                scopeString = String.format("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s",
                        subscriptionId, resourceGroupName, accountName);
                break;
        }
        return scopeString;
    }

    private enum Scope {
        Subscription,
        ResourceGroup,
        Account,
        Database,
        Container
    }

    private static String getAssignableScope() {
        return "/subscriptions/" + subscriptionId + "/resourceGroups/" + resourceGroupName + "/providers/Microsoft.DocumentDB/databaseAccounts/" + accountName;
    }

    private static String getBuiltInDataContributorRoleDefinition() {
            // Built-in role definition ID for Data Contributor
            String roleDefinitionId = "00000000-0000-0000-0000-000000000002";

            // Retrieve the role definition resource using the constructed ID
            SqlRoleDefinitionGetResultsInner roleDefinition = cosmosManager.serviceClient().getSqlResources().getSqlRoleDefinition(roleDefinitionId, resourceGroupName, accountName);

            return roleDefinition.id();
    }

    private static String createOrUpdateCustomRoleDefinition() {
        String scope = getAssignableScope(Scope.Account);
        SqlRoleDefinitionCreateUpdateParameters properties = new SqlRoleDefinitionCreateUpdateParameters();
        List<com.azure.resourcemanager.cosmos.models.Permission> permissions = List.of();
        com.azure.resourcemanager.cosmos.models.Permission permission = new com.azure.resourcemanager.cosmos.models.Permission();
        List<String> dataActions = new java.util.ArrayList<>(List.of());
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/readMetadata");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure");
        dataActions.add("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts");
        permission.withDataActions(dataActions);
        properties.withAssignableScopes(List.of(scope));
        properties.withRoleName("My Custom Cosmos DB Permissions");
        //permissions.add(permission);
        properties.withPermissions(List.of(permission));
        String roleDefinitionId = UUID.randomUUID().toString();
        // create or update the role definition
        SqlRoleDefinitionGetResultsInner roleDefinition = cosmosManager.serviceClient().getSqlResources().createUpdateSqlRoleDefinition(roleDefinitionId, resourceGroupName, accountName, properties);
        System.out.println("Created new Custom Role Definition: " + roleDefinition.id());

        return roleDefinitionId;
    }


    private static String getCurrentUserPrincipalIdAsync(TokenCredential credential) {
        try {
            // Create a credential using DefaultAzureCredential
            credential = new DefaultAzureCredentialBuilder().build();

            // Create a GraphServiceClient using the credential
            TokenCredentialAuthProvider authProvider = new TokenCredentialAuthProvider(credential);
            GraphServiceClient<Request> graphClient = GraphServiceClient
                    .builder()
                    .authenticationProvider(authProvider)
                    .buildClient();

            // Get the currently signed-in user
            User user = graphClient.me()
                    .buildRequest()
                    .get();

            // Return the principal ID (user ID)
            if (user == null || user.id == null) {
                throw new IllegalArgumentException("User or User ID is null.");
            }
            return user.id;

        } catch (Exception e) {
            System.err.println("An error occurred while fetching the principal ID: " + e.getMessage());
            return null;
        }
    }

    private static void getCosmosDbAccount(CosmosManager cosmosManager) {
        CosmosDBAccount account = cosmosManager.databaseAccounts()
                .getByResourceGroup(resourceGroupName, accountName);

        System.out.println("Cosmos DB Account Details:");
        System.out.println("Account Name: " + account.name());
        System.out.println("Region: " + account.regionName());
        System.out.println("Kind: " + account.kind());
        System.out.println("Consistency Policy: " + account.consistencyPolicy().defaultConsistencyLevel());
        System.out.println("Readable locations: " + account.readableReplications().stream().map(Location::locationName).collect(Collectors.joining(",")));
        System.out.println("Writable locations: " + account.writableReplications().stream().map(Location::locationName).collect(Collectors.joining(",")));
    }
}
