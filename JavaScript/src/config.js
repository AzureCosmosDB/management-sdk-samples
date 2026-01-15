import fs from "node:fs";
import path from "node:path";
import process from "node:process";

import dotenv from "dotenv";

/**
 * Loads and validates configuration for this sample.
 *
 * Lookup order for each key:
 * 1) Environment variables (recommended for CI)
 * 2) Optional local dotenv file: `JavaScript/config.env`
 */
export function loadConfig() {
  const envFilePath = path.resolve(process.cwd(), "config.env");
  if (fs.existsSync(envFilePath)) {
    // By default, dotenv does not override values already present in process.env.
    dotenv.config({ path: envFilePath });
  }

  const subscriptionId = clean(process.env.AZURE_SUBSCRIPTION_ID);
  const resourceGroupName = clean(process.env.AZURE_RESOURCE_GROUP);
  const location = clean(process.env.AZURE_LOCATION);

  const accountName = clean(process.env.COSMOS_ACCOUNT_NAME);
  const databaseName = clean(process.env.COSMOS_DATABASE_NAME);
  const containerName = clean(process.env.COSMOS_CONTAINER_NAME);

  const maxAutoscaleThroughput = parseIntOrDefault(
    clean(process.env.COSMOS_MAX_AUTOSCALE_THROUGHPUT),
    1000,
  );

  const missing = [];
  if (!subscriptionId) missing.push("AZURE_SUBSCRIPTION_ID");
  if (!resourceGroupName) missing.push("AZURE_RESOURCE_GROUP");
  if (!location) missing.push("AZURE_LOCATION");
  if (!accountName) missing.push("COSMOS_ACCOUNT_NAME");
  if (!databaseName) missing.push("COSMOS_DATABASE_NAME");
  if (!containerName) missing.push("COSMOS_CONTAINER_NAME");

  if (missing.length > 0) {
    const err = new Error(
      `Missing configuration. Provide values via environment variables or JavaScript/config.env. Missing: ${missing.join(
        ", ",
      )}`,
    );
    err.name = "ConfigError";
    throw err;
  }

  if (maxAutoscaleThroughput < 1000) {
    const err = new Error(
      `COSMOS_MAX_AUTOSCALE_THROUGHPUT must be >= 1000 (got ${maxAutoscaleThroughput}).`,
    );
    err.name = "ConfigError";
    throw err;
  }

  return {
    subscriptionId,
    resourceGroupName,
    location,
    accountName,
    databaseName,
    containerName,
    maxAutoscaleThroughput,
  };
}

export function printConfigurationHelp() {
  // Keep this aligned with config.env.sample.
  // Intentionally printed to stderr so it shows up in CI logs.
  console.error();
  console.error("Missing configuration. Provide values via environment variables or JavaScript/config.env.");
  console.error();
  console.error("Environment variables:");
  console.error("  AZURE_SUBSCRIPTION_ID");
  console.error("  AZURE_RESOURCE_GROUP");
  console.error("  AZURE_LOCATION");
  console.error("  COSMOS_ACCOUNT_NAME");
  console.error("  COSMOS_DATABASE_NAME");
  console.error("  COSMOS_CONTAINER_NAME");
  console.error("  COSMOS_MAX_AUTOSCALE_THROUGHPUT (optional, default 1000)");
  console.error();
  console.error("Setup:");
  console.error("  copy config.env.sample config.env");
  console.error("  # then edit config.env");
}

function clean(value) {
  if (value == null) return null;
  const trimmed = String(value).trim();
  if (!trimmed) return null;

  // Remove wrapping quotes for common dotenv usage.
  if (
    (trimmed.startsWith("\"") && trimmed.endsWith("\"")) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    const unquoted = trimmed.slice(1, -1).trim();
    return unquoted || null;
  }

  return trimmed;
}

function parseIntOrDefault(raw, defaultValue) {
  if (raw == null) return defaultValue;
  const parsed = Number.parseInt(raw, 10);
  if (Number.isNaN(parsed)) {
    const err = new Error(`Invalid integer value: '${raw}'.`);
    err.name = "ConfigError";
    throw err;
  }
  return parsed;
}
