package com.example;

import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Properties;

/**
 * Loads and validates configuration for this sample.
 *
 * <p>Lookup order for each key:
 * <ol>
 *   <li>Environment variables (recommended for CI)</li>
 *   <li>Optional local properties file: {@code Java/application.properties}</li>
 * </ol>
 */
public final class ConfigLoader {
     private static final String DEFAULT_PROPERTIES_FILE = "application.properties";

    private ConfigLoader() {
    }

    public static CosmosConfig load(Path projectRoot) {
        Properties properties = new Properties();
        Path propertiesPath = projectRoot.resolve(DEFAULT_PROPERTIES_FILE);
        if (Files.exists(propertiesPath)) {
            try (InputStream in = Files.newInputStream(propertiesPath)) {
                properties.load(in);
            } catch (IOException e) {
                throw new IllegalStateException("Failed to read " + propertiesPath + ": " + e.getMessage(), e);
            }
        }

        String subscriptionId = firstNonBlank(
            System.getenv("AZURE_SUBSCRIPTION_ID"),
            properties.getProperty("subscriptionId"));

        String resourceGroupName = firstNonBlank(
            System.getenv("AZURE_RESOURCE_GROUP"),
            properties.getProperty("resourceGroupName"));

        String accountName = firstNonBlank(
            System.getenv("COSMOS_ACCOUNT_NAME"),
            properties.getProperty("accountName"));

        String location = firstNonBlank(
            System.getenv("AZURE_LOCATION"),
            properties.getProperty("location"));

        String databaseName = firstNonBlank(
            System.getenv("COSMOS_DATABASE_NAME"),
            properties.getProperty("databaseName"));

        String containerName = firstNonBlank(
            System.getenv("COSMOS_CONTAINER_NAME"),
            properties.getProperty("containerName"));

        int maxAutoScaleThroughput = parseIntOrDefault(
            firstNonBlank(
                System.getenv("COSMOS_MAX_AUTOSCALE_THROUGHPUT"),
                properties.getProperty("maxAutoScaleThroughput")),
            1000);

        return new CosmosConfig(
            subscriptionId,
            resourceGroupName,
            accountName,
            location,
            databaseName,
            containerName,
            maxAutoScaleThroughput);
    }

    public static void printConfigurationHelp() {
        System.err.println();
        System.err.println("Missing configuration. Provide values via environment variables or Java/application.properties.");
        System.err.println();
        System.err.println("Environment variables:");
        System.err.println("  AZURE_SUBSCRIPTION_ID");
        System.err.println("  AZURE_RESOURCE_GROUP");
        System.err.println("  AZURE_LOCATION");
        System.err.println("  COSMOS_ACCOUNT_NAME");
        System.err.println("  COSMOS_DATABASE_NAME");
        System.err.println("  COSMOS_CONTAINER_NAME");
        System.err.println("  COSMOS_MAX_AUTOSCALE_THROUGHPUT (optional, default 1000)");
        System.err.println();
        System.err.println("application.properties keys:");
        System.err.println("  subscriptionId=...");
        System.err.println("  resourceGroupName=...");
        System.err.println("  location=...  (example: eastus)");
        System.err.println("  accountName=...");
        System.err.println("  databaseName=...");
        System.err.println("  containerName=...");
        System.err.println("  maxAutoScaleThroughput=1000");
        System.err.println();
    }

    private static String firstNonBlank(String first, String second) {
        if (first != null && !first.isBlank()) {
            return first;
        }
        if (second != null && !second.isBlank()) {
            return second;
        }
        return null;
    }

    private static int parseIntOrDefault(String raw, int defaultValue) {
        if (raw == null || raw.isBlank()) {
            return defaultValue;
        }

        try {
            return Integer.parseInt(raw.trim());
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("Invalid integer value: '" + raw + "'", e);
        }
    }
}
