# Vehicle comparison

Français : [vehicle-comparison.fr.md](vehicle-comparison.fr.md).

Choose an electric, plug-in hybrid (PHEV) or range-extender (REEV) vehicle, open **Comparison**, and create a **tracked vehicle** comparison. All makes and models use the same calculation.

Set the conventional reference vehicle's fuel consumption in L/100 km, price per litre, purchase and resale prices, annual maintenance, insurance and taxes. Fuel-type presets are starting points, not a model catalogue; replace them with assumptions appropriate to your vehicle, market and currency.

The tracked side uses recorded costs divided by known mileage, then applies the annual distance and duration you enter. Hybrids include fill-ups and charging sessions. A fuel-price change applies to the fuel component of both vehicles; an electricity-price change applies only to charging. The fuel sensitivity chart changes both fuel components as well.

Results show energy costs, maintenance, insurance, depreciation, total cost, cost per distance and projected purchase break-even. Compare the two **Energy** rows for energy savings; total savings include the other cost categories. Recorded costs are left unchanged.

## Data and assumptions

- Record every fill-up and charging session, including free charges, and enter odometer readings. Missing costs or mileage make the estimate unreliable. A hybrid with only fuel or only charging records can be compared; the result then carries a warning, since the calculation cannot infer unrecorded purchases.
- Costs represent purchases in the ledger, not energy measured while driving. Changes in fuel remaining in the tank or battery charge can bias a short history.
- Both sides use the same annual distance. With less than three months of history, annual distance defaults to 12,000 km; edit it to suit your use.
- Insurance: a current recurring premium is annualized by its billing interval (100 per month, 300 per quarter and 1,200 every twelve months all give 1,200 per year); a one-off premium with a coverage period is annualized over that period. Payments with neither use their trailing twelve-month total, with an assumption shown in the result. Future and expired policies are excluded, so set an end date on a replaced recurring policy. Both sides use the same insurance inflation, and annual mileage never changes an insurance total.
- Financing, loans and leases are excluded. Depreciation follows the existing purchase/resale calculation; break-even compares purchase plus running outlays without deducting resale.
- **Projection** remains a manually entered electric-versus-combustion scenario. A tracked hybrid comparison needs recorded hybrid data.

## Free and unknown charging costs

Enter an explicit cost of **0** for a free charge, regardless of why it is free. Zero is a known cost; an absent cost remains unknown. Recorded free charges count as charging records in tracked hybrid comparisons and do not trigger the missing-energy-source warning. Charges with an unknown cost receive a separate reminder to enter their actual cost, or zero if they were free.
