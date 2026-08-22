package metadata

// migrationStep upgrades a document's raw JSON representation from one
// schema version to the next, mutating raw in place and setting its
// "schemaVersion" field to the version it produces.
type migrationStep func(raw map[string]any) error

// migrations maps a schema version to the step that upgrades a document from
// that version to the next one. readDocument applies these in sequence until
// the document reaches CurrentSchemaVersion.
//
// Version 0 covers a metadata file with no "schemaVersion" field at all, for
// example one created or hand-edited before this field existed. Every field
// schema version 1 introduced already defaults safely to its Go zero value,
// so this step only needs to stamp the version; it exists so a real future
// schema change has a tested migration path to extend instead of improvising
// one under pressure.
var migrations = map[int]migrationStep{
	0: migrateFromUnversioned,
}

func migrateFromUnversioned(raw map[string]any) error {
	raw["schemaVersion"] = 1
	return nil
}

// Reads the "schemaVersion" field from a document's raw JSON representation.
// A missing field means schema version 0: a file written before this field
// existed. The value is float64 when it came from encoding/json (JSON
// numbers always decode to that type) or int when a migrationStep just set
// it directly, as migrateFromUnversioned does.
func schemaVersionOf(raw map[string]any) int {
	switch version := raw["schemaVersion"].(type) {
	case float64:
		return int(version)
	case int:
		return version
	default:
		return 0
	}
}
