# Wealth Portfolio Module - Design Proposal

## 1. Objective
- Extend the existing My Finance backend (Go + Gin + MongoDB) with a portfolio module that tracks multi-asset wealth, including bank accounts, cash, gold, crypto, stocks, and future asset classes.
- Provide CRUD management for holdings and contributions, allow flexible base-currency reporting, and compute performance metrics that adjust for currency fluctuation.
- Support projected growth for interest-bearing accounts (e.g., term deposits) alongside mark-to-market valuations for market-traded instruments.

## 2. Current System Touchpoints
- **Authentication & Authorization**: Reuse JWT-based auth; enforce that portfolios are scoped to `user_id` similar to expenses/categories.
- **MongoDB Usage**: Existing collections (expenses, categories, tags) store documents per user. New collections should follow the same pattern (`user_id` + data) to keep per-user isolation simple.
- **Configuration**: Leverage `config.Config` for new collection names and integration settings (API keys, base currency defaults).

## 3. Scope
- **In Scope**
  - Persisting asset definitions, lots/transactions, income contributions, and valuations.
  - Calculating gain/loss in arbitrary base currencies using historical FX data.
  - Summaries: total wealth, unrealised/realised gains, interest projections.
  - Extensible mechanism to add new asset classes without schema churn.
- **Out of Scope (initial phase)**
  - Real-money trading or brokerage integrations.
  - Automated tax calculations.
  - UI/visualisation specifics (handled by frontend).

## 4. Domain Concepts
| Concept | Purpose | Key Fields |
|---------|---------|------------|
| `AssetClass` | Canonical list of supported categories (bank, cash, gold, crypto, stock, property, etc.). | `code`, `name`, `default_valuation_method`, `metadata_schema` |
| `Asset` | A user-owned asset container (e.g., "Vietcombank Savings Account", "1 oz Gold Bar"). | `_id`, `user_id`, `asset_class_code`, `display_name`, `metadata`, `default_currency`, `active` |
| `PositionLot` | Immutable record of an acquisition/contribution (buy order, deposit). | `_id`, `asset_id`, `user_id`, `acquired_at`, `quantity`, `unit_cost`, `cost_currency`, `fees`, `notes` |
| `CashFlow` | Additional transfers affecting cost basis (top-ups, withdrawals, dividends, interest payouts). | `_id`, `asset_id`, `type`, `amount`, `currency`, `occurred_at`, `notes` |
| `ValuationSnapshot` | Point-in-time valuation of an asset expressed in multiple currencies. | `_id`, `asset_id`, `captured_at`, `market_value_native`, `native_currency`, `market_value_usd`, `rates_used` |
| `PricePoint` | Source-agnostic market prices (per instrument & timestamp). | `_id`, `instrument_key`, `price`, `currency`, `effective_at`, `source` |
| `FxRate` | Exchange rates relative to a canonical reference (e.g., USD). | `_id`, `base_currency`, `quote_currency`, `rate`, `effective_at`, `source` |

Notes:
- `Asset.metadata` captures asset-class-specific details (e.g., bank interest rate, ticker symbol, wallet address) stored as a JSON/BSON document validated against optional JSON schema.
- For bank accounts with compounding, store `interest_policy` inside metadata (e.g., `type: fixed`, `rate: 3.5`, `period: monthly`, `capitalization: true`).

## 5. Data Model (Mongo Collections)
```
asset_classes
  { code: "BANK_SAVINGS", name: "Bank Savings", default_valuation_method: "BALANCE_WITH_INTEREST", metadata_schema: {...} }

assets
  { _id, user_id, asset_class_code, display_name, default_currency, metadata, created_at, updated_at }

position_lots
  { _id, asset_id, user_id, acquired_at, quantity, unit_cost, cost_currency, fees, notes }

cash_flows
  { _id, asset_id, user_id, type: "DEPOSIT"|"WITHDRAWAL"|"DIVIDEND"|"INTEREST", amount, currency, occurred_at, notes }

valuation_snapshots
  { _id, asset_id, user_id, captured_at, native_currency_value, native_currency, valuation_currency_map: { "USD": 9321.12, "VND": 220_000_000 }, rates_used }

price_points
  { instrument_key: "GOLD:OUNCE", price: 2300.25, currency: "USD", effective_at, source }

fx_rates
  { base_currency: "USD", quote_currency: "VND", rate: 24500.12, effective_at, source }
```
- Add new collection names to `config.Config` (e.g., `CollectionAssetsName`, `CollectionValuationsName`).
- Indexing strategy:
  - `assets`: compound index on `{ user_id: 1, asset_class_code: 1 }`
  - `position_lots`: `{ asset_id: 1, acquired_at: -1 }`
  - `valuation_snapshots`: `{ asset_id: 1, captured_at: -1 }`
  - `fx_rates`: `{ quote_currency: 1, effective_at: -1 }`
  - TTL index on stale `price_points` if necessary.

## 6. Valuation & Currency Strategy
- **Canonical Reference Currency**: Store all FX rates relative to a single base (e.g., USD). When the user requests a report in a different base (VND, gold-equivalent), derive via conversion matrix.
- **Historical Consistency**: For gain/loss since acquisition, convert the original cost at acquisition date using the FX rate from that date (or closest prior). Store the rate used within `PositionLot` for auditing (`fx_rate_used`).
- **Base Currency Switching**:
  1. Load all relevant `ValuationSnapshot` entries (or compute on-demand using latest `PricePoint` + `FxRate`).
  2. Convert native market value to desired base currency using FX chain (`target_value = native_value * rate(native_currency→reference) * rate(reference→target)`).
  3. Display both absolute and percentage performance.
- **Gold or other commodity as numéraire**: Treat “Gold Ounce” as pseudo-currency with its own `FxRate` derived from price per ounce; conversions then behave the same way.
- **Interest Accrual**: For savings accounts, compute expected balance using stored interest policy (daily compound, monthly, etc.) within valuation pipeline before converting currency.

## 7. Core Use Cases & Flows
1. **Asset CRUD**
   - POST `/api/assets`: create asset with class metadata (validate structure per class).
   - GET `/api/assets`: list per user with latest valuation summary.
   - PATCH/DELETE endpoints similar to categories.
2. **Record Acquisition or Contribution**
   - POST `/api/assets/{id}/lots`: record new buy/deposit with cost data.
   - For bank accounts, options to track both principal and interest accrual events.
3. **Update Market Prices**
   - Background job fetches `PricePoint` data (e.g., gold price, crypto tickers) from external APIs (requires API key configuration).
   - For assets tied to static valuation (e.g., real estate), allow manual `ValuationSnapshot` entry by user.
4. **FX Rate Ingestion**
   - Nightly job to refresh central bank / market FX rates; store in `fx_rates`.
5. **Portfolio Summary**
   - GET `/api/portfolio?base=USD`: aggregates all assets, combining lots and latest valuation, returns:
     - total_current_value
     - total_cost_basis
     - unrealised_gain (absolute & %)
     - breakdown per asset class, per asset
     - optional “gold equivalent” by requesting `base=GOLD:OUNCE`
6. **Performance Over Time**
   - Use `valuation_snapshots` to generate time series; user can request `/api/portfolio/history?base=USD&range=1y`.
7. **Income Tracking**
   - For job income, represent as `CashFlow` entries on selected cash/bank assets.
   - Optionally link to existing expense records via `link_id` to avoid duplicate entry, but keep systems decoupled initially.

## 8. Service Architecture
- **Handlers**: Introduce `portfolio` package with sub-handlers: `asset_handler.go`, `valuation_handler.go`, `summary_handler.go`.
- **Services Layer**: Add internal services to encapsulate valuation logic (`valuation.Service`, `fx.Service`) to avoid bloated handlers.
- **Background Jobs**:
  - Implement as separate Go routines triggered via `cron` (if app runs continuously) or companion CLI commands invoked by external scheduler.
  - Jobs use shared Mongo client + config; ensure idempotency.
- **Third-Party Integrations**:
  - Wrap external price/FX APIs in provider interfaces to allow switching data sources or offline mode.
  - Cache responses to avoid rate limits (e.g., store last successful rate and timestamp).

## 9. Calculations & Reporting
- **Portfolio Valuation Algorithm**:
  1. Retrieve all assets for user.
  2. For each asset:
     - Aggregate cost basis: sum(`quantity * unit_cost` + fees) converted to reference currency using acquisition-date FX.
     - Determine current value:
       - Market-priced assets: latest `PricePoint`.
       - Bank savings: principal + accrued interest per policy.
       - Cash: direct balance (sum deposits – withdrawals + interest).
     - Convert to base currency as per section 6.
  3. Compute gain/loss, percentage change, and share of portfolio.
- **Projections**:
  - Support simple projection endpoint that extrapolates bank account balances using current interest policy (e.g., one-year forecast).
  - For investments, optionally allow user-specified expected annual return stored in `metadata` to power scenario analysis later.

## 10. Validation & Auditing
- **Consistency Checks**: Before recording valuation, ensure the asset exists and belongs to the requesting user.
- **Audit Trails**: Store `created_by`, `created_at`, `updated_at` on mutable documents. For regulatory-style logging, append to an `activity_logs` collection.
- **Data Integrity**: Use MongoDB schema validation rules for collections (via `$jsonSchema`) to prevent missing fields.

## 11. Security & Access Control
- Reuse existing JWT middleware; add role checks where needed (e.g., admin can define `AssetClass` catalogue).
- Ensure all queries filter by `user_id`.
- If multi-tenant admin features are added, consider namespacing `asset_classes` by owner vs. global definitions.

## 12. Migration Plan
1. Update `config.Config` with new collection names and defaults (base currency, external API keys).
2. Introduce new Mongo collections with indexes (migration script or bootstrap function).
3. Build `portfolio` package with handlers, repositories, services.
4. Implement FX and price ingestion jobs (start with manual upload endpoints if external API access is deferred).
5. Create portfolio summary endpoint and align with frontend requirements.

## 13. Future Enhancements
- Support derivative assets (options, futures) with risk metrics (Greeks).
- Integrate budgeting/expense module for holistic cashflow analysis.
- Machine-learning forecasts using historical valuations.
- Alerts & notifications when asset drifts beyond configured thresholds.

---

This design keeps the existing Go/Mongo architecture intact while introducing a modular portfolio subsystem that can grow in complexity (more asset classes, analytics, automation) without constant schema changes. By normalizing on a canonical currency and preserving acquisition-time FX rates, the system can present gains/losses in any base currency, including commodity-based numeraires like gold, meeting the outlined requirements.
