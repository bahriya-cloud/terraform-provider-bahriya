---
page_title: "bahriya_billing_details Resource - Bahriya"
subcategory: ""
description: |-
  The organisation's billing identity — the legal name, postal address, country, tax registration and invoice reference printed on invoices.
---

# bahriya_billing_details (Resource)

Manages the organisation's billing identity: the legal name, postal address, country, tax registration and invoice reference that appear in the **Bill To** block of your invoices. It is a **singleton** — each organisation has exactly one set of billing details, so declare at most one of these per organisation.

Creating the resource writes your declared identity; destroying it clears every declared field. The billing email is the one exception: it is never cleared, and omitting `email` leaves the current address unchanged.

All fields are optional. Invoices are issued either way — the computed `complete` flag only tells you whether the identity is invoice-ready (entity type, address line 1, city and country, plus the legal name for companies).

## Example Usage

### A company with a tax registration

```hcl
resource "bahriya_billing_details" "this" {
  entity_type = "company"
  legal_name  = "Acme Trading LLC"
  email       = "accounts@acme.example"

  address_line1 = "Office 402, Al Majaz Tower"
  address_line2 = "Corniche Street"
  city          = "Sharjah"
  state         = "Sharjah"
  country       = "AE"

  tax_id      = "100123456789003"
  tax_id_type = "trn"

  registration_number = "1234567"
  billing_reference   = "PO-4471"
}
```

### An individual

```hcl
resource "bahriya_billing_details" "this" {
  entity_type   = "individual"
  address_line1 = "12 Harbour Lane"
  city          = "Istanbul"
  country       = "TR"
}
```

## Schema

### Optional

- `legal_name` (String) - Registered legal entity name, printed on invoices. Invoices fall back to the organisation display name when unset.
- `email` (String) - The billing email all organisation-scoped email goes to. Omitting it leaves the current address unchanged.
- `entity_type` (String) - Whether the organisation bills as an individual or a company. One of: individual, company.
- `address_line1` (String) - First line of the postal address.
- `address_line2` (String) - Second line of the postal address.
- `city` (String) - City.
- `state` (String) - State, province or emirate.
- `postcode` (String) - Postal code. Not required everywhere, deliberately optional.
- `country` (String) - ISO 3166-1 alpha-2 country code, e.g. AE or DE. Unrelated to Bahriya's deployment regions.
- `tax_id` (String) - Tax registration number (VAT / TRN / GST / ABN / EIN). Requires tax_id_type.
- `tax_id_type` (String) - The kind of tax registration, driving the label the invoice prints. One of: vat, trn, gst, abn, ein, other.
- `registration_number` (String) - Company registration or trade licence number.
- `billing_reference` (String) - A PO number or cost centre printed on invoices.

### Read-Only

- `id` (String) - UUID of the organisation the details belong to.
- `complete` (Boolean) - Whether the identity is invoice-ready. Advisory only — invoices are issued either way.

## Import

The details are a singleton in the provider's configured organisation, so any import ID refreshes the same record; by convention use the organisation UUID:

```shell
terraform import bahriya_billing_details.this fbae2caa-40de-4196-a58b-5dd26bf8de38
```
