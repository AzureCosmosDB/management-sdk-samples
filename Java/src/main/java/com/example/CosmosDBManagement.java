package com.example;

import java.nio.file.Path;
import java.util.Locale;
import java.util.Scanner;

/**
 * Entry point for the Cosmos DB management-plane Java sample.
 *
 * <p>By default, runs an interactive menu (similar to the C# sample). When stdin is redirected
 * (for example, CI), it runs the full sample once.
 */
public class CosmosDBManagement {

    public static void main(String[] args) {
        CosmosConfig settings;
        try {
            settings = ConfigLoader.load(Path.of("."));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
            ConfigLoader.printConfigurationHelp();
            System.exit(2);
            return;
        }

        CosmosManagement sample = CosmosManagement.create(settings);

        String selection = parseSelection(args);
        if (selection != null) {
            runOne(sample, selection, parseDelta(args), parseConfirmDelete(args));
            return;
        }

        if (isNonInteractiveEnvironment()) {
            sample.run();
            return;
        }

        runInteractive(sample);
    }

    private static boolean isNonInteractiveEnvironment() {
        if (System.in == null) {
            return true;
        }

        String nonInteractive = System.getenv("COSMOS_SAMPLE_NON_INTERACTIVE");
        if (nonInteractive != null && nonInteractive.equalsIgnoreCase("true")) {
            return true;
        }

        // Common CI indicator.
        String ci = System.getenv("CI");
        return ci != null && !ci.isBlank();
    }

    private static void runInteractive(CosmosManagement sample) {
        try (Scanner scanner = new Scanner(System.in)) {
            while (true) {
                System.out.println();
                System.out.println("Cosmos management sample - choose an action:");
                System.out.println("  1) Run full sample");
                System.out.println("  2) Create/update Cosmos DB account");
                System.out.println("  3) Create Azure RBAC assignment (Cosmos DB Operator)");
                System.out.println("  4) Create/update NoSQL database");
                System.out.println("  5) Create/update NoSQL container");
                System.out.println("  6) Update container throughput (+delta)");
                System.out.println("  7) Create Cosmos NoSQL RBAC assignment (Built-in Data Contributor)");
                System.out.println("  8) Delete Cosmos DB account");
                System.out.println("  0) Exit");
                System.out.print("Selection: ");

                String selection = scanner.nextLine().trim();
                if (selection.isBlank()) {
                    continue;
                }

                try {
                    switch (selection) {
                        case "1" -> sample.run();
                        case "2" -> sample.createOrUpdateCosmosDBAccount();
                        case "3" -> sample.createOrUpdateAzureRoleAssignment();
                        case "4" -> sample.createOrUpdateCosmosDBDatabase();
                        case "5" -> sample.createOrUpdateCosmosDBContainer();
                        case "6" -> {
                            int delta = promptInt(scanner, "Throughput delta to add", 1000);
                            sample.updateThroughput(delta);
                        }
                        case "7" -> sample.createOrUpdateCosmosSqlRoleAssignment();
                        case "8" -> {
                            if (confirmDelete(scanner)) {
                                sample.deleteCosmosDBAccount();
                            } else {
                                System.out.println("Delete cancelled.");
                            }
                        }
                        case "0", "q", "quit", "exit" -> {
                            return;
                        }
                        default -> System.out.println("Unknown selection.");
                    }
                } catch (Exception ex) {
                    System.err.println("Operation failed: " + ex.getClass().getName() + ": " + ex.getMessage());
                    ex.printStackTrace(System.err);
                }
            }
        }
    }

    private static void runOne(CosmosManagement sample, String selection, Integer delta, boolean confirmDelete) {
        switch (selection) {
            case "1" -> sample.run();
            case "2" -> sample.createOrUpdateCosmosDBAccount();
            case "3" -> sample.createOrUpdateAzureRoleAssignment();
            case "4" -> sample.createOrUpdateCosmosDBDatabase();
            case "5" -> sample.createOrUpdateCosmosDBContainer();
            case "6" -> sample.updateThroughput(delta != null ? delta : 1000);
            case "7" -> sample.createOrUpdateCosmosSqlRoleAssignment();
            case "8" -> {
                if (!confirmDelete) {
                    throw new IllegalArgumentException("Refusing to delete without --confirm-delete.");
                }
                sample.deleteCosmosDBAccount();
            }
            default -> throw new IllegalArgumentException("Unknown selection: " + selection);
        }
    }

    private static String parseSelection(String[] args) {
        if (args == null || args.length == 0) {
            return null;
        }

        for (String arg : args) {
            if (arg == null) {
                continue;
            }

            String trimmed = arg.trim();
            if (trimmed.isBlank()) {
                continue;
            }

            if (trimmed.equals("--help") || trimmed.equals("-h")) {
                printHelp();
                System.exit(0);
            }

            if (trimmed.matches("^[0-8]$")) {
                return trimmed;
            }

            String lower = trimmed.toLowerCase(Locale.ROOT);
            if (lower.startsWith("--option=")) {
                return trimmed.substring("--option=".length()).trim();
            }
            if (lower.startsWith("--selection=")) {
                return trimmed.substring("--selection=".length()).trim();
            }
        }

        return null;
    }

    private static Integer parseDelta(String[] args) {
        if (args == null) {
            return null;
        }

        for (String arg : args) {
            if (arg == null) {
                continue;
            }
            String trimmed = arg.trim();
            if (trimmed.isBlank()) {
                continue;
            }
            String lower = trimmed.toLowerCase(Locale.ROOT);
            if (lower.startsWith("--delta=")) {
                String value = trimmed.substring("--delta=".length()).trim();
                try {
                    return Integer.parseInt(value);
                } catch (NumberFormatException e) {
                    throw new IllegalArgumentException("Invalid --delta value: " + value);
                }
            }
        }

        return null;
    }

    private static boolean parseConfirmDelete(String[] args) {
        if (args == null) {
            return false;
        }

        for (String arg : args) {
            if (arg == null) {
                continue;
            }
            if (arg.trim().equalsIgnoreCase("--confirm-delete")) {
                return true;
            }
        }

        return false;
    }

    private static void printHelp() {
        System.out.println("Usage: CosmosDBManagement [<selection>] [--delta=<n>] [--confirm-delete]");
        System.out.println();
        System.out.println("Selections:");
        System.out.println("  1  Run full sample");
        System.out.println("  2  Create/update Cosmos DB account");
        System.out.println("  3  Create Azure RBAC assignment (Cosmos DB Operator)");
        System.out.println("  4  Create/update NoSQL database");
        System.out.println("  5  Create/update NoSQL container");
        System.out.println("  6  Update container throughput (+delta)");
        System.out.println("  7  Create Cosmos NoSQL RBAC assignment (Built-in Data Contributor)");
        System.out.println("  8  Delete Cosmos DB account (requires --confirm-delete)");
    }

    private static int promptInt(Scanner scanner, String label, int defaultValue) {
        System.out.print(label + " (default " + defaultValue + "): ");
        String raw = scanner.nextLine();
        if (raw == null || raw.isBlank()) {
            return defaultValue;
        }

        try {
            return Integer.parseInt(raw.trim());
        } catch (NumberFormatException e) {
            return defaultValue;
        }
    }

    private static boolean confirmDelete(Scanner scanner) {
        System.out.print("Type DELETE to confirm deleting the Cosmos DB account: ");
        String raw = scanner.nextLine();
        return raw != null && raw.trim().toUpperCase(Locale.ROOT).equals("DELETE");
    }
}
