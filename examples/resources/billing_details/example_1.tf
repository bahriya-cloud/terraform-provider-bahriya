resource "bahriya_billing_details" "this" {
  entity_type = "company"
  legal_name  = "Acme Trading LLC"
  email       = "accounts@acme.example"

  address_line1 = "Office 402"
  address_line2 = "Corniche Street"
  city          = "Dubai"
  state         = "Dubai"
  country       = "AE"

  tax_id      = "100123456789003"
  tax_id_type = "trn"

  registration_number = "1234567"
  billing_reference   = "PO-4471"
}
