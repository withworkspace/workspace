// An amount in minor units, as the customer reads it: 109440 GBP is £1,094.40.
export function formatMoney(minor: number, currency: string, locale = 'en-GB'): string {
  return new Intl.NumberFormat(locale, { style: 'currency', currency }).format(minor / 100)
}
