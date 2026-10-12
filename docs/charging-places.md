# Charging places

Français : [charging-places.fr.md](charging-places.fr.md).

Every charge reports a location as text: a TeslaMate geofence name (`Home`), an address (`Valence Supercharger, Valence`), a Home Assistant tag, or the name given to a charging station. AutoLedger never infers a place from the charging speed or the source of a charge.

## Classifying locations

**Energy → Charges → Places** lists the distinct texts the charges of the vehicle report, with their number of sessions and energy. Each text is linked to a place with one choice; every charge reporting that text, past and future, is classified at once. Two spellings that differ only by case or spacing are the same text.

A place has a name and a category: **Home**, **Work**, **Public** or **Other**. Places belong to the vehicle's owner and are shared by everyone with access to the vehicle. Deleting a place leaves its charges unclassified.

A single charge can be given a place of its own in the charge form, which then wins over its reported text.

## Charging by place

**Energy → Charges** opens with a summary of the charges of the current month. The period is chosen with the presets (1 month, 3 months, 6 months, 1 year, all) or with two days picked on a calendar. **Energy / Cost** switches the monthly bars and the split by place between kWh and the recorded cost:

- the energy, the amount spent and the average price per kWh;
- the energy of each month by place, and the average price of each month;
- a split by place, by energy or by cost, with each place's energy, cost, average price, free sessions and sessions without a cost;
- **If everything were charged at home**: what the energy charged in places classified outside **Home** cost, against the same energy at the average price of the **Home** places.

How the figures are counted:

- The average price is the recorded cost over the energy of the sessions whose cost is known. A free session (cost 0) counts as zero; a session without a recorded cost is left out of the price and its energy and count are shown beside it.
- Sessions that recorded no energy are not counted and are reported apart.
- Energy at unclassified places is shown as such and is never assumed to be away from home, so it is left out of the home figure.
- The home figure is negative ("more") when charging away cost less than the home price, for example free charging at work.
- Costs are the cost ledger amounts, so they match the monthly energy totals of the dashboard.

## Home Assistant

The integration sends the location of a session from an optional **location entity** (for example the vehicle's device tracker, whose state is the zone name) or from a **default location tag**. Without either, the session is placed at the name given to its charging station.
