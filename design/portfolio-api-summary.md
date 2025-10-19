# Portfolio API Summary

This document captures the new REST endpoints introduced to manage multi-asset wealth portfolios. All routes live under the existing `/api` namespace and require a valid JWT via the `Authorization: Bearer <token>` header, unless stated otherwise. Responses follow JSON format and timestamps use RFC 3339 (UTC).

## Conventions
- `:id`, `:lotId`, `:flowId`, `:valuationId`, `:rateId` denote MongoDB object IDs returned by the API.
- Monetary fields are decimals (`float64`); clients should format/round for display.
- Currency codes use ISO 4217 (uppercase). Commodity numeraires (e.g., `GOLD:OUNCE`) are also uppercase strings.

## Asset Classes

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/asset_classes` | Create a global asset class definition (admin-level usage). |
| `GET`  | `/api/asset_classes` | List all asset classes, sorted by name. |

**Create request**
```json
{
  "code": "BANK_SAVINGS",
  "name": "Bank Savings",
  "default_valuation_method": "BALANCE_WITH_INTEREST",
  "metadata_schema": {
    "interest_rate": "number",
    "compounding": "string"
  }
}
```

**Response**
```json
{
  "code": "BANK_SAVINGS",
  "name": "Bank Savings",
  "default_valuation_method": "BALANCE_WITH_INTEREST",
  "metadata_schema": {
    "interest_rate": "number",
    "compounding": "string"
  },
  "created_at": "2025-01-05T09:23:11Z",
  "updated_at": "2025-01-05T09:23:11Z"
}
```

## Assets

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/assets` | Create a new asset container for the authenticated user. |
| `GET`  | `/api/assets` | List assets. Use `?include_inactive=true` to include archived assets. |
| `GET`  | `/api/assets/:id` | Retrieve a single asset. |
| `PATCH`| `/api/assets/:id` | Update fields on an asset. |
| `DELETE`| `/api/assets/:id` | Delete the asset and cascade dependent records. |

**Create request**
```json
{
  "display_name": "Vietcombank Savings",
  "asset_class_code": "BANK_SAVINGS",
  "default_currency": "VND",
  "metadata": {
    "interest_rate": 0.035,
    "compounding": "monthly"
  },
  "tags": ["cash", "emergency"],
  "expected_return": 0.025
}
```

**Response**
```json
{
  "id": "677a23f05989fd93a4e8902a",
  "asset_class_code": "BANK_SAVINGS",
  "display_name": "Vietcombank Savings",
  "default_currency": "VND",
  "metadata": {
    "interest_rate": 0.035,
    "compounding": "monthly"
  },
  "tags": ["cash", "emergency"],
  "expected_return": 0.025,
  "active": true,
  "created_at": "2025-01-05T09:24:00Z",
  "updated_at": "2025-01-05T09:24:00Z"
}
```

## Position Lots (Acquisitions / Deposits)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/assets/:id/lots` | Create a position lot tied to an asset. |
| `GET`  | `/api/assets/:id/lots` | List lots for an asset (newest first). |
| `PATCH`| `/api/assets/:id/lots/:lotId` | Update a lot. |
| `DELETE`| `/api/assets/:id/lots/:lotId` | Delete a lot. |

**Create request**
```json
{
  "acquired_at": "2024-11-01T00:00:00Z",
  "quantity": 1.5,
  "unit_cost": 2200,
  "cost_currency": "USD",
  "fees": 12.5,
  "notes": "Bought via broker ABC",
  "fx_pair": "USD/VND",
  "fx_rate_used": 24450.12
}
```

**Response**
```json
{
  "id": "677a240d5989fd93a4e8902b",
  "asset_id": "677a23f05989fd93a4e8902a",
  "quantity": 1.5,
  "unit_cost": 2200,
  "cost_currency": "USD",
  "fees": 12.5,
  "notes": "Bought via broker ABC",
  "fx_pair": "USD/VND",
  "fx_rate_used": 24450.12,
  "acquired_at": "2024-11-01T00:00:00Z",
  "created_at": "2025-01-05T09:24:29Z",
  "updated_at": "2025-01-05T09:24:29Z"
}
```

## Cash Flows (Deposits, Withdrawals, Income)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/assets/:id/cashflows` | Record a cash movement tied to an asset. |
| `GET`  | `/api/assets/:id/cashflows` | List cash flows (newest first). |
| `DELETE`| `/api/assets/:id/cashflows/:flowId` | Delete a cash flow entry. |

**Create request (example: November salary)**  
```json
{
  "type": "DEPOSIT",
  "amount": 3200,
  "currency": "USD",
  "occurred_at": "2025-11-05T10:15:00Z",
  "notes": "November salary"
}
```

**Response**
```json
{
  "id": "677a24515989fd93a4e8902c",
  "asset_id": "677a23f05989fd93a4e8902a",
  "type": "DEPOSIT",
  "amount": 3200,
  "currency": "USD",
  "occurred_at": "2025-11-05T10:15:00Z",
  "notes": "November salary",
  "created_at": "2025-01-05T09:24:49Z"
}
```

## Valuation Snapshots

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/assets/:id/valuations` | Record a point-in-time valuation (manual or automated). |
| `GET`  | `/api/assets/:id/valuations?limit=50` | List recent snapshots (default `limit=50`, max `500`). |
| `DELETE`| `/api/assets/:id/valuations/:valuationId` | Delete a valuation snapshot. |

**Create request**
```json
{
  "captured_at": "2025-01-01T00:00:00Z",
  "native_currency_value": 250000000,
  "native_currency": "VND",
  "valuation_currency_map": {
    "USD": 10215.4,
    "VND": 250000000
  },
  "rates_used": {
    "USD/VND": 24470.0
  },
  "source": "manual",
  "notes": "Year start balance"
}
```

## FX Rates

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/fx_rates?scope=global|user` | Insert an FX rate (global default or user-specific). |
| `GET`  | `/api/fx_rates?scope=global|user&base=USD&quote=VND` | List FX rates (latest first). |
| `DELETE`| `/api/fx_rates/:rateId?scope=global|user` | Remove an FX rate. Deletion limited to matching scope. |

**Create request**
```json
{
  "base_currency": "USD",
  "quote_currency": "VND",
  "rate": 24480.5,
  "effective_at": "2025-01-04T00:00:00Z",
  "source": "Vietcombank daily fix"
}
```

## Portfolio Summary

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/api/portfolio/summary?base=USD` | Aggregate all assets for the user, converted to the requested base currency (defaults to the configured reference currency). |

**Sample response**
```json
{
  "base_currency": "USD",
  "total_current_value": 15234.18,
  "total_cost_basis": 13100.00,
  "total_unrealized_gain": 2134.18,
  "positions": [
    {
      "asset": {
        "id": "677a23f05989fd93a4e8902a",
        "asset_class_code": "BANK_SAVINGS",
        "display_name": "Vietcombank Savings",
        "default_currency": "VND",
        "active": true,
        "created_at": "2025-01-05T09:24:00Z",
        "updated_at": "2025-01-05T09:24:00Z"
      },
      "current_value": 10215.40,
      "cost_basis": 10000.00,
      "unrealized_gain": 215.40,
      "unrealized_gain_percent": 2.154,
      "last_valuation_at": "2025-01-01T00:00:00Z"
    }
  ],
  "generated_at": "2025-01-05T09:25:10Z"
}
```

## Error Handling
- Standard HTTP status codes are used (`400` validation errors, `401` unauthorized, `404` not found, `409` conflict, `500` server error).
- Errors are returned as:
```json
{
  "error": "Readable message"
}
```

## Environment / Configuration
- New `config.Config` fields expose collection names and `DefaultReferenceCurrency`.
- `DEFAULT_REFERENCE_CURRENCY` env var controls the fallback conversion scalar (defaults to `USD`).
- Collections introduced: `asset_classes`, `assets`, `position_lots`, `cash_flows`, `valuation_snapshots`, `price_points` (placeholder), `fx_rates`.

This summary should equip downstream tooling (e.g., frontend generator AI) to integrate with the backend portfolio module. Let me know if you need OpenAPI/Swagger form or SDK stubs. 

