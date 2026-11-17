using System.Collections.Generic;

namespace Contoso.Billing
{
    public enum Status { Open, Paid }

    public partial class Invoice : EntityBase, IPayable
    //                             ^ resolved src/Shared/Money.cs:EntityBase "in Contoso.Shared"
    {
        private readonly List<Line> _lines = new();
        public Status State { get; set; } = Status.Open;
        //                                         ^ resolved :Open "a member of Status"

        public Money Pay(int count)
        {
            var total = MoneyMath.Zero("GBP");
            //          ^ resolved src/Shared/Money.cs:MoneyMath "in Contoso.Shared"
            //                    ^ resolved src/Shared/Money.cs:Zero
            foreach (var line in _lines)
            //                   ^ resolved :_lines "a member of Invoice"
            {
                total = total.Add(line.Price);
                //            ^ resolved src/Shared/Money.cs:Add "by the type written on total"
                //                     ^ resolved src/Billing/Line.cs:Price "by the type written on line"
            }
            _lines.Add(new Line());
            //     ^ builtin
            State = Status.Paid;
            // <- resolved :State "a member of Invoice"
            return total;
        }

        public override string Describe() => base.Describe() + Summary();
        //                                        ^ resolved src/Shared/Money.cs:Describe
        //                                                     ^ resolved src/Billing/Invoice.Summary.cs:Summary "a member of Invoice"
    }
}
