import { DefaultAzureCredential } from "@azure/identity";
import { AuthorizationManagementClient } from "@azure/arm-authorization";
import { CosmosDBManagementClient } from "@azure/arm-cosmosdb";

/**
 * Creates Azure SDK clients used by this sample.
 *
 * Notes:
 * - This sample uses DefaultAzureCredential (Azure CLI login, VS Code sign-in, Managed Identity, etc.)
 * - Client instances should be reused (do not create per operation).
 */
export function createClients(config) {
  if (!config?.subscriptionId) {
    throw new Error("createClients requires config.subscriptionId");
  }

  const credential = new DefaultAzureCredential();

  const cosmosClient = new CosmosDBManagementClient(credential, config.subscriptionId);
  const authorizationClient = new AuthorizationManagementClient(credential, config.subscriptionId);

  return {
    credential,
    cosmosClient,
    authorizationClient,
  };
}

/**
 * Best-effort cleanup for credentials.
 *
 * Azure management clients don't require disposal, but credentials may hold resources.
 */
export async function closeClients(clients) {
  if (!clients?.credential) return;

  const maybeClose = clients.credential.close;
  if (typeof maybeClose === "function") {
    await maybeClose.call(clients.credential);
  }
}
