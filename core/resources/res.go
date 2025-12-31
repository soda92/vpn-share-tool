package resources

import (
	_ "embed"
)

//go:embed injector.js
var InjectorScript []byte

//go:embed solver_script.js
var SolverScript []byte

//go:embed calendar.unpacked.js
var CalendarScript []byte
