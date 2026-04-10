package glyph

import _ "embed"

// glyphHelperDex holds the compiled glyph-helper DEX binary embedded at build time.
//
// The DEX is built from the companion repo:
//
//	https://github.com/Limplom/nothingctl-glyph-helper
//
// To update: download the classes.dex artifact from a release and replace
// go/internal/glyph/assets/glyph-helper.dex, then rebuild.
//
// The placeholder file (empty) disables helper-based feedback gracefully —
// deployHelper returns an error when len(glyphHelperDex) == 0.
//
//go:embed assets/glyph-helper.dex
var glyphHelperDex []byte
