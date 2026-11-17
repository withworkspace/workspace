namespace Contoso.Billing;

public class Line
{
    public Money Price { get; set; } = MoneyMath.Zero("GBP");
}
