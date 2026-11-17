namespace Contoso.Billing
{
    public partial class Invoice
    {
        public string Summary() => this.State.ToString() + Id;
        //                              ^ resolved src/Billing/Invoice.cs:State
        //                                                 ^ resolved src/Shared/Money.cs:Id "a member of Invoice"
    }
}
