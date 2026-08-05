// Package api names the resource group every committed koment record belongs
// to. The group and version live here rather than in store or policy because
// they belong to neither: both resources carry the same apiVersion, and a
// second copy of that string is a second thing to forget to change.
package api

// Group is the canonical domain. A Kubernetes API group is a DNS name for the
// same reason ours is: it makes the owner of a schema unambiguous without a
// central registry. ADR 0119.
const Group = "koment.dev"

// Version is matched exactly. A record carrying anything else is refused
// rather than guessed at.
const Version = Group + "/v1alpha"

// SchemaBase is where the schemas for this API version are published. The path
// carries the version so that a record written today keeps pointing at the
// schema that describes it after the next version exists. ADR 0121.
const SchemaBase = "https://raw.githubusercontent.com/janpuc/koment/main/schema/v1alpha/"
