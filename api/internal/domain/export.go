package domain

// LedgerExport is a versioned, portable JSON snapshot, without credentials.
type LedgerExport struct{ Content []byte }
