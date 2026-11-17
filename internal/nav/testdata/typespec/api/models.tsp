namespace Contoso.Models;

/** Something kept. */
model Base {
  id: string;
}

model Audit {
  createdAt: utcDateTime;
}

enum Status {
  Open,
  Paid: "paid",
}

model Line {
  amount: decimal;
}

dec tagged(target: Model, name: string);
