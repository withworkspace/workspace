namespace Contoso.Shared;

/// <summary>An amount in a currency.</summary>
public record Money(decimal Amount, string Currency)
{
    public Money Add(Money other) => new Money(Amount + other.Amount, Currency);
}

public static class MoneyMath
{
    public static Money Zero(string currency) => new Money(0, currency);
}

public interface IPayable
{
    Money Pay(int count);
}

public abstract class EntityBase
{
    public string Id { get; set; } = "";
    public virtual string Describe() => Id;
}
