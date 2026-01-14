package com.example;

/**
 * Strongly-typed configuration for the Cosmos DB management sample.
 *
 * <p>Values are loaded via {@link ConfigLoader} and validated up-front so the sample fails fast with
 * actionable error messages.
 */
public final class CosmosConfig {
    private final String subscriptionId;
    private final String resourceGroupName;
    private final String accountName;
    private final String location;
    private final String databaseName;
    private final String containerName;
    private final int maxAutoScaleThroughput;

    public CosmosConfig(
        String subscriptionId,
        String resourceGroupName,
        String accountName,
        String location,
        String databaseName,
        String containerName,
        int maxAutoScaleThroughput) {

        this.subscriptionId = requireNonBlank(subscriptionId, "SubscriptionId");
        this.resourceGroupName = requireNonBlank(resourceGroupName, "ResourceGroupName");
        this.accountName = requireNonBlank(accountName, "AccountName");
        this.location = requireNonBlank(location, "Location");
        this.databaseName = requireNonBlank(databaseName, "DatabaseName");
        this.containerName = requireNonBlank(containerName, "ContainerName");

        if (maxAutoScaleThroughput < 1000) {
            throw new IllegalArgumentException("MaxAutoScaleThroughput must be >= 1000");
        }
        this.maxAutoScaleThroughput = maxAutoScaleThroughput;
    }

    public String subscriptionId() {
        return subscriptionId;
    }

    public String resourceGroupName() {
        return resourceGroupName;
    }

    public String accountName() {
        return accountName;
    }

    public String location() {
        return location;
    }

    public String databaseName() {
        return databaseName;
    }

    public String containerName() {
        return containerName;
    }

    public int maxAutoScaleThroughput() {
        return maxAutoScaleThroughput;
    }

    private static String requireNonBlank(String value, String key) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException("Missing required configuration value: " + key);
        }
        return value;
    }
}
