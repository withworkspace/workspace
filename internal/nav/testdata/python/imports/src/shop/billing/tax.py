from .invoice import Invoice
from ..catalog import price
from . import invoice


def rate(inv: Invoice) -> float:
    #             ^ resolved src/shop/billing/invoice.py:Invoice "imported from .invoice"
    return price("x") * invoice.Invoice().total()
    #      ^ resolved src/shop/catalog.py:price "imported from ..catalog"
    #                   ^ resolved src/shop/billing/invoice.py
    #                           ^ resolved src/shop/billing/invoice.py:Invoice
    #                                     ^ resolved src/shop/billing/invoice.py:total
