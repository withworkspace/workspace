# Reminders for overdue invoices

## Problem

A studio finds out an invoice is overdue when it reads its own list. Chasing it is an email written by hand, usually a week late.

## Proposal

Seven days after an invoice's due date, Sorrel sends its customer a reminder, with the invoice and how to pay it. The studio is copied in. A second goes at twenty-one days, and none after that.

A studio can turn reminders off for a customer, and for itself.

## What changes

- **api:** a daily job finds invoices that are overdue and unreminded, and sends `invoice.reminded` as it sends the email.
- **web:** an invoice says when it was reminded, and a customer's page has the switch.

## Not in this

- Late fees. That's a decision about money, and a proposal of its own.
