package data

import _ "embed"

//go:embed debloat.json
var DebloatJSON []byte

//go:embed modules.json
var ModulesJSON []byte
