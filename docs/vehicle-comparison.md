# Vehicle comparison

Français : [vehicle-comparison.fr.md](vehicle-comparison.fr.md).

Choose an electric, plug-in hybrid (PHEV) or range-extender (REEV) vehicle, open **Comparison**, and create a **tracked vehicle** comparison. All makes and models use the same calculation.

Set the conventional reference vehicle's fuel consumption in L/100 km, price per litre, purchase and resale prices, annual maintenance, insurance and taxes. Fuel and electricity prices use currency-neutral storage bounds, including in the What if inputs. Fuel-type presets are starting points, not a model catalogue; replace them with assumptions appropriate to your vehicle, market and currency.

The tracked side uses recorded costs divided by known mileage, then applies the annual distance and duration you enter. Hybrids include fill-ups and charging sessions. A fuel-price change applies to the fuel component of both vehicles; an electricity-price change applies only to charging. The fuel sensitivity chart changes both fuel components as well.

Results show energy costs, maintenance, insurance, depreciation, total cost, cost per distance and projected purchase break-even. Compare the two **Energy** rows for energy savings; total savings include the other cost categories. Recorded costs are left unchanged.

## Data and assumptions

- Record every fill-up and charging session, including free charges, and enter odometer readings. Missing costs or mileage make the estimate unreliable. A hybrid with only fuel or only charging records can be compared; the result then carries a warning, since the calculation cannot infer unrecorded purchases.
- Costs represent purchases in the ledger, not energy measured while driving. Changes in fuel remaining in the tank or battery charge can bias a short history.
- Both sides use the same annual distance. With less than three months of history, annual distance defaults to 12,000 km; edit it to suit your use.
- Insurance: a current recurring premium is annualized by its billing interval (100 per month, 300 per quarter and 1,200 every twelve months all give 1,200 per year); a one-off premium with a coverage period is annualized over that period. Payments with neither use their trailing twelve-month total, with an assumption shown in the result. Future and expired policies are excluded, so set an end date on a replaced recurring policy. Both sides use the same insurance inflation, and annual mileage never changes an insurance total.
- The tracked side is labelled "projected from your recorded costs": it extrapolates what you entered, it is not a measured total. The result states under the verdict, on screen, in print and in the CSV export, what is not compared: tolls, parking, financing, subscriptions and other costs.
- Financing, loans and leases are excluded. Depreciation follows the existing purchase/resale calculation; break-even compares purchase plus running outlays without deducting resale.
- **Projection** remains a manually entered electric-versus-combustion scenario. A tracked hybrid comparison needs recorded hybrid data.

## Break-even

The break-even is the year after which the cumulated outlay of the tracked vehicle (purchase price plus yearly energy, maintenance, insurance and taxes, with the inflation of the scenario) drops below the combustion one. Resale value is not deducted; when a resale value is entered, a second figure credits each vehicle with its value at that point. The search stops at the end of the period and never extrapolates beyond it.

When it is not reached, the result says why:

- **the period is too short**: the running savings shrink the purchase price gap but do not close it within the period;
- **no running saving**: the tracked vehicle does not cost less to run than the combustion one;
- **reversed**: the tracked vehicle is cheaper to buy but ends up costing more than the combustion one over the period.

## What if

The result of a **projection** has a **What if…?** section. Move the sliders, or type a value, for the annual distance, the fuel price per litre and the electricity price per kWh; the sliders span half to twice the saved value. **More assumptions** opens the period (whole years), the purchase and resale prices of both vehicles and the fuel, electricity and other-costs inflation, with the same limits as the scenario form. The explored figures appear next to the saved ones, with the gap: energy savings (fuel and electricity only), total savings (every cost of the scenario) and the break-even on the full cost are three separate figures. The break-even gap is in years, or states that the break-even becomes or stops being reached. **Reset** returns to the saved values and **Save as a new comparison** opens the form with the explored values, leaving the original comparison unchanged.

The explored inputs are sent to the same calculation as a saved scenario and are not stored. A hypothetical price never reprices a recorded charge or fill-up. Distances and volumes follow the account settings. A tracked comparison uses its reference vehicle’s currency; a projection stores its chosen currency. Changing that code never converts amounts. An older projection takes the currency shared by all the vehicles of its owner; when they differ, its owner confirms the currency before viewing it. A projection saved without a currency takes that same shared currency; when the vehicles differ, the request is refused until a currency is chosen. comparisons in different currencies cannot share a chart or CSV. The section is available for projections only.

## Free and unknown charging costs

Enter an explicit cost of **0** for a free charge, regardless of why it is free. Zero is a known cost; an absent cost remains unknown. In a manual projection, enter an electricity price of 0 for free energy; an empty price is not zero. Recorded free charges count as charging records in tracked hybrid comparisons and do not trigger the missing-energy-source warning. Charges with an unknown cost receive a separate reminder to enter their actual cost, or zero if they were free.

The projection what-if editor warns before using a fuel or electricity price above three times the median of at least five known purchases for the selected vehicle. Fuel purchases must match the scenario’s fuel type. Charges in another currency are converted using their recorded exchange rate; unknown costs are excluded and free charges count as zero. With no suitable history, a zero median or a different scenario currency, no warning is shown. You can confirm the price and continue; confirmation never changes the value or the stored records. Changing the price or reference asks again.
