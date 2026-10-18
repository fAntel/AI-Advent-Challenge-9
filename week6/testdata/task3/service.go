package catalog

func Featured() string { return Lookup(42) }
func Missing() string { return Lookup(-1) }
