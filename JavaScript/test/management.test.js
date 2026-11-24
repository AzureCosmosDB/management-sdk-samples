import assert from "node:assert/strict";
import test from "node:test";
import { AuthorizationManagementClient } from "@azure/arm-authorization";
import { CosmosDBManagementClient } from "@azure/arm-cosmosdb";
import { v5 as uuidv5 } from "uuid";
import * as sample from "../src/cosmosManagement.js";

const config = {
  subscriptionId: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  resourceGroupName: "sample-rg",
  accountName: "sampleaccount",
  databaseName: "sample-db",
  containerName: "sample-container",
  location: "westus",
  maxAutoscaleThroughput: 3000,
};
const subscription = `/subscriptions/${config.subscriptionId}`;
const account = `${subscription}/resourceGroups/sample-rg/providers/Microsoft.DocumentDB/databaseAccounts/sampleaccount`;
const database = `${account}/sqlDatabases/sample-db`;
const container = `${database}/containers/sample-container`;
const principalId = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb";
const email = "owner@example.invalid";

// Exercise the real SDK serializers and response parsers. Only the credential
// and the last HTTP transport step are replaced.
function mockClients(expected) {
  const credential = {
    async getToken() {
      const claims = Buffer.from(JSON.stringify({ oid: principalId, upn: email })).toString("base64url");
      return { token: `mock.${claims}.mock`, expiresOnTimestamp: Date.now() + 3600000 };
    },
  };
  const pending = [...expected];
  const httpClient = {
    async sendRequest(request) {
      const step = pending.shift();
      assert.ok(step, `Unexpected ${request.method} ${request.url}`);
      const url = new URL(request.url);
      assert.equal(url.origin, "https://management.azure.com");
      assert.equal(url.pathname, step.path);
      assert.equal(request.method, step.method);
      assert.ok(url.searchParams.get("api-version"));
      const body = request.body && JSON.parse(request.body);
      if (step.check) step.check(body);
      else assert.equal(body, undefined);
      const headers = new Headers({ "content-type": "application/json" });
      headers.toJSON = () => Object.fromEntries(headers);
      return {
        request, headers, status: step.status ?? 200,
        bodyAsText: step.status === 204 ? undefined : JSON.stringify({ id: step.path, ...body, ...step.response }),
      };
    },
  };
  const options = { httpClient, retryOptions: { maxRetries: 0 } };
  return {
    clients: {
      credential,
      cosmosClient: new CosmosDBManagementClient(credential, config.subscriptionId, options),
      authorizationClient: new AuthorizationManagementClient(credential, config.subscriptionId, options),
    },
    done: () => assert.equal(pending.length, 0, "Expected requests were not sent"),
  };
}

test("account, database and container resource URLs and provisioning parameters", async () => {
  const { clients, done } = mockClients([
    { method: "PUT", path: account, check(body) {
      assert.deepEqual(body, {
        kind: "GlobalDocumentDB", location: config.location, tags: { owner: email },
        properties: {
          locations: [{ locationName: config.location, failoverPriority: 0, isZoneRedundant: false }],
          databaseAccountOfferType: "Standard", disableLocalAuth: true,
          capabilities: [{ name: "EnableNoSQLVectorSearch" }],
        },
      });
    } },
    { method: "PUT", path: database, check(body) {
      assert.deepEqual(body, { location: config.location, properties: { resource: { id: config.databaseName } } });
    } },
    { method: "PUT", path: container, check(body) {
      assert.equal(body.location, config.location);
      const { options, resource } = body.properties;
      assert.deepEqual(options, { autoscaleSettings: { maxThroughput: config.maxAutoscaleThroughput } });
      assert.equal(resource.id, config.containerName);
      assert.equal(resource.defaultTtl, -1);
      assert.deepEqual(resource.partitionKey, { paths: ["/companyId", "/departmentId", "/userId"], kind: "MultiHash", version: 2 });
      assert.deepEqual(resource.indexingPolicy, {
        automatic: true, indexingMode: "consistent", includedPaths: [{ path: "/*" }],
        excludedPaths: [{ path: '/"_etag"/?' }], vectorIndexes: [{ path: "/vectors", type: "diskANN" }],
      });
      assert.deepEqual(resource.vectorEmbeddingPolicy.vectorEmbeddings, [
        { path: "/vectors", dataType: "float32", dimensions: 1536, distanceFunction: "cosine" },
      ]);
      assert.deepEqual(resource.uniqueKeyPolicy.uniqueKeys, [{ paths: ["/userId"] }]);
      assert.deepEqual(resource.conflictResolutionPolicy, { mode: "LastWriterWins", conflictResolutionPath: "/_ts" });
      assert.deepEqual(resource.computedProperties, [{ name: "cp_lowerName", query: "SELECT VALUE LOWER(c.userName) FROM c" }]);
    } },
    { method: "DELETE", path: account, status: 204 },
  ]);
  await sample.createOrUpdateCosmosDbAccount(clients, config);
  await sample.createOrUpdateSqlDatabase(clients, config);
  await sample.createOrUpdateSqlContainer(clients, config);
  await sample.deleteCosmosDbAccount(clients, config);
  done();
});

test("Azure and Cosmos RBAC use the correct scope, definition and assignment IDs", async (t) => {
  const previous = process.env.AZURE_PRINCIPAL_OBJECT_ID;
  process.env.AZURE_PRINCIPAL_OBJECT_ID = principalId;
  t.after(() => {
    if (previous === undefined) delete process.env.AZURE_PRINCIPAL_OBJECT_ID;
    else process.env.AZURE_PRINCIPAL_OBJECT_ID = previous;
  });
  const operator = `${subscription}/providers/Microsoft.Authorization/roleDefinitions/230815da-be43-4aae-9cb4-875f7bd000aa`;
  const contributor = `${account}/sqlRoleDefinitions/00000000-0000-0000-0000-000000000002`;
  const assignment = (role) => uuidv5(`${account}|${role}|${principalId}`, uuidv5.URL);
  const { clients, done } = mockClients([
    { method: "PUT", path: `${account}/providers/Microsoft.Authorization/roleAssignments/${assignment(operator)}`,
      check: (body) => assert.deepEqual(body.properties, {
        principalId, roleDefinitionId: operator, description: "Role assignment for Cosmos DB (management plane)",
      }) },
    { method: "GET", path: contributor },
    { method: "PUT", path: `${account}/sqlRoleAssignments/${assignment(contributor)}`,
      check: (body) => assert.deepEqual(body.properties, { principalId, roleDefinitionId: contributor, scope: account }) },
    { method: "PUT", path: `${account}/sqlRoleDefinitions/11111111-1111-1111-1111-111111111111`, check(body) {
      const { roleName, type, assignableScopes, permissions } = body.properties;
      assert.equal(roleName, "My Custom Cosmos DB Data Role Except Delete");
      assert.equal(type, "CustomRole");
      assert.deepEqual(assignableScopes, [account]);
      assert.ok(permissions[0].dataActions.includes("Microsoft.DocumentDB/databaseAccounts/readMetadata"));
      assert.ok(!permissions[0].dataActions.some((action) => action.endsWith("/delete")));
    } },
  ]);
  await sample.createOrUpdateAzureRbacCosmosDbOperatorAssignment(clients, config);
  await sample.createOrUpdateCosmosNoSqlRbacDataContributorAssignment(clients, config);
  await sample.createOrUpdateCustomCosmosNoSqlRbacRoleDefinitionExceptDelete(clients, config);
  done();
});

for (const [name, before, after] of [
  ["autoscale", { autoscaleSettings: { maxThroughput: 3000 } }, { autoscaleSettings: { maxThroughput: 4000 } }],
  ["manual", { throughput: 500 }, { throughput: 1500 }],
]) {
  test(`${name} throughput reads and updates the dedicated container offer`, async () => {
    const path = `${container}/throughputSettings/default`;
    const { clients, done } = mockClients([
      { method: "GET", path, response: { properties: { resource: before } } },
      { method: "PUT", path, check: (body) => assert.deepEqual(body, {
        location: config.location, properties: { resource: after },
      }) },
    ]);
    const result = await sample.updateContainerThroughput(clients, config, 1000);
    assert.partialDeepStrictEqual(result.resource, after);
    done();
  });
}
