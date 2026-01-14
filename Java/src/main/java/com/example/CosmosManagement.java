package com.example;

import com.azure.core.credential.AccessToken;
import com.azure.core.credential.TokenCredential;
import com.azure.core.credential.TokenRequestContext;
import com.azure.core.management.AzureEnvironment;
import com.azure.core.management.Region;
import com.azure.core.management.profile.AzureProfile;
import com.azure.core.util.BinaryData;
import com.azure.core.exception.HttpResponseException;
import com.azure.identity.DefaultAzureCredentialBuilder;
import com.azure.resourcemanager.authorization.AuthorizationManager;
import com.azure.resourcemanager.authorization.models.RoleAssignment;
import com.azure.resourcemanager.cosmos.CosmosManager;
import com.azure.resourcemanager.cosmos.fluent.models.SqlRoleDefinitionGetResultsInner;
import com.azure.resourcemanager.cosmos.fluent.models.ThroughputSettingsGetResultsInner;
import com.azure.resourcemanager.cosmos.models.AutoscaleSettings;
import com.azure.resourcemanager.cosmos.models.AutoscaleSettingsResource;
import com.azure.resourcemanager.cosmos.models.ComputedProperty;
import com.azure.resourcemanager.cosmos.models.ConflictResolutionMode;
import com.azure.resourcemanager.cosmos.models.ConflictResolutionPolicy;
import com.azure.resourcemanager.cosmos.models.ContainerPartitionKey;
import com.azure.resourcemanager.cosmos.models.CreateUpdateOptions;
import com.azure.resourcemanager.cosmos.models.ExcludedPath;
import com.azure.resourcemanager.cosmos.models.IncludedPath;
import com.azure.resourcemanager.cosmos.models.IndexingMode;
import com.azure.resourcemanager.cosmos.models.IndexingPolicy;
import com.azure.resourcemanager.cosmos.models.SqlContainerCreateUpdateParameters;
import com.azure.resourcemanager.cosmos.models.SqlContainerResource;
import com.azure.resourcemanager.cosmos.models.SqlDatabaseCreateUpdateParameters;
import com.azure.resourcemanager.cosmos.models.SqlDatabaseResource;
import com.azure.resourcemanager.cosmos.models.SqlRoleAssignmentCreateUpdateParameters;
import com.azure.resourcemanager.cosmos.models.ThroughputSettingsResource;
import com.azure.resourcemanager.cosmos.models.ThroughputSettingsUpdateParameters;
import com.azure.resourcemanager.cosmos.models.UniqueKey;
import com.azure.resourcemanager.cosmos.models.UniqueKeyPolicy;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * Implements Cosmos DB management-plane operations for this sample.
 *
 * <p>This class is intentionally small and console-driven: it demonstrates the management SDK calls
 * while keeping the sample safe (no destructive actions unless explicitly confirmed).
 */
public final class CosmosManagement {
    private static final Logger logger = LoggerFactory.getLogger(CosmosManagement.class);

    private static final String MANAGEMENT_SCOPE = "https://management.azure.com/.default";
    private static final String COSMOS_DB_OPERATOR_ROLE_DEFINITION_ID = "230815da-be43-4aae-9cb4-875f7bd000aa";
    private static final String COSMOS_SQL_BUILT_IN_DATA_CONTRIBUTOR_ROLE_DEFINITION_ID = "00000000-0000-0000-0000-000000000002";

    private final CosmosConfig config;
    private final TokenCredential credential;
    private final CosmosManager cosmosManager;
    private final AuthorizationManager authorizationManager;

    private CosmosManagement(
        CosmosConfig config,
        TokenCredential credential,
        CosmosManager cosmosManager,
        AuthorizationManager authorizationManager) {

        this.config = config;
        this.credential = credential;
        this.cosmosManager = cosmosManager;
        this.authorizationManager = authorizationManager;
    }

    /**
     * Creates a fully-initialized sample instance (credential + CosmosManager client).
     */
    public static CosmosManagement create(CosmosConfig config) {
        String tenantId = System.getenv("AZURE_TENANT_ID");
        AzureProfile profile = new AzureProfile(tenantId, config.subscriptionId(), AzureEnvironment.AZURE);

        TokenCredential credential = new DefaultAzureCredentialBuilder()
            .authorityHost(profile.getEnvironment().getActiveDirectoryEndpoint())
            .build();

        CosmosManager cosmosManager = CosmosManager.authenticate(credential, profile);

        AuthorizationManager authorizationManager = AuthorizationManager.authenticate(credential, profile);
        return new CosmosManagement(config, credential, cosmosManager, authorizationManager);
    }

    /**
     * Runs the full end-to-end sample.
     */
    public void run() {
        createOrUpdateCosmosDBAccount();
        createOrUpdateAzureRoleAssignment();
        createOrUpdateCosmosDBDatabase();
        createOrUpdateCosmosDBContainer();
        updateThroughput(1000);
        createOrUpdateCosmosSqlRoleAssignment();

        if (Boolean.parseBoolean(System.getenv("COSMOS_SAMPLE_DELETE_ACCOUNT"))) {
            deleteCosmosDBAccount();
        }
    }

    /**
     * Creates or updates a Cosmos DB account (SQL API / NoSQL) in the configured resource group.
     */
    public void createOrUpdateCosmosDBAccount() {
        String resourceGroupName = config.resourceGroupName();
        String accountName = config.accountName();

        Region location = Region.fromName(config.location());

        logger.info("Creating/updating Cosmos DB account: {}/{}", resourceGroupName, accountName);

        cosmosManager
            .databaseAccounts()
            .define(accountName)
            .withRegion(location)
            .withExistingResourceGroup(resourceGroupName)
            .withDataModelSql()
            .withSessionConsistency()
            .withWriteReplication(location)
            .disableLocalAuth()
            .createAsync().block();

        logger.info("Created/updated Cosmos DB account: {}/{}", resourceGroupName, accountName);
    }

    /**
     * Creates an Azure RBAC role assignment at the Cosmos account scope.
     *
     * <p>This assigns the built-in 'Cosmos DB Operator' role which provides management-plane access
     * to Cosmos DB resources. But does not include key access or data-plane permissions.
     */
    public void createOrUpdateAzureRoleAssignment() {
        String principalObjectId = getCurrentPrincipalObjectId();
        if (principalObjectId == null || principalObjectId.isBlank()) {
            throw new IllegalStateException("Could not determine principal object id (oid) from access token.");
        }

        String scope = getAssignableScope(Scope.Account);

        String roleDefinitionResourceId = "/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s"
            .formatted(config.subscriptionId(), COSMOS_DB_OPERATOR_ROLE_DEFINITION_ID);

        // We generate a deterministic UUIDv5 (name-based) for the role assignment name so repeated
        // runs of the sample converge on the same role assignment instead of creating duplicates.
        //
        // UUIDv5 requires a namespace UUID + a name string. We derive the namespace from the current
        // subscription id (see getDeterministicNamespace()) so it is user/environment-defined, stable
        // for a given subscription, and different across subscriptions.
        UUID namespace = getDeterministicNamespace();

        String roleAssignmentName = uuid5(namespace, "%s|%s|%s".formatted(scope, roleDefinitionResourceId, principalObjectId))
            .toString();

        RoleAssignment roleAssignment = authorizationManager
            .roleAssignments()
            .define(roleAssignmentName)
            .forObjectId(principalObjectId)
            .withRoleDefinition(roleDefinitionResourceId)
            .withScope(scope)
            .create();

        logger.info("Created Azure RBAC role assignment: {}", roleAssignment.id());
    }

    /**
     * Creates or updates a NoSQL database under the Cosmos DB account.
     */
    public void createOrUpdateCosmosDBDatabase() {
        String resourceGroupName = config.resourceGroupName();
        String accountName = config.accountName();
        String databaseName = config.databaseName();

        logger.info("Creating/updating NoSQL database: {}", databaseName);

        SqlDatabaseResource sqlDatabaseResource = new SqlDatabaseResource().withId(databaseName);
        SqlDatabaseCreateUpdateParameters parameters = new SqlDatabaseCreateUpdateParameters().withResource(sqlDatabaseResource);

        cosmosManager.serviceClient()
            .getSqlResources()
            .createUpdateSqlDatabase(resourceGroupName, accountName, databaseName, parameters);

        logger.info("Created/updated NoSQL database: {}", databaseName);
    }

    /**
     * Creates or updates a NoSQL container under the configured NoSQL database.
     */
    public void createOrUpdateCosmosDBContainer() {
        String resourceGroupName = config.resourceGroupName();
        String accountName = config.accountName();
        String databaseName = config.databaseName();
        String containerName = config.containerName();

        logger.info("Creating/updating NoSQL container: {}", containerName);

        SqlContainerCreateUpdateParameters parameters = new SqlContainerCreateUpdateParameters();
        SqlContainerResource resource = new SqlContainerResource().withId(containerName);

        // Partition key
        resource.withPartitionKey(new ContainerPartitionKey().withPaths(List.of("/id")));

        // Indexing policy
        IndexingPolicy indexingPolicy = new IndexingPolicy()
            .withAutomatic(true)
            .withIndexingMode(IndexingMode.CONSISTENT)
            .withIncludedPaths(List.of(new IncludedPath().withPath("/*")))
            .withExcludedPaths(List.of(new ExcludedPath().withPath("/\"_etag\"/?")));
        resource.withIndexingPolicy(indexingPolicy);

        // Unique key policy
        UniqueKey uniqueKey = new UniqueKey().withPaths(List.of("/userName"));
        UniqueKeyPolicy uniqueKeyPolicy = new UniqueKeyPolicy().withUniqueKeys(List.of(uniqueKey));
        resource.withUniqueKeyPolicy(uniqueKeyPolicy);

        // Computed property
        ComputedProperty computedProperty = new ComputedProperty()
            .withName("cp_lowerName")
            .withQuery("SELECT VALUE LOWER(c.userName) FROM c");
        resource.withComputedProperties(List.of(computedProperty));

        // Conflict resolution policy (useful when multi-region writes are enabled)
        ConflictResolutionPolicy conflictResolutionPolicy = new ConflictResolutionPolicy()
            .withMode(ConflictResolutionMode.LAST_WRITER_WINS)
            .withConflictResolutionPath("/_ts");
        resource.withConflictResolutionPolicy(conflictResolutionPolicy);

        parameters.withResource(resource);

        // Autoscale settings
        CreateUpdateOptions options = new CreateUpdateOptions()
            .withAutoscaleSettings(new AutoscaleSettings().withMaxThroughput(config.maxAutoScaleThroughput()));
        parameters.withOptions(options);

        cosmosManager.serviceClient()
            .getSqlResources()
            .createUpdateSqlContainer(resourceGroupName, accountName, databaseName, containerName, parameters);

        logger.info("Created/updated NoSQL container: {}", containerName);
    }

    /**
     * Updates container throughput by reading the current settings first, then applying a delta.
     *
     * <p>If the container uses autoscale throughput, the delta is applied to autoscale <em>max</em> throughput.
     * If the container uses manual throughput, the delta is applied to the fixed RU/s.
     *
     * <p>If the container does not have a dedicated throughput resource, this throws a clear exception.
     * That typically means the container is using shared database throughput or the account is serverless.
     */
    public void updateThroughput(int delta) {
        String resourceGroupName = config.resourceGroupName();
        String accountName = config.accountName();
        String databaseName = config.databaseName();
        String containerName = config.containerName();

        logger.info("Reading current container throughput settings: {}/{}/{}/{}", resourceGroupName, accountName, databaseName, containerName);

        ThroughputSettingsGetResultsInner existing;
        try {
            existing = cosmosManager.serviceClient()
                .getSqlResources()
                .getSqlContainerThroughput(resourceGroupName, accountName, databaseName, containerName);
        } catch (HttpResponseException ex) {
            if (ex.getResponse() != null && ex.getResponse().getStatusCode() == 404) {
                throw new IllegalStateException(
                    "Container throughput settings were not found. This usually means the container uses shared database throughput or the account is serverless, so there is no dedicated container throughput resource to update.",
                    ex);
            }
            throw ex;
        }

        ThroughputSettingsResource existingResource = existing == null ? null : existing.resource();
        AutoscaleSettingsResource existingAutoscale = existingResource == null ? null : existingResource.autoscaleSettings();

        Integer currentAutoscaleMax = existingAutoscale == null ? null : existingAutoscale.maxThroughput();
        Integer currentManualThroughput = existingResource == null ? null : existingResource.throughput();

        if (currentAutoscaleMax == null && currentManualThroughput == null) {
            throw new IllegalStateException(
                "Container throughput settings did not include autoscale or manual throughput. The container likely uses shared database throughput or serverless, and therefore does not have a dedicated throughput resource to update.");
        }

        ThroughputSettingsUpdateParameters parameters;

        if (currentAutoscaleMax != null) {
            int baseline = currentAutoscaleMax == 0 ? config.maxAutoScaleThroughput() : currentAutoscaleMax;
            int newAutoscaleMax = Math.max(1000, Math.addExact(baseline, delta));

            logger.info("Updating container autoscale max throughput from {} to {}", currentAutoscaleMax, newAutoscaleMax);

            AutoscaleSettingsResource autoscaleSettings = new AutoscaleSettingsResource().withMaxThroughput(newAutoscaleMax);
            ThroughputSettingsResource throughputSettingsResource = new ThroughputSettingsResource().withAutoscaleSettings(autoscaleSettings);
            parameters = new ThroughputSettingsUpdateParameters().withResource(throughputSettingsResource);
        } else {
            int baseline = currentManualThroughput == null ? 0 : currentManualThroughput;
            int adjustedDelta = delta;
            if (baseline == 0) {
                // If the service didn't return a baseline, treat the input as the absolute target.
                baseline = delta;
                adjustedDelta = 0;
            }

            int newManualThroughput = Math.max(400, Math.addExact(baseline, adjustedDelta));

            logger.info("Updating container manual throughput from {} to {}", currentManualThroughput, newManualThroughput);

            ThroughputSettingsResource throughputSettingsResource = new ThroughputSettingsResource().withThroughput(newManualThroughput);
            parameters = new ThroughputSettingsUpdateParameters().withResource(throughputSettingsResource);
        }

        cosmosManager.serviceClient()
            .getSqlResources()
            .updateSqlContainerThroughput(resourceGroupName, accountName, databaseName, containerName, parameters);

        ThroughputSettingsGetResultsInner applied = cosmosManager.serviceClient()
            .getSqlResources()
            .getSqlContainerThroughput(resourceGroupName, accountName, databaseName, containerName);

        ThroughputSettingsResource appliedResource = applied == null ? null : applied.resource();
        Integer appliedAutoscaleMax = appliedResource == null || appliedResource.autoscaleSettings() == null
            ? null
            : appliedResource.autoscaleSettings().maxThroughput();
        Integer appliedManual = appliedResource == null ? null : appliedResource.throughput();

        logger.info("Applied throughput settings: autoscaleMax={}, manual={}", appliedAutoscaleMax, appliedManual);
    }

    /**
     * Creates a Cosmos DB NoSQL RBAC role assignment for the current principal.
     *
     * <p>In the C# sample this assigns the built-in 'Cosmos DB Built-in Data Contributor' role.
     */
    public void createOrUpdateCosmosSqlRoleAssignment() {
        String roleDefinitionResourceId = getBuiltInCosmosSqlDataContributorRoleDefinitionResourceId();
        String principalObjectId = getCurrentPrincipalObjectId();

        if (principalObjectId == null || principalObjectId.isBlank()) {
            throw new IllegalStateException("Could not determine principal object id (oid) from access token.");
        }

        String assignableScope = getAssignableScope(Scope.Account);

        // Same deterministic namespace as Azure RBAC role assignment IDs.
        UUID namespace = getDeterministicNamespace();

        SqlRoleAssignmentCreateUpdateParameters properties = new SqlRoleAssignmentCreateUpdateParameters()
            .withRoleDefinitionId(roleDefinitionResourceId)
            .withScope(assignableScope)
            .withPrincipalId(principalObjectId);

        String roleAssignmentId = uuid5(namespace, "%s|%s|%s".formatted(assignableScope, roleDefinitionResourceId, principalObjectId))
            .toString();

        cosmosManager.serviceClient()
            .getSqlResources()
            .createUpdateSqlRoleAssignment(roleAssignmentId, config.resourceGroupName(), config.accountName(), properties);

        logger.info("Created/updated Cosmos NoSQL RBAC role assignment: {}", roleAssignmentId);
    }

    
    /**
     * Deletes the configured Cosmos DB account.
     */
    public void deleteCosmosDBAccount() {
        cosmosManager.databaseAccounts().deleteByResourceGroup(config.resourceGroupName(), config.accountName());
        logger.info("Deleted Cosmos DB account: {}", config.accountName());
    }

    /**
     * Resolves the built-in Cosmos DB NoSQL RBAC role definition id for "Cosmos DB Built-in Data Contributor".
     *
     * <p>Cosmos DB NoSQL RBAC role assignments reference a role definition by its ARM resource id.
     * This method fetches the role definition resource so we can pass its {@code id()} into
     * {@link #createOrUpdateCosmosSqlRoleAssignment()}.
     */
    private String getBuiltInCosmosSqlDataContributorRoleDefinitionResourceId() {
        SqlRoleDefinitionGetResultsInner roleDefinition = cosmosManager.serviceClient()
            .getSqlResources()
            .getSqlRoleDefinition(COSMOS_SQL_BUILT_IN_DATA_CONTRIBUTOR_ROLE_DEFINITION_ID, config.resourceGroupName(), config.accountName());

        logger.info("Cosmos NoSQL RBAC built-in role definition: {}", roleDefinition.id());
        return roleDefinition.id();
    }

    /**
     * Returns a deterministic UUID namespace derived from the configured subscription id.
     *
     * <p>UUIDv5 requires a namespace UUID. Instead of using a fixed "magic" seed value, this sample
     * derives the namespace from the subscription id (which is already a UUID/GUID).
     *
     * <p>Why subscription id?
     * <ul>
     *   <li>It is stable and user/environment-defined.</li>
     *   <li>It makes generated UUIDs deterministic for the same subscription.</li>
     *   <li>It prevents accidental collisions across different subscriptions.</li>
     * </ul>
     *
     * <p>Note: the namespace is only the "seed" for UUIDv5. The actual uniqueness comes from the
     * name string we hash (for example: scope + role definition id + principal object id).
     */
    private UUID getDeterministicNamespace() {
        return UUID.fromString(config.subscriptionId());
    }

    /**
     * Attempts to extract the current principal's object id (OID) from an Azure AD access token.
     *
     * <p>This sample avoids calling Microsoft Graph by instead requesting a management-plane token
     * (ARM scope) and reading the {@code oid} claim from the JWT payload.
     *
     * @return the principal object id (OID) if present; otherwise {@code null}
     */
    private String getCurrentPrincipalObjectId() {
        try {
            TokenRequestContext tokenRequestContext = new TokenRequestContext().addScopes(MANAGEMENT_SCOPE);
            AccessToken token = credential.getToken(tokenRequestContext).block();
            if (token == null || token.getToken() == null || token.getToken().isBlank()) {
                return null;
            }

            Map<?, ?> claims = parseJwtClaims(token.getToken());
            Object oid = claims.get("oid");
            if (oid instanceof String oidStr && !oidStr.isBlank()) {
                return oidStr;
            }
            return null;
        } catch (Exception e) {
            logger.info("Could not extract principal oid from access token claims: {}", e.getMessage());
            return null;
        }
    }

    /**
     * Parses the JWT payload into a map of claims.
     *
     * <p>This performs a lightweight decode of the middle JWT segment (base64url JSON) and does not
     * validate signatures. It is only used to read non-sensitive claims (like {@code oid}) from a token
     * already issued by the current credential.
     */
    private static Map<?, ?> parseJwtClaims(String jwt) {
        String[] parts = jwt.split("\\.");
        if (parts.length < 2) {
            throw new IllegalArgumentException("Invalid JWT");
        }

        String payloadJson = new String(Base64.getUrlDecoder().decode(parts[1]), StandardCharsets.UTF_8);
        return BinaryData.fromString(payloadJson).toObject(Map.class);
    }

    /**
     * Generates a deterministic (name-based) UUIDv5.
     *
     * <p>This sample uses UUIDv5 to create stable, idempotent resource names (for example role assignment ids)
     * derived from a small set of inputs like scope + role definition id + principal id. That allows repeated
     * runs of the sample to converge on the same ARM resource id rather than creating duplicates.
     *
     * @param namespace UUID namespace (for example URL namespace).
     * @param name name to hash within the namespace.
     * @return UUIDv5 derived from {@code namespace} and {@code name}.
     */
    private static UUID uuid5(UUID namespace, String name) {
        try {
            MessageDigest sha1 = MessageDigest.getInstance("SHA-1");

            sha1.update(uuidToBytes(namespace));
            sha1.update(name.getBytes(StandardCharsets.UTF_8));

            byte[] hash = sha1.digest();

            // Set version to 5 (name-based SHA-1) and variant to IETF.
            hash[6] = (byte) ((hash[6] & 0x0F) | 0x50);
            hash[8] = (byte) ((hash[8] & 0x3F) | 0x80);

            long msb = 0;
            long lsb = 0;
            for (int i = 0; i < 8; i++) {
                msb = (msb << 8) | (hash[i] & 0xFF);
            }
            for (int i = 8; i < 16; i++) {
                lsb = (lsb << 8) | (hash[i] & 0xFF);
            }
            return new UUID(msb, lsb);
        } catch (Exception e) {
            throw new IllegalStateException("Could not generate UUIDv5.", e);
        }
    }

    /**
     * Converts a UUID to a 16-byte big-endian representation.
     *
     * <p>Used as the first input to the UUIDv5 hash (namespace UUID bytes + name bytes).
     */
    private static byte[] uuidToBytes(UUID uuid) {
        byte[] bytes = new byte[16];
        long msb = uuid.getMostSignificantBits();
        long lsb = uuid.getLeastSignificantBits();

        for (int i = 0; i < 8; i++) {
            bytes[i] = (byte) (msb >>> (8 * (7 - i)));
        }
        for (int i = 8; i < 16; i++) {
            bytes[i] = (byte) (lsb >>> (8 * (15 - i)));
        }

        return bytes;
    }

    /**
     * Builds the fully-qualified Azure resource scope string for use with role assignments.
     *
     * <p>Azure RBAC (control plane) commonly uses broader scopes such as subscription, resource group, or
     * the Cosmos account. Cosmos DB NoSQL RBAC (data plane) assignments are created under the account
     * resource, but the assignment's {@code scope} can be narrowed to account / database / container for
     * least-privilege.
     *
     * @param scope desired scope level
     * @return ARM-style scope string (for example {@code /subscriptions/{subId}/...})
     */
    private String getAssignableScope(Scope scope) {
        return switch (scope) {
            case Subscription -> "/subscriptions/%s".formatted(config.subscriptionId());
            case ResourceGroup -> "/subscriptions/%s/resourceGroups/%s".formatted(config.subscriptionId(), config.resourceGroupName());
            case Account -> "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s"
                .formatted(config.subscriptionId(), config.resourceGroupName(), config.accountName());
            case Database -> "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s/dbs/%s"
                .formatted(config.subscriptionId(), config.resourceGroupName(), config.accountName(), config.databaseName());
            case Container -> "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s/dbs/%s/colls/%s"
                .formatted(config.subscriptionId(), config.resourceGroupName(), config.accountName(), config.databaseName(), config.containerName());
        };
    }

    private enum Scope {
        Subscription,
        ResourceGroup,
        Account,
        Database,
        Container
    }
}
