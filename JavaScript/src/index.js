import process from "node:process";
import readline from "node:readline/promises";

import { loadConfig, printConfigurationHelp } from "./config.js";
import { closeClients, createClients } from "./clients.js";
import { getCurrentPrincipalObjectId, getCurrentUserEmailBestEffort } from "./tokenClaims.js";
import {
  createOrUpdateCosmosDbAccount,
  createOrUpdateAzureRbacCosmosDbOperatorAssignment,
  createOrUpdateCosmosNoSqlRbacDataContributorAssignment,
  createOrUpdateSqlContainer,
  createOrUpdateSqlDatabase,
  deleteCosmosDbAccount,
  updateContainerThroughput,
} from "./cosmosManagement.js";

function getArgValue(prefix) {
  const match = process.argv.find((arg) => arg.startsWith(prefix));
  if (!match) return null;
  const value = match.slice(prefix.length);
  return value ? value : null;
}

function hasAnyRunFlags() {
  return (
    process.argv.includes("--run-account") ||
    process.argv.includes("--run-db") ||
    process.argv.includes("--run-container") ||
    process.argv.includes("--run-throughput") ||
    process.argv.includes("--run-azure-rbac") ||
    process.argv.includes("--run-cosmos-rbac")
  );
}

function isDeleteOptInEnabled() {
  return String(process.env.COSMOS_SAMPLE_DELETE_ACCOUNT || "").toLowerCase() === "true";
}

async function promptInt(rl, label, defaultValue) {
  const raw = (await rl.question(`${label} (default ${defaultValue}): `)).trim();
  if (!raw) return defaultValue;
  const value = Number.parseInt(raw, 10);
  return Number.isNaN(value) ? defaultValue : value;
}

async function confirmDelete(rl) {
  const raw = (await rl.question("Type DELETE to confirm deleting the Cosmos DB account: ")).trim();
  return raw === "DELETE";
}

async function runFullSample(clients, config) {
  await createOrUpdateCosmosDbAccount(clients, config);
  await createOrUpdateAzureRbacCosmosDbOperatorAssignment(clients, config);
  await createOrUpdateSqlDatabase(clients, config);
  await createOrUpdateSqlContainer(clients, config);
  await updateContainerThroughput(clients, config, 1000);
  await createOrUpdateCosmosNoSqlRbacDataContributorAssignment(clients, config);

  if (isDeleteOptInEnabled()) {
    await deleteCosmosDbAccount(clients, config);
  }
}

async function runFlagFlow(clients, config) {
  // Optional non-interactive shortcuts.
  // Use:
  //   `npm run start -- --run-account`
  //   `npm run start -- --run-db`
  //   `npm run start -- --run-container`
  //   `npm run start -- --run-throughput --throughput-delta=1000`
  //   `npm run start -- --run-azure-rbac`
  //   `npm run start -- --run-cosmos-rbac`

  const runAccount = process.argv.includes("--run-account");
  const runDb = process.argv.includes("--run-db");
  const runContainer = process.argv.includes("--run-container");
  const runThroughput = process.argv.includes("--run-throughput");
  const runAzureRbac = process.argv.includes("--run-azure-rbac");
  const runCosmosRbac = process.argv.includes("--run-cosmos-rbac");

  if (runAccount) {
    await createOrUpdateCosmosDbAccount(clients, config);
  }

  if (runContainer) {
    // The container depends on the database.
    await createOrUpdateSqlDatabase(clients, config);
    await createOrUpdateSqlContainer(clients, config);
  } else if (runDb) {
    await createOrUpdateSqlDatabase(clients, config);
  }

  if (runThroughput) {
    const rawDelta = getArgValue("--throughput-delta=");
    const delta = rawDelta == null ? 1000 : Number.parseInt(rawDelta, 10);
    if (Number.isNaN(delta)) {
      throw new Error(`Invalid --throughput-delta value: '${rawDelta}'`);
    }
    await updateContainerThroughput(clients, config, delta);
  }

  if (runAzureRbac) {
    await createOrUpdateAzureRbacCosmosDbOperatorAssignment(clients, config);
  }

  if (runCosmosRbac) {
    await createOrUpdateCosmosNoSqlRbacDataContributorAssignment(clients, config);
  }
}

async function runInteractiveMenu(clients, config) {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  try {
    while (true) {
      console.log();
      console.log("Cosmos management sample - choose an action:");
      console.log("  1) Run full sample");
      console.log("  2) Create/update Cosmos DB account");
      console.log("  3) Create Azure RBAC assignment (Cosmos DB Operator)");
      console.log("  4) Create/update SQL database");
      console.log("  5) Create/update container");
      console.log("  6) Update container throughput (+delta)");
      console.log("  7) Create Cosmos SQL RBAC assignment (Built-in Data Contributor)");
      console.log("  8) Delete Cosmos DB account");
      console.log("  0) Exit");
      const selection = (await rl.question("Selection: ")).trim().toLowerCase();
      if (!selection) continue;

      try {
        switch (selection) {
          case "0":
          case "q":
          case "quit":
          case "exit":
            return;
          case "1":
            await runFullSample(clients, config);
            break;
          case "2":
            await createOrUpdateCosmosDbAccount(clients, config);
            break;
          case "3":
            await createOrUpdateAzureRbacCosmosDbOperatorAssignment(clients, config);
            break;
          case "4":
            await createOrUpdateSqlDatabase(clients, config);
            break;
          case "5":
            await createOrUpdateSqlContainer(clients, config);
            break;
          case "6": {
            const delta = await promptInt(rl, "Throughput delta to add", 1000);
            await updateContainerThroughput(clients, config, delta);
            break;
          }
          case "7":
            await createOrUpdateCosmosNoSqlRbacDataContributorAssignment(clients, config);
            break;
          case "8":
            if (await confirmDelete(rl)) {
              await deleteCosmosDbAccount(clients, config);
            } else {
              console.log("Delete cancelled.");
            }
            break;
          default:
            console.log("Unknown selection.");
            break;
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        console.error(`Operation failed: ${message}`);
      }
    }
  } finally {
    rl.close();
  }
}

async function main() {
  console.log("Cosmos DB management SDK sample (JavaScript)");

  try {
    const config = loadConfig();

    const clients = createClients(config);

    // For now, just confirm configuration loads.
    // Next steps: token-claim helpers, interactive menu + Cosmos management operations.
    console.log("Configuration loaded:");
    console.log(`  subscriptionId: ${config.subscriptionId}`);
    console.log(`  resourceGroupName: ${config.resourceGroupName}`);
    console.log(`  location: ${config.location}`);
    console.log(`  accountName: ${config.accountName}`);
    console.log(`  databaseName: ${config.databaseName}`);
    console.log(`  containerName: ${config.containerName}`);
    console.log(`  maxAutoscaleThroughput: ${config.maxAutoscaleThroughput}`);
    console.log("Clients initialized:");
    console.log(`  cosmosClient: ${clients.cosmosClient.constructor?.name ?? "CosmosDBManagementClient"}`);
    console.log(
      `  authorizationClient: ${clients.authorizationClient.constructor?.name ?? "AuthorizationManagementClient"}`,
    );

    const principalObjectId = await getCurrentPrincipalObjectId(clients.credential);
    const ownerEmail = await getCurrentUserEmailBestEffort(clients.credential);
    console.log("Identity (from ARM token claims):");
    console.log(`  principalObjectId (oid): ${principalObjectId}`);
    console.log(`  ownerEmail (best-effort): ${ownerEmail ?? "(not present)"}`);

    console.log("Node:", process.version);

    if (hasAnyRunFlags()) {
      await runFlagFlow(clients, config);
    } else {
      if (!process.stdin.isTTY) {
        console.error(
          "Interactive menu requires a TTY. Run in a terminal, or use explicit --run-* flags for non-interactive execution.",
        );
        process.exitCode = 1;
        return;
      }

      await runInteractiveMenu(clients, config);
    }

    await closeClients(clients);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error(message);
    printConfigurationHelp();
    process.exitCode = 2;
  }
}

void main();
