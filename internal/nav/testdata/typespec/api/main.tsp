import "@typespec/http";
import "./models.tsp";
//       ^ resolved models.tsp

using TypeSpec.Http;
using Contoso.Models;

namespace Contoso.Billing;

alias Bill = Invoice;
//           ^ resolved :Invoice "in this file"

@doc("An invoice.")
//^ builtin
model Invoice extends Base {
//                    ^ resolved models.tsp:Base "in Contoso.Models"
  @tagged("i")
  //^ resolved models.tsp:tagged "in Contoso.Models"
  status?: Status = Status.Open;
  //       ^ resolved models.tsp:Status
  //                       ^ resolved models.tsp:Open "a member of Status"
  lines: Contoso.Models.Line[];
  //                    ^ resolved models.tsp:Line "in Contoso.Models"
  ...Audit;
  // ^ resolved models.tsp:Audit
}

model Page<T> {
  items: T[];
  //     ^ resolved :T "local"
}

model Paid is Invoice {}

interface Invoices {
  @route("/invoices")
  //^ external "likely from TypeSpec.Http"
  list(): Page<Invoice>;
  //      ^ resolved :Page
  get(id: string): Bill;
  //               ^ resolved :Bill
}

op ping(): void;
//         ^ builtin
