import { v5 as uuidv5 } from "uuid";

import { getCurrentPrincipalObjectId, getCurrentUserEmailBestEffort } from "./tokenClaims.js";

function getAssignableScope(config, scope) {
  const base = `/subscriptions/${config.subscriptionId}/resourceGroups/${config.resourceGroupName}/providers/Microsoft.DocumentDB/databaseAccounts/${config.accountName}`;
  switch (scope) {
    case "ResourceGroup":
      return `/subscriptions/${config.subscriptionId}/resourceGroups/${config.resourceGroupName}`;
    case "Account":
      return base;
    case "Database":
      return `${base}/dbs/${config.databaseName}`;
    case "Container":
      return `${base}/dbs/${config.databaseName}/colls/${config.containerName}`;
    default:
      return base;
  }
}

function getStableGuid(name) {
  return uuidv5(name, uuidv5.URL);
}

/**
 * Create or update a Cosmos DB account (control plane).
 *
 * Configures:
 * - `disableLocalAuth: true` to require Entra ID + RBAC
 * - Enables the `EnableNoSQLVectorSearch` capability
 * - Best-effort `owner` tag from the current identity
 *
 * @param {{
 *   credential: import('@azure/core-auth').TokenCredential,
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   resourceGroupName: string,
 *   accountName: string,
 *   location: string
 * }} config
 */
export async function createOrUpdateCosmosDbAccount(clients, config) {
  console.log(
    `Creating/updating Cosmos DB account (this can take a couple minutes): ${config.resourceGroupName}/${config.accountName}`,
  );

  const ownerEmail = await getCurrentUserEmailBestEffort(clients.credential);

  // Capabilities are additive. Uncomment EnableServerless to experiment with serverless accounts.
  const capabilities = [{ name: "EnableNoSQLVectorSearch" }];
  // capabilities.push({ name: "EnableServerless" });

  const parameters = {
    location: config.location,
    kind: "GlobalDocumentDB",
    locations: [
      {
        locationName: config.location,
        failoverPriority: 0,
        isZoneRedundant: false,
      },
    ],
    databaseAccountOfferType: "Standard",
    capabilities,
    disableLocalAuth: true,
    tags: {
      owner: ownerEmail ?? "",
    },
  };

  const account = await clients.cosmosClient.databaseAccounts.beginCreateOrUpdateAndWait(
    config.resourceGroupName,
    config.accountName,
    parameters,
  );

  console.log(`Created/updated Cosmos DB account: ${account?.id ?? "(no id returned)"}`);
  return account;
}

/**
 * Delete a Cosmos DB account (control plane).
 *
 * WARNING: This is irreversible and deletes all data.
 *
 * @param {{
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   resourceGroupName: string,
 *   accountName: string
 * }} config
 */
export async function deleteCosmosDbAccount(clients, config) {
  console.log(
    `Deleting Cosmos DB account (this can take a couple minutes): ${config.resourceGroupName}/${config.accountName}`,
  );

  await clients.cosmosClient.databaseAccounts.beginDeleteAndWait(
    config.resourceGroupName,
    config.accountName,
  );

  console.log(`Deleted Cosmos DB account: ${config.resourceGroupName}/${config.accountName}`);
}

/**
 * Create or update an Azure RBAC role assignment (control plane).
 *
 * Grants ARM permissions to manage the Cosmos DB account resource.
 * This does NOT grant access to Cosmos DB data.
 *
 * Uses the Azure built-in role "Cosmos DB Operator".
 *
 * @param {{
 *   credential: import('@azure/core-auth').TokenCredential,
 *   authorizationClient: import('@azure/arm-authorization').AuthorizationManagementClient
 * }} clients
 * @param {{
 *   subscriptionId: string,
 *   resourceGroupName: string,
 *   accountName: string,
 *   databaseName: string,
 *   containerName: string
 * }} config
 */
export async function createOrUpdateAzureRbacCosmosDbOperatorAssignment(clients, config) {
  console.log("Creating Azure RBAC role assignment (Cosmos DB Operator)...");

  const principalId = await getCurrentPrincipalObjectId(clients.credential);
  const scope = getAssignableScope(config, "Account");

  // Azure built-in role definition: Cosmos DB Operator
  const roleDefinitionGuid = "230815da-be43-4aae-9cb4-875f7bd000aa";
  const roleDefinitionId = `/subscriptions/${config.subscriptionId}/providers/Microsoft.Authorization/roleDefinitions/${roleDefinitionGuid}`;

  // Deterministic GUID so repeated runs are idempotent.
  const roleAssignmentName = getStableGuid(`${scope}|${roleDefinitionId}|${principalId}`);

  const assignment = await clients.authorizationClient.roleAssignments.create(scope, roleAssignmentName, {
    roleDefinitionId,
    principalId,
    description: "Role assignment for Cosmos DB (management plane)",
  });

  console.log(`Created Azure RBAC role assignment: ${assignment?.id ?? "(no id returned)"}`);
  return assignment;
}

/**
 * Create or update a Cosmos DB NoSQL RBAC role assignment (data plane).
 *
 * Grants permissions to work with databases/containers/items.
 * Uses the built-in Cosmos DB NoSQL RBAC role "Cosmos DB Built-in Data Contributor".
 *
 * @param {{
 *   credential: import('@azure/core-auth').TokenCredential,
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   subscriptionId: string,
 *   resourceGroupName: string,
 *   accountName: string,
 *   databaseName: string,
 *   containerName: string
 * }} config
 */
export async function createOrUpdateCosmosNoSqlRbacDataContributorAssignment(clients, config) {
  console.log("Creating Cosmos NoSQL RBAC role assignment (Built-in Data Contributor)...");

  const principalId = await getCurrentPrincipalObjectId(clients.credential);
  const assignableScope = getAssignableScope(config, "Account");

  // Cosmos DB SQL RBAC built-in role definition ID.
  const roleDefinitionGuid = "00000000-0000-0000-0000-000000000002";
  const roleDefinition = await clients.cosmosClient.sqlResources.getSqlRoleDefinition(
    roleDefinitionGuid,
    config.resourceGroupName,
    config.accountName,
  );
  const roleDefinitionId = roleDefinition?.id;
  if (!roleDefinitionId) {
    throw new Error("Could not resolve Cosmos NoSQL RBAC built-in role definition id.");
  }

  // Deterministic GUID so repeated runs are idempotent.
  const roleAssignmentId = getStableGuid(`${assignableScope}|${roleDefinitionId}|${principalId}`);

  const assignment = await clients.cosmosClient.sqlResources.beginCreateUpdateSqlRoleAssignmentAndWait(
    roleAssignmentId,
    config.resourceGroupName,
    config.accountName,
    {
      roleDefinitionId,
      principalId,
      scope: assignableScope,
    },
  );

  console.log(`Created/updated Cosmos NoSQL RBAC role assignment: ${assignment?.id ?? "(no id returned)"}`);
  return assignment;
}

/**
 * Create or update a *custom* Cosmos DB NoSQL RBAC role definition.
 *
 * Example only: similar to the built-in Data Contributor, but excludes delete permissions.
 * This helper is not called by default.
 *
 * @param {{
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   subscriptionId: string,
 *   resourceGroupName: string,
 *   accountName: string,
 *   databaseName: string,
 *   containerName: string
 * }} config
 */
export async function createOrUpdateCustomCosmosNoSqlRbacRoleDefinitionExceptDelete(clients, config) {
  console.log("Creating/updating custom Cosmos NoSQL RBAC role definition (except delete)...");

  const assignableScope = getAssignableScope(config, "Account");

  const roleName = "My Custom Cosmos DB Data Role Except Delete";

  // Keep a stable ID so rerunning the sample updates the same role definition.
  const roleDefinitionId = "11111111-1111-1111-1111-111111111111";

  const roleDefinition = await clients.cosmosClient.sqlResources.beginCreateUpdateSqlRoleDefinitionAndWait(
    roleDefinitionId,
    config.resourceGroupName,
    config.accountName,
    {
      roleName,
      type: "CustomRole",
      assignableScopes: [assignableScope],
      permissions: [
        {
          dataActions: [
            "Microsoft.DocumentDB/databaseAccounts/readMetadata",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert",
            // "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/delete", // Don't allow deletes
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure",
            "Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts",
          ],
        },
      ],
    },
  );

  console.log(
    `Created/updated custom Cosmos NoSQL RBAC role definition: ${roleDefinition?.id ?? "(no id returned)"}`,
  );
  return roleDefinition;
}

/**
 * Create or update a Cosmos DB NoSQL database (control plane).
 *
 * @param {{
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   resourceGroupName: string,
 *   accountName: string,
 *   location: string,
 *   databaseName: string
 * }} config
 */
export async function createOrUpdateSqlDatabase(clients, config) {
  console.log(
    `Creating/updating SQL database: ${config.resourceGroupName}/${config.accountName}/${config.databaseName}`,
  );

  const parameters = {
    location: config.location,
    resource: {
      id: config.databaseName,
    },
  };

  const database = await clients.cosmosClient.sqlResources.beginCreateUpdateSqlDatabaseAndWait(
    config.resourceGroupName,
    config.accountName,
    config.databaseName,
    parameters,
  );

  console.log(`Created/updated SQL database: ${database?.id ?? "(no id returned)"}`);
  return database;
}

/**
 * Create or update a Cosmos DB SQL container (control plane).
 *
 * Demonstrates:
 * - Multi-hash partition keys
 * - Indexing policy (including vector indexes)
 * - Unique keys
 * - Computed properties
 * - Conflict resolution policy (useful for multi-region writes)
 * - Vector embedding policy
 * - Autoscale throughput settings
 *
 * @param {{
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   resourceGroupName: string,
 *   accountName: string,
 *   location: string,
 *   databaseName: string,
 *   containerName: string,
 *   maxAutoscaleThroughput: number
 * }} config
 */
export async function createOrUpdateSqlContainer(clients, config) {
  console.log(
    `Creating/updating SQL container (this can take a couple minutes): ${config.resourceGroupName}/${config.accountName}/${config.databaseName}/${config.containerName}`,
  );

  const parameters = {
    location: config.location,
    resource: {
      id: config.containerName,
      defaultTtl: -1,
      partitionKey: {
        paths: ["/companyId", "/departmentId", "/userId"],
        kind: "MultiHash",
        version: 2,
      },
      indexingPolicy: {
        automatic: true,
        indexingMode: "consistent",
        includedPaths: [{ path: "/*" }],
        excludedPaths: [{ path: '/"_etag"/?' }],
        vectorIndexes: [{ path: "/vectors", type: "diskANN" }],
      },
      uniqueKeyPolicy: {
        uniqueKeys: [{ paths: ["/userId"] }],
      },
      computedProperties: [
        {
          name: "cp_lowerName",
          query: "SELECT VALUE LOWER(c.userName) FROM c",
        },
      ],
      conflictResolutionPolicy: {
        mode: "LastWriterWins",
        conflictResolutionPath: "/_ts",
      },
      vectorEmbeddingPolicy: {
        vectorEmbeddings: [
          {
            path: "/vectors",
            dimensions: 1536,
            dataType: "float32",
            distanceFunction: "cosine",
          },
        ],
      },
    },
    // NOTE: When using serverless, omit `options` (no throughput provisioning).
    options: {
      autoscaleSettings: {
        maxThroughput: config.maxAutoscaleThroughput,
      },
    },
  };

  const container = await clients.cosmosClient.sqlResources.beginCreateUpdateSqlContainerAndWait(
    config.resourceGroupName,
    config.accountName,
    config.databaseName,
    config.containerName,
    parameters,
  );

  console.log(`Created/updated SQL container: ${container?.id ?? "(no id returned)"}`);
  return container;
}

/**
 * Update container throughput by adding a delta.
 *
 * If the container is autoscale, updates `autoscaleSettings.maxThroughput`.
 * If the container is manual, updates the fixed `throughput` value.
 *
 * @param {{
 *   cosmosClient: import('@azure/arm-cosmosdb').CosmosDBManagementClient
 * }} clients
 * @param {{
 *   resourceGroupName: string,
 *   accountName: string,
 *   location: string,
 *   databaseName: string,
 *   containerName: string,
 *   maxAutoscaleThroughput: number
 * }} config
 * @param {number} addThroughput
 */
export async function updateContainerThroughput(clients, config, addThroughput) {
  console.log(
    `Starting throughput update (this can take a couple minutes): account=${config.accountName}, database=${config.databaseName}, container=${config.containerName}, delta=${addThroughput}`,
  );

  let existing;
  try {
    existing = await clients.cosmosClient.sqlResources.getSqlContainerThroughput(
      config.resourceGroupName,
      config.accountName,
      config.databaseName,
      config.containerName,
    );
  } catch (err) {
    // 404 typically means there's no dedicated container throughput (shared database throughput or serverless).
    const status = /** @type {{ statusCode?: number }} */ (err)?.statusCode;
    if (status === 404) {
      throw new Error(
        "Container throughput settings were not found. This usually means the container uses shared database throughput or serverless, without a dedicated throughput resource to update.",
      );
    }
    throw err;
  }

  const existingResource = existing?.resource;
  if (!existingResource) {
    throw new Error(
      "Container throughput settings did not include a resource payload. The container likely uses shared database throughput or serverless.",
    );
  }

  const currentAutoscaleMax = existingResource.autoscaleSettings?.maxThroughput;
  const currentManualThroughput = existingResource.throughput;

  /** @type {import('@azure/arm-cosmosdb').ThroughputSettingsUpdateParameters} */
  const update = {
    location: config.location,
    resource: {},
  };

  if (currentAutoscaleMax != null) {
    const baseline = currentAutoscaleMax === 0 ? config.maxAutoscaleThroughput : currentAutoscaleMax;
    const newAutoscaleMax = Math.max(1000, baseline + addThroughput);
    console.log(
      `Updating container autoscale max throughput from ${currentAutoscaleMax} to ${newAutoscaleMax}`,
    );
    update.resource.autoscaleSettings = { maxThroughput: newAutoscaleMax };
  } else {
    const baseline = currentManualThroughput ?? 0;
    const newManualThroughput = Math.max(400, baseline + addThroughput);
    console.log(`Updating container manual throughput from ${baseline} to ${newManualThroughput}`);
    update.resource.throughput = newManualThroughput;
  }

  const updated = await clients.cosmosClient.sqlResources.beginUpdateSqlContainerThroughputAndWait(
    config.resourceGroupName,
    config.accountName,
    config.databaseName,
    config.containerName,
    update,
  );

  const updatedResource = updated?.resource;
  const newAuto = updatedResource?.autoscaleSettings?.maxThroughput;
  const newManual = updatedResource?.throughput;
  if (newAuto != null) {
    console.log(`Updated container autoscale max throughput: ${newAuto}`);
  } else if (newManual != null) {
    console.log(`Updated container manual throughput: ${newManual}`);
  } else {
    console.log("Updated throughput settings (no throughput value returned).");
  }

  return updated;
}
