import os
import requests
import shop.billing.tax
from shop import Invoice
from shop.billing import tax as t

os.path.join("a", "b")
# <- external os
#  ^ external os
requests.get("x")
# <- external requests
Invoice().total()
# <- resolved src/shop/billing/invoice.py:Invoice "imported from shop"
shop.billing.tax.rate(Invoice())
# <- resolved src/shop/__init__.py
#    ^ resolved src/shop/billing/__init__.py
#            ^ resolved src/shop/billing/tax.py
#                ^ resolved src/shop/billing/tax.py:rate
t.rate(None)
# ^ resolved src/shop/billing/tax.py:rate "imported from shop.billing.tax"
from pathlib import Path

Path("x").is_file()
#         ^ external pathlib
