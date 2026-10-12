// Package tolldata provides a French toll station reference (name, position, network topology)
// vendored from OpenTollData (https://github.com/louis2038/OpenTollData), used to detect which
// toll stations a drive's GPS trace crossed.
package tolldata

import "embed"

//go:embed data/opentolldata_network.json data/extra_gates.json
var FS embed.FS
