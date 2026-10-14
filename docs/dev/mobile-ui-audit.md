# Mobile UI audit

Scope: every route of the signed-in app and the forms opened from each page, on the demo data (`cmd/demoseed`, electric, combustion and `layout` vehicles), as a phone with a touch pointer. Chromium with `hasTouch` and `isMobile` at 360×740, 390×844 and 430×932 (the 390 px run also records a screenshot per page). The rules checked are the ones of the "UI and mobile" section of `CLAUDE.md`.

## Method

For each route and each vehicle, a script measures in the rendered page:

- horizontal overflow: `documentElement.scrollWidth` against the viewport, and elements whose right edge passes it outside a scroll container;
- hit areas: buttons, links, `summary`, selects and inputs under 36 px, and under 44 px;
- checkboxes: the box of the closest `label` or `.tap` wrapper, not the 16 px box itself;
- fields: computed `font-size` under 16 px on `input`, `select` and `textarea` (iOS zooms the page on focus below 16 px);
- text under 11 px, and clipped text without an ellipsis.

The forms and sheets opened by the "Add", "New", "Edit", "Filters", "Import" buttons and by the quick-add button are probed the same way: position against the viewport, overflow, hit areas, field font size.

`e2e/tests/mobile.spec.ts` keeps the two rules that regressed in the end-to-end suite; `e2e/tests/a11y.spec.ts` runs axe at 375 px on every route.

## Results

| Check | Result |
| --- | --- |
| Horizontal overflow, 3 widths × 2 vehicles × 12 routes | none |
| Forms and sheets opened from each page | all anchored to the bottom edge, no overflow, footers inside the viewport |
| Text under 11 px | none |
| axe WCAG 2.1 AA at 375 px, 12 routes | passes |
| Hit area of checkboxes | 5 pages had a 13–24 px target, fixed (below) |
| Date fields in forms | 13 px on touch, fixed (below) |
| Short chips and inline links | 3 under 44 px, fixed (below) |
| Long text, GBP amounts, outliers, overdue items (`layout` dataset) | no overflow; long text truncates with an ellipsis; vehicle card actions realigned (below) |

## Defects fixed

- **Date fields in forms were 13 px on touch.** `size="xs"` of `AppDatePicker` sets `--touch-font-size: 0.8125rem` for the compact pickers of toolbars; the charge, toll and tire forms use the same size, so iOS zoomed the page on focus. Inside a `[role="dialog"]` the variable is 16 px again (`main.css`).
- **Checkboxes had a 16 px hit area.** The drive, carpool, tire and maintenance lists, and the comparison list, put the box in a label that was exactly as high as the text (16 px), or had no label at all (13 px). A label around a `.select-box` is now 36 px (44 px on touch) high; the comparison list and the service book option use the shared `.select-box`.
- **One-letter chips of the month range selector were 35 px wide** (`All`). They are 44 px wide on touch.
- **Inline link "See the charges" of the efficiency panel was 30 px high**, and the fleet budget "Edit" button 23 px wide: both now meet the hit area.
- **The fleet budget progress bar had no accessible name** (axe `aria-progressbar-name`); it only appeared once the demo had a budget.

- **Vehicle card actions wrapped unevenly.** With a long vehicle or contract label the flex row left a button alone on its line; the actions are a two-column grid on a phone, the select button spans both.

## Demo data

The electric vehicle is a LOA (a running lease with an option above its expected value) and carries a two-leg carpool with a passenger who boards at the first stop and another at the second, and the household has a monthly budget. The combustion vehicle is a purchase with a planned resale. This fills the resale value card (lease and purchase branches), Carpooling, and the budget of the Fleet page, which were empty or in their empty state.

The `layout` dataset (`-datasets ev,ice,layout`) is a plug-in hybrid in GBP built to stress rendering: a 60-character name, long make, model and address, a drive with an outlier distance, a £4,280 repair, an overdue reminder with a long title, a purchase without resale data and a set of winter tires.

## Remaining findings

- Raw checkboxes that do not use `.select-box` (different size and look from the rest of the app): `FuelLogsPanel`, `TireSessionModal`, `TariffPlanModal`, `ComparisonCompare`, the three service options of `OwnershipWizardModal`, and the two of `OnboardingDataSourcesStep` (20 px). Their labels now carry the hit area only where the box is a `.select-box`; the others keep the browser box.
- The native vehicle `<select>` of the header clips a long name mid-glyph instead of using an ellipsis.
- Every drive of the demo vehicle has the source "Manual", so the amber badge repeats on each row (open item of the demo issue).
- The CSV import file field is 12 px; iOS does not zoom on a file input, so it is left as is.
- Not covered by this run: iOS Safari rendering, the on-screen keyboard over a sheet footer, landscape, the offline banner states and the PWA install flow. They need a device or a browser other than Chromium.
