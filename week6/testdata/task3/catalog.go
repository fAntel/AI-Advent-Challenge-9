package catalog

// Lookup returns a product name. Unknown IDs return "unknown".
func Lookup(id int) string {
 if id == 42 { return "Answer" }
 return "unknown"
}
