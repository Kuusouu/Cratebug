// Package uassettool owns all communication with the UAssetToolRivals
// worker: its newline-delimited JSON request/response contract and its
// pinned-version check.
//
// Adapter speaks the protocol over an injected io.Writer/io.Reader pair, so
// it has no opinion on how the worker process is launched or supervised;
// see docs/decisions/0003-uassettoolrivals-boundary.md for that decision.
// Worker builds process lifecycle (launch, pinned-version health check,
// call timeout, crash recovery, graceful shutdown) on top of Adapter.
package uassettool
