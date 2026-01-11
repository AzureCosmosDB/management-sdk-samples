using System.ComponentModel.DataAnnotations;

public sealed class AppSettings
{
    [Required]
    public string SubscriptionId { get; set; } = string.Empty;

    [Required]
    public string ResourceGroupName { get; set; } = string.Empty;

    [Required]
    public string AccountName { get; set; } = string.Empty;

    [Required]
    public string Location { get; set; } = string.Empty;

    [Required]
    public string DatabaseName { get; set; } = string.Empty;

    [Required]
    public string ContainerName { get; set; } = string.Empty;

    [Range(1000, int.MaxValue)]
    public int MaxAutoScaleThroughput { get; set; } = 1000;
}
