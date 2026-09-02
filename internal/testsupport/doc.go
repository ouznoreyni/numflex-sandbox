// Package testsupport holds test-only infrastructure shared across
// packages: the test database helper here, in-memory port doubles under
// ./inmemory, and the router harness under ./routerharness.
//
// May import: anything an outer layer may — it is test scaffolding, not
// production code, and no production package imports it.
//
// Must never know: a business rule. A double here mimics a gateway's
// storage, never an interactor's decision; the moment a double starts
// deciding, the test stops proving the interactor does.
package testsupport
