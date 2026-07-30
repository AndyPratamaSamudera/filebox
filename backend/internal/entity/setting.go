package entity

// Setting maps to the settings table and stores a runtime configuration key/value pair.
type Setting struct {
	Key   string  `db:"key" json:"key"`
	Value *string `db:"value" json:"value"`
}
