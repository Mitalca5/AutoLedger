# Charging places

Français : [charging-places.fr.md](charging-places.fr.md).

Every charge reports a location as text: a TeslaMate geofence name (`Home`), an address (`Valence Supercharger, Valence`), a Home Assistant tag, or the name given to a charging station. AutoLedger never infers a place from the charging speed or the source of a charge.

## Classifying locations

**Energy → Charges → Places** lists the distinct texts the charges of the vehicle report, with their number of sessions and energy. Each text is linked to a place with one choice; every charge reporting that text, past and future, is classified at once. Two spellings that differ only by case or spacing are the same text.

A place has a name and a category: **Home**, **Work**, **Public** or **Other**. Places belong to the vehicle's owner and are shared by everyone with access to the vehicle. Deleting a place leaves its charges unclassified.

A single charge can be given a place of its own in the charge form, which then wins over its reported text.

## Home Assistant

The integration sends the location of a session from an optional **location entity** (for example the vehicle's device tracker, whose state is the zone name) or from a **default location tag**. Without either, the session is placed at the name given to its charging station.
