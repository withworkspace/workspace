using System;
using Contoso.Billing;
using static Contoso.Shared.MoneyMath;
using Bill = Contoso.Billing.Invoice;

namespace Contoso.App;

public class Program
{
    public static void Main()
    {
        var invoice = new Invoice();
        //                ^ resolved src/Billing/Invoice.cs:Invoice src/Billing/Invoice.Summary.cs:Invoice "in Contoso.Billing"
        Bill other = new Bill();
        // <- resolved src/Billing/Invoice.cs:Invoice src/Billing/Invoice.Summary.cs:Invoice
        var paid = invoice.Pay(2);
        //                 ^ resolved src/Billing/Invoice.cs:Pay "by the type written on invoice"
        Console.WriteLine(paid.Amount + other.Summary());
        // <- builtin
        //                     ^ resolved src/Shared/Money.cs:Amount
        //                                    ^ resolved src/Billing/Invoice.Summary.cs:Summary
        var zero = Zero("EUR");
        //         ^ resolved src/Shared/Money.cs:Zero "using static Contoso.Shared.MoneyMath"
        var full = new Contoso.Billing.Line();
        //                             ^ resolved src/Billing/Line.cs:Line "in Contoso.Billing"
        Nothing.Here();
        // <- external "likely from System"
        nothing.Here();
        // <- unresolved
    }
}
