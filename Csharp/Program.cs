using Azure.Core;
using Azure.Identity;
using Azure.ResourceManager;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;

public static class Program
{
    // Generic Host for configuration, DI, and graceful shutdown.
    public static async Task Main(string[] args)
    {
        // Default to Development for this sample unless the user explicitly overrides it.
        // This ensures appsettings.development.json is picked up without requiring extra setup.
        if (string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable("DOTNET_ENVIRONMENT")))
        {
            Environment.SetEnvironmentVariable("DOTNET_ENVIRONMENT", Environments.Development);
        }

        using IHost host = Host.CreateDefaultBuilder(args)
            .ConfigureServices((context, services) =>
            {
                services
                    .AddOptions<AppSettings>()
                    .Bind(context.Configuration)
                    .ValidateDataAnnotations()
                    .Validate(o =>
                        !string.IsNullOrWhiteSpace(o.SubscriptionId) &&
                        !string.IsNullOrWhiteSpace(o.ResourceGroupName) &&
                        !string.IsNullOrWhiteSpace(o.AccountName) &&
                        !string.IsNullOrWhiteSpace(o.Location) &&
                        !string.IsNullOrWhiteSpace(o.DatabaseName) &&
                        !string.IsNullOrWhiteSpace(o.ContainerName),
                        "Missing required configuration values")
                    .ValidateOnStart();

                services.AddSingleton<TokenCredential>(_ => new DefaultAzureCredential());
                services.AddSingleton(sp => new ArmClient(sp.GetRequiredService<TokenCredential>()));

                services.AddTransient<CosmosManagement>();
            })
            .Build();

        CancellationToken stoppingToken = host.Services.GetRequiredService<IHostApplicationLifetime>().ApplicationStopping;
        CosmosManagement sample = host.Services.GetRequiredService<CosmosManagement>();

        await sample.RunInteractiveAsync(stoppingToken);
    }
}
